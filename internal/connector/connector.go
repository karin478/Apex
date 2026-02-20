// Package connector provides the core connector framework including YAML spec
// loading, a circuit breaker with four-state machine (CLOSED → OPEN → HALF_OPEN
// → RECOVERING → CLOSED), and a thread-safe registry.
package connector

import (
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ---- Spec types ----

// ConnectorSpec describes a connector loaded from a YAML specification file.
type ConnectorSpec struct {
	Name           string              `yaml:"name" json:"name"`
	Type           string              `yaml:"type" json:"type"`
	SpecVersion    string              `yaml:"spec_version" json:"spec_version"`
	APIVersion     string              `yaml:"api_version" json:"api_version"`
	BaseURL        string              `yaml:"base_url" json:"base_url"`
	Auth           AuthConfig          `yaml:"auth" json:"auth"`
	CircuitBreaker CBConfig            `yaml:"circuit_breaker" json:"circuit_breaker"`
	RateLimitGroup string              `yaml:"rate_limit_group" json:"rate_limit_group"`
	Endpoints      map[string]Endpoint `yaml:"endpoints" json:"endpoints"`
	AllowedAgents  []string            `yaml:"allowed_agents" json:"allowed_agents"`
	RiskLevel      string              `yaml:"risk_level" json:"risk_level"`
}

// AuthConfig specifies the authentication method for a connector.
type AuthConfig struct {
	Type   string `yaml:"type" json:"type"`
	EnvVar string `yaml:"env_var" json:"env_var"`
}

// CBConfig holds circuit breaker configuration from the spec.
type CBConfig struct {
	FailureThreshold int `yaml:"failure_threshold" json:"failure_threshold"`
	CooldownSeconds  int `yaml:"cooldown_seconds" json:"cooldown_seconds"`
}

// Endpoint describes a single API endpoint within a connector spec.
type Endpoint struct {
	Path               string `yaml:"path" json:"path"`
	Method             string `yaml:"method" json:"method"`
	Timeout            string `yaml:"timeout" json:"timeout"`
	IdempotencySupport string `yaml:"idempotency_support" json:"idempotency_support"`
}

// LoadSpec reads and parses a YAML connector spec from the given path.
// It returns an error if the file cannot be read, is not valid YAML, or if the
// required "name" field is empty.
func LoadSpec(path string) (*ConnectorSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("connector: read spec: %w", err)
	}

	var spec ConnectorSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("connector: parse spec: %w", err)
	}

	if spec.Name == "" {
		return nil, fmt.Errorf("connector: spec missing required field: name")
	}

	return &spec, nil
}

// ---- Circuit Breaker ----

// CBState represents the current state of a circuit breaker.
type CBState string

const (
	CBClosed     CBState = "CLOSED"
	CBOpen       CBState = "OPEN"
	CBHalfOpen   CBState = "HALF_OPEN"
	CBRecovering CBState = "RECOVERING"
)

const maxCooldown = 300 * time.Second

// CircuitBreaker implements a four-state circuit breaker pattern.
type CircuitBreaker struct {
	state            CBState
	failures         int
	successes        int // tracks successes during RECOVERING ramp-up
	probeInFlight    bool
	lastFailure      time.Time
	cooldownDuration time.Duration
	maxCooldown      time.Duration
	failureThreshold int
	mu               sync.Mutex
}

// CBStatus is a snapshot of a circuit breaker's current state.
type CBStatus struct {
	State    CBState       `json:"state"`
	Failures int           `json:"failures"`
	Cooldown time.Duration `json:"cooldown"`
}

// NewCircuitBreaker creates a circuit breaker from the given config.
// Zero-value fields are filled with defaults (FailureThreshold=5, CooldownSeconds=60).
func NewCircuitBreaker(cfg CBConfig) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.CooldownSeconds <= 0 {
		cfg.CooldownSeconds = 60
	}

	return &CircuitBreaker{
		state:            CBClosed,
		cooldownDuration: time.Duration(cfg.CooldownSeconds) * time.Second,
		maxCooldown:      maxCooldown,
		failureThreshold: cfg.FailureThreshold,
	}
}

// Allow reports whether a request is permitted under the current breaker state.
//
// State transitions triggered by Allow:
//   - OPEN → HALF_OPEN when cooldown has elapsed (returns true for a single probe).
//   - HALF_OPEN blocks all calls while a probe is in flight.
//   - RECOVERING allows requests according to the ramp-up schedule.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CBClosed:
		return true

	case CBOpen:
		if time.Since(cb.lastFailure) >= cb.cooldownDuration {
			cb.state = CBHalfOpen
			cb.probeInFlight = true
			return true
		}
		return false

	case CBHalfOpen:
		// Only one probe at a time.
		if cb.probeInFlight {
			return false
		}
		// Probe completed previously but state hasn't transitioned yet — block.
		return false

	case CBRecovering:
		// Allow requests through; RecordSuccess tracks ramp progress (4 successes → CLOSED).
		return true

	default:
		return false
	}
}

// RecordSuccess records a successful request.
//
// State transitions:
//   - CLOSED: resets failure counter to 0.
//   - HALF_OPEN: transitions to RECOVERING (successes reset to 0).
//   - RECOVERING: increments successes; at 4 successes transitions to CLOSED.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CBClosed:
		cb.failures = 0

	case CBHalfOpen:
		cb.state = CBRecovering
		cb.successes = 0
		cb.probeInFlight = false

	case CBRecovering:
		cb.successes++
		if cb.successes >= 4 {
			cb.state = CBClosed
			cb.failures = 0
			cb.successes = 0
		}
	}
}

// RecordFailure records a failed request.
//
// State transitions:
//   - CLOSED: increments failures; at threshold transitions to OPEN.
//   - HALF_OPEN: transitions to OPEN with doubled cooldown (capped at 300s).
//   - RECOVERING: transitions to OPEN.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CBClosed:
		cb.failures++
		if cb.failures >= cb.failureThreshold {
			cb.state = CBOpen
			cb.lastFailure = time.Now()
		}

	case CBHalfOpen:
		cb.state = CBOpen
		cb.lastFailure = time.Now()
		cb.probeInFlight = false
		// Double the cooldown, capped at maxCooldown.
		cb.cooldownDuration *= 2
		if cb.cooldownDuration > cb.maxCooldown {
			cb.cooldownDuration = cb.maxCooldown
		}

	case CBRecovering:
		cb.state = CBOpen
		cb.lastFailure = time.Now()
		cb.successes = 0
	}
}

// Status returns a snapshot of the circuit breaker's current state.
func (cb *CircuitBreaker) Status() CBStatus {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	return CBStatus{
		State:    cb.state,
		Failures: cb.failures,
		Cooldown: cb.cooldownDuration,
	}
}

// ---- Registry ----

// Registry is a thread-safe collection of connector specs and their breakers.
type Registry struct {
	connectors map[string]*ConnectorSpec
	breakers   map[string]*CircuitBreaker
	mu         sync.RWMutex
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		connectors: make(map[string]*ConnectorSpec),
		breakers:   make(map[string]*CircuitBreaker),
	}
}

// Register adds a connector spec to the registry and creates its circuit breaker.
// Returns an error if a connector with the same name is already registered.
func (r *Registry) Register(spec *ConnectorSpec) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.connectors[spec.Name]; exists {
		return fmt.Errorf("connector: already registered: %s", spec.Name)
	}

	r.connectors[spec.Name] = spec
	r.breakers[spec.Name] = NewCircuitBreaker(spec.CircuitBreaker)
	return nil
}

// Get retrieves a connector spec by name.
func (r *Registry) Get(name string) (*ConnectorSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	spec, ok := r.connectors[name]
	return spec, ok
}

// List returns all registered connector specs sorted alphabetically by name.
func (r *Registry) List() []*ConnectorSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	specs := make([]*ConnectorSpec, 0, len(r.connectors))
	for _, s := range r.connectors {
		specs = append(specs, s)
	}
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].Name < specs[j].Name
	})
	return specs
}

// BreakerStatus returns the CBStatus for the named connector's circuit breaker.
func (r *Registry) BreakerStatus(name string) (*CBStatus, bool) {
	r.mu.RLock()
	cb, ok := r.breakers[name]
	r.mu.RUnlock()
	if !ok {
		return nil, false
	}
	status := cb.Status()
	return &status, true
}

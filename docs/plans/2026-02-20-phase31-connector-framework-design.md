# Phase 31: Connector Framework — Design Document

**Date:** 2026-02-20
**Status:** Approved (recommendation credit 9/10)

## Overview

Implement a tool connector framework (`internal/connector`) with YAML spec loading, circuit breaker state machine, and connector registry. Based on Architecture v11.0 §2.6 and §2.10.

## Architecture

### Connector Spec (from Architecture §2.10)

YAML-based connector specification defining endpoints, auth, circuit breaker config, rate limit group, and risk level.

### Core Types

```go
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

type AuthConfig struct {
    Type   string `yaml:"type" json:"type"`
    EnvVar string `yaml:"env_var" json:"env_var"`
}

type CBConfig struct {
    FailureThreshold int `yaml:"failure_threshold" json:"failure_threshold"` // default 5
    CooldownSeconds  int `yaml:"cooldown_seconds" json:"cooldown_seconds"`   // default 60
}

type Endpoint struct {
    Path               string `yaml:"path" json:"path"`
    Method             string `yaml:"method" json:"method"`
    Timeout            string `yaml:"timeout" json:"timeout"`
    IdempotencySupport string `yaml:"idempotency_support" json:"idempotency_support"`
}

type CBState string

const (
    CBClosed     CBState = "CLOSED"
    CBOpen       CBState = "OPEN"
    CBHalfOpen   CBState = "HALF_OPEN"
    CBRecovering CBState = "RECOVERING"
)

type CircuitBreaker struct {
    state            CBState
    failures         int
    successes        int
    lastFailure      time.Time
    cooldownDuration time.Duration
    maxCooldown      time.Duration // 300s
    failureThreshold int
    mu               sync.Mutex
}

type CBStatus struct {
    State    CBState       `json:"state"`
    Failures int           `json:"failures"`
    Cooldown time.Duration `json:"cooldown"`
}

type Registry struct {
    connectors map[string]*ConnectorSpec
    breakers   map[string]*CircuitBreaker
    mu         sync.RWMutex
}
```

### Operations

```go
func LoadSpec(path string) (*ConnectorSpec, error)
func NewCircuitBreaker(cfg CBConfig) *CircuitBreaker
func (cb *CircuitBreaker) RecordSuccess()
func (cb *CircuitBreaker) RecordFailure()
func (cb *CircuitBreaker) Allow() bool
func (cb *CircuitBreaker) Status() CBStatus
func NewRegistry() *Registry
func (r *Registry) Register(spec *ConnectorSpec) error
func (r *Registry) Get(name string) (*ConnectorSpec, bool)
func (r *Registry) List() []*ConnectorSpec
func (r *Registry) BreakerStatus(name string) (*CBStatus, bool)
```

- **LoadSpec**: Reads YAML file, unmarshals to ConnectorSpec. Returns error for invalid YAML or missing name field.
- **CircuitBreaker**: State machine per Architecture §2.6:
  - CLOSED: Normal operation. Consecutive failures >= threshold → OPEN.
  - OPEN: All requests blocked. After cooldown → HALF_OPEN.
  - HALF_OPEN: Single probe request allowed. Success → RECOVERING. Failure → OPEN (cooldown doubles, max 300s).
  - RECOVERING: Gradual ramp-up (1→2→4→full). After reaching full → CLOSED. Any failure → OPEN.
- **Registry**: Thread-safe registry of connectors with their circuit breakers.

### CLI Command

```
apex connector list           # List registered connectors
apex connector status         # Show circuit breaker status for all connectors
```

## Testing

| Test | Description |
|------|-------------|
| TestLoadSpec | Parse valid YAML connector spec, verify all fields |
| TestLoadSpecInvalid | Invalid YAML returns error |
| TestCircuitBreakerClosed | Normal state allows requests, tracks failures |
| TestCircuitBreakerOpenAfterFailures | Consecutive failures trigger OPEN state |
| TestCircuitBreakerHalfOpen | After cooldown, transitions to HALF_OPEN |
| TestCircuitBreakerRecovering | Success in HALF_OPEN enters RECOVERING with gradual ramp |
| TestRegistry | Register/Get/List/BreakerStatus operations |
| E2E: TestConnectorList | apex connector list shows connectors |
| E2E: TestConnectorStatusEmpty | No connectors shows empty message |
| E2E: TestConnectorStatusRuns | Command exits cleanly |

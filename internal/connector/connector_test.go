package connector

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- helpers ----

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "spec.yaml")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}

const validYAML = `
name: github
type: http_api
spec_version: "1.0"
api_version: v3
base_url: https://api.github.com
auth:
  type: bearer_token
  env_var: GITHUB_TOKEN
circuit_breaker:
  failure_threshold: 3
  cooldown_seconds: 30
rate_limit_group: github_core
endpoints:
  list_repos:
    path: /user/repos
    method: GET
    timeout: 10s
    idempotency_support: native
  create_issue:
    path: /repos/{owner}/{repo}/issues
    method: POST
    timeout: 15s
    idempotency_support: client_generated
allowed_agents:
  - code_agent
  - review_agent
risk_level: medium
`

// ---------- Spec tests ----------

func TestLoadSpec(t *testing.T) {
	path := writeTempYAML(t, validYAML)

	spec, err := LoadSpec(path)
	require.NoError(t, err)

	// Top-level fields.
	assert.Equal(t, "github", spec.Name)
	assert.Equal(t, "http_api", spec.Type)
	assert.Equal(t, "1.0", spec.SpecVersion)
	assert.Equal(t, "v3", spec.APIVersion)
	assert.Equal(t, "https://api.github.com", spec.BaseURL)

	// Auth.
	assert.Equal(t, "bearer_token", spec.Auth.Type)
	assert.Equal(t, "GITHUB_TOKEN", spec.Auth.EnvVar)

	// CircuitBreaker config.
	assert.Equal(t, 3, spec.CircuitBreaker.FailureThreshold)
	assert.Equal(t, 30, spec.CircuitBreaker.CooldownSeconds)

	// Rate limit group.
	assert.Equal(t, "github_core", spec.RateLimitGroup)

	// Endpoints.
	require.Len(t, spec.Endpoints, 2)
	lr := spec.Endpoints["list_repos"]
	assert.Equal(t, "/user/repos", lr.Path)
	assert.Equal(t, "GET", lr.Method)
	assert.Equal(t, "10s", lr.Timeout)
	assert.Equal(t, "native", lr.IdempotencySupport)

	ci := spec.Endpoints["create_issue"]
	assert.Equal(t, "/repos/{owner}/{repo}/issues", ci.Path)
	assert.Equal(t, "POST", ci.Method)
	assert.Equal(t, "15s", ci.Timeout)
	assert.Equal(t, "client_generated", ci.IdempotencySupport)

	// Allowed agents.
	assert.Equal(t, []string{"code_agent", "review_agent"}, spec.AllowedAgents)

	// Risk level.
	assert.Equal(t, "medium", spec.RiskLevel)
}

func TestLoadSpecInvalid(t *testing.T) {
	t.Run("malformed yaml", func(t *testing.T) {
		path := writeTempYAML(t, "{{{{not yaml")
		_, err := LoadSpec(path)
		assert.Error(t, err)
	})

	t.Run("missing name", func(t *testing.T) {
		path := writeTempYAML(t, `type: http_api`)
		_, err := LoadSpec(path)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name")
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := LoadSpec("/nonexistent/path/spec.yaml")
		assert.Error(t, err)
	})
}

// ---------- CircuitBreaker tests ----------

func TestCircuitBreakerClosed(t *testing.T) {
	cb := NewCircuitBreaker(CBConfig{FailureThreshold: 3, CooldownSeconds: 30})

	// CLOSED state allows requests.
	assert.True(t, cb.Allow())
	assert.Equal(t, CBClosed, cb.Status().State)

	// Record a failure — should still be CLOSED (threshold=3).
	cb.RecordFailure()
	assert.Equal(t, CBClosed, cb.Status().State)
	assert.Equal(t, 1, cb.Status().Failures)

	// A success resets the failure counter.
	cb.RecordSuccess()
	assert.Equal(t, 0, cb.Status().Failures)
	assert.Equal(t, CBClosed, cb.Status().State)
}

func TestCircuitBreakerOpenAfterFailures(t *testing.T) {
	cb := NewCircuitBreaker(CBConfig{FailureThreshold: 3, CooldownSeconds: 30})

	// Accumulate failures to reach threshold.
	cb.RecordFailure()
	cb.RecordFailure()
	assert.Equal(t, CBClosed, cb.Status().State) // still CLOSED at 2

	cb.RecordFailure() // 3rd failure → OPEN
	assert.Equal(t, CBOpen, cb.Status().State)

	// OPEN state blocks requests.
	assert.False(t, cb.Allow())
	assert.False(t, cb.Allow())
}

func TestCircuitBreakerHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(CBConfig{FailureThreshold: 2, CooldownSeconds: 1})

	// Trip the breaker.
	cb.RecordFailure()
	cb.RecordFailure()
	require.Equal(t, CBOpen, cb.Status().State)

	// Wait for cooldown to expire.
	time.Sleep(1100 * time.Millisecond)

	// After cooldown, Allow() should return true (HALF_OPEN probe).
	assert.True(t, cb.Allow())
	assert.Equal(t, CBHalfOpen, cb.Status().State)

	// While probe is in flight, further Allow() calls should be blocked.
	assert.False(t, cb.Allow())
}

func TestCircuitBreakerRecovering(t *testing.T) {
	cb := NewCircuitBreaker(CBConfig{FailureThreshold: 2, CooldownSeconds: 1})

	// Trip → OPEN → wait → HALF_OPEN.
	cb.RecordFailure()
	cb.RecordFailure()
	require.Equal(t, CBOpen, cb.Status().State)

	time.Sleep(1100 * time.Millisecond)
	require.True(t, cb.Allow()) // probe allowed (HALF_OPEN)

	// Success in HALF_OPEN → RECOVERING.
	cb.RecordSuccess()
	assert.Equal(t, CBRecovering, cb.Status().State)

	// Gradual ramp-up: successes starts at 0, needs to reach 1, 2, 4.
	// After 1st success in RECOVERING → successes=1 (threshold 1 met, allow next).
	assert.True(t, cb.Allow()) // allowed (successes=0 < threshold 1)
	cb.RecordSuccess()         // successes=1

	assert.True(t, cb.Allow()) // allowed (successes=1 < threshold 2)
	cb.RecordSuccess()         // successes=2

	// Need 2 more successes to reach threshold 4.
	assert.True(t, cb.Allow())
	cb.RecordSuccess() // successes=3
	assert.True(t, cb.Allow())
	cb.RecordSuccess() // successes=4 → CLOSED

	assert.Equal(t, CBClosed, cb.Status().State)
	assert.Equal(t, 0, cb.Status().Failures)

	// Failure during RECOVERING → OPEN.
	// Re-trip to test this path.
	cb2 := NewCircuitBreaker(CBConfig{FailureThreshold: 2, CooldownSeconds: 1})
	cb2.RecordFailure()
	cb2.RecordFailure()
	time.Sleep(1100 * time.Millisecond)
	require.True(t, cb2.Allow())
	cb2.RecordSuccess() // → RECOVERING
	require.Equal(t, CBRecovering, cb2.Status().State)

	cb2.RecordFailure() // should go back to OPEN
	assert.Equal(t, CBOpen, cb2.Status().State)
}

// ---------- Registry tests ----------

func TestRegistry(t *testing.T) {
	reg := NewRegistry()

	specA := &ConnectorSpec{
		Name: "alpha",
		Type: "http_api",
		CircuitBreaker: CBConfig{
			FailureThreshold: 3,
			CooldownSeconds:  30,
		},
	}
	specB := &ConnectorSpec{
		Name: "beta",
		Type: "http_api",
		CircuitBreaker: CBConfig{
			FailureThreshold: 5,
			CooldownSeconds:  60,
		},
	}

	// Register.
	require.NoError(t, reg.Register(specA))
	require.NoError(t, reg.Register(specB))

	// Duplicate register should fail.
	assert.Error(t, reg.Register(specA))

	// Get.
	got, ok := reg.Get("alpha")
	require.True(t, ok)
	assert.Equal(t, "alpha", got.Name)

	_, ok = reg.Get("nonexistent")
	assert.False(t, ok)

	// List — sorted by name.
	list := reg.List()
	require.Len(t, list, 2)
	names := make([]string, len(list))
	for i, s := range list {
		names[i] = s.Name
	}
	assert.True(t, sort.StringsAreSorted(names), "List() should return specs sorted by name")
	assert.Equal(t, "alpha", names[0])
	assert.Equal(t, "beta", names[1])

	// BreakerStatus.
	status, ok := reg.BreakerStatus("alpha")
	require.True(t, ok)
	assert.Equal(t, CBClosed, status.State)
	assert.Equal(t, 0, status.Failures)

	_, ok = reg.BreakerStatus("nonexistent")
	assert.False(t, ok)
}

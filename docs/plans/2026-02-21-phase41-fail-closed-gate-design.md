# Phase 41: Fail-Closed Gate

> Design doc for Apex Agent CLI — highest-priority safety gate that blocks operations when system state is unhealthy.

## Problem

The system has no fail-closed mechanism. When components are unavailable or degraded (health RED, kill switch active, sandbox unavailable), high-risk operations can still proceed. The architecture requires a highest-priority gate that defaults to denying execution when safety cannot be verified.

## Solution

A `failclose` package with a `Gate` that evaluates multiple `Condition` checks before allowing execution. If ANY condition fails, the gate blocks the operation. The gate follows a fail-closed principle: uncertainty = denial.

## Architecture

```
internal/failclose/
├── failclose.go       # Condition, GateResult, Gate
└── failclose_test.go  # 7 unit tests
```

## Key Types

### Condition

```go
type Condition struct {
    Name    string                `json:"name"`
    Check   func() (bool, string) `json:"-"` // returns (pass, reason)
}
```

### GateResult

```go
type GateResult struct {
    Allowed  bool             `json:"allowed"`
    Failures []ConditionResult `json:"failures"`
    Passed   []ConditionResult `json:"passed"`
}

type ConditionResult struct {
    Name   string `json:"name"`
    Passed bool   `json:"passed"`
    Reason string `json:"reason"`
}
```

### Gate

```go
type Gate struct {
    mu         sync.RWMutex
    conditions []Condition
}
```

## Core Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `NewGate` | `() *Gate` | Creates empty gate |
| `(*Gate) AddCondition` | `(c Condition)` | Adds a condition check |
| `(*Gate) Evaluate` | `() GateResult` | Runs all conditions, returns result |
| `(*Gate) MustPass` | `() error` | Evaluate + return error if not allowed |
| `(*Gate) Conditions` | `() []string` | Returns condition names |
| `DefaultGate` | `() *Gate` | Creates gate with standard conditions (health, killswitch) |
| `HealthCondition` | `(status string) Condition` | Returns condition checking health != RED/CRITICAL |
| `KillSwitchCondition` | `(active bool) Condition` | Returns condition checking kill switch not active |

## Unit Tests (7)

| Test | Description |
|------|-------------|
| `TestNewGate` | Empty gate, no conditions |
| `TestGateAddCondition` | Conditions accumulate |
| `TestGateEvaluateAllPass` | All conditions pass → Allowed=true |
| `TestGateEvaluateOneFail` | One fails → Allowed=false, failure recorded |
| `TestGateEvaluateMultipleFail` | Multiple fail → all failures recorded |
| `TestGateMustPass` | Returns nil on pass, error on fail |
| `TestHealthCondition` | GREEN/YELLOW pass, RED/CRITICAL fail |

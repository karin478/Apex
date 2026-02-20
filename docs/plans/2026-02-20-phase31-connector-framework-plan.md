# Phase 31: Connector Framework — Implementation Plan

**Design:** `2026-02-20-phase31-connector-framework-design.md`

## Task 1: Connector Core — Spec + CircuitBreaker + Registry

**Files:** `internal/connector/connector.go`, `internal/connector/connector_test.go`

**Tests (7):**
1. `TestLoadSpec` — Parse valid YAML, verify all fields
2. `TestLoadSpecInvalid` — Invalid YAML returns error
3. `TestCircuitBreakerClosed` — Normal state allows, RecordFailure increments
4. `TestCircuitBreakerOpenAfterFailures` — Failures >= threshold → OPEN, Allow() returns false
5. `TestCircuitBreakerHalfOpen` — After cooldown, Allow() returns true (HALF_OPEN)
6. `TestCircuitBreakerRecovering` — Success in HALF_OPEN → RECOVERING, gradual ramp 1→2→4→full → CLOSED
7. `TestRegistry` — Register/Get/List/BreakerStatus

**TDD workflow:** Write all 7 tests first, then implement.

## Task 2: Format + CLI — `apex connector list/status`

**Files:** `internal/connector/format.go`, `internal/connector/format_test.go`, `cmd/apex/connector.go`

**Format functions:**
- `FormatConnectorList(specs []*ConnectorSpec) string` — table: name, type, base_url, risk_level
- `FormatBreakerStatus(statuses map[string]CBStatus) string` — table: name, state, failures, cooldown
- `FormatBreakerStatusJSON(statuses map[string]CBStatus) string`

**CLI:**
- `apex connector list` — load specs from ~/.claude/connectors/*.yaml, display list
- `apex connector status [--format json]` — show circuit breaker status

## Task 3: E2E Tests

**File:** `e2e/connector_test.go`

**Tests (3):**
1. `TestConnectorList` — Create temp connector YAML, run list
2. `TestConnectorStatusEmpty` — No connectors, shows empty message
3. `TestConnectorStatusRuns` — Command exits code 0

## Task 4: PROGRESS.md Update

# E2E Testing Module Design

**Date:** 2026-02-19 | **Phase:** E2E Testing Infrastructure | **Status:** Approved

## Goal

Build a comprehensive end-to-end testing module that invokes the real `apex` CLI binary, validates all commands and subsystems work together correctly, and produces a summary report. The module serves as the project's regression gate — run it after any change to confirm nothing is broken.

## Architecture

Two-layer testing strategy:

1. **Mock mode (default):** Builds `apex` binary, replaces `claude` with a configurable mock script. Fast (~10s), no token cost, covers all code paths.
2. **Live mode (`-tags=live`):** Calls real Claude CLI for smoke tests. Validates true end-to-end flow, consumes minimal tokens.

## Directory Layout

```
e2e/
├── testdata/
│   └── mock_claude.sh          # Mock claude binary (env-driven behavior)
├── setup_test.go               # TestMain: compile apex, prepare mock, isolated HOME
├── helpers_test.go             # Shared helpers: runApex(), assertOutput(), etc.
├── run_test.go                 # apex run scenarios
├── plan_test.go                # apex plan scenarios
├── doctor_test.go              # apex doctor scenarios
├── killswitch_test.go          # kill switch scenarios
├── snapshot_test.go            # snapshot scenarios
├── status_test.go              # apex status / history scenarios
├── retry_test.go               # retry / fault tolerance scenarios
├── config_test.go              # config edge cases (missing file, defaults)
└── live_test.go                # //go:build live — real Claude smoke tests
```

## Mock Claude Binary

A shell script (`mock_claude.sh`) that simulates the `claude` CLI. Behavior is controlled via environment variables:

| Env Var | Purpose | Default |
|---------|---------|---------|
| `MOCK_RESPONSE` | JSON string returned on stdout | `{"result": "mock ok"}` |
| `MOCK_EXIT_CODE` | Process exit code | `0` |
| `MOCK_DELAY_MS` | Sleep before responding (ms) | `0` |
| `MOCK_STDERR` | Content written to stderr | (empty) |
| `MOCK_PLANNER_RESPONSE` | Response specifically for planner calls (detected by task content) | Single-node DAG JSON |

The mock detects whether it's being called by the planner (task decomposition) or executor (task execution) by inspecting the prompt argument, and returns the appropriate response.

### Mock Planner Default Response

When the mock detects a planning call, it returns a single-node DAG:

```json
[{"id": "task_1", "task": "<the original task>", "depends": []}]
```

This can be overridden with `MOCK_PLANNER_RESPONSE` for multi-node DAG tests.

## Environment Isolation

Each test function gets a fully isolated environment:

```go
func setupTestEnv(t *testing.T) *TestEnv {
    home := t.TempDir()
    workDir := t.TempDir()
    // Initialize git repo in workDir (needed for snapshot)
    // Create ~/.apex/ directory structure
    // Write config.yaml pointing claude.binary to mock
    // Return TestEnv with paths and helper methods
}
```

Key isolation guarantees:
- `HOME` set to temp dir → `~/.apex/` is isolated
- Working directory is a fresh git repo
- Config's `claude.binary` points to `mock_claude.sh`
- No pollution between tests or to real user environment

## Test Scenarios

### apex run (run_test.go)

| Test | Setup | Assert |
|------|-------|--------|
| `TestRunHappyPath` | Simple task, mock returns success | Exit 0, audit file created, manifest written |
| `TestRunDryRun` | `--dry-run` flag | Exit 0, mock claude NOT called for execution, plan output shown |
| `TestRunMultiNodeDAG` | Mock planner returns 3-node DAG | All 3 nodes executed in dependency order |
| `TestRunWithFailure` | Mock returns exit=1 + "timeout" stderr | Retry triggered, then success on 2nd attempt |
| `TestRunNonRetriableFailure` | Mock returns exit=2 | Immediate failure, no retry, error reported |
| `TestRunRetryExhausted` | Mock always fails with retriable error | Fails after max_attempts, reports "failed after N attempts" |
| `TestRunMissingConfig` | No config file | Runs with defaults successfully |

### apex plan (plan_test.go)

| Test | Setup | Assert |
|------|-------|--------|
| `TestPlanSimpleTask` | Short task | Shows plan output with single node |
| `TestPlanComplexTask` | Mock returns multi-node DAG | Shows DAG structure with dependencies |

### apex doctor (doctor_test.go)

| Test | Setup | Assert |
|------|-------|--------|
| `TestDoctorHealthy` | Valid audit chain | Reports healthy |
| `TestDoctorCorruptedAudit` | Tampered audit file | Detects corruption |
| `TestDoctorNoAuditDir` | Empty ~/.apex/ | Handles gracefully |

### Kill Switch (killswitch_test.go)

| Test | Setup | Assert |
|------|-------|--------|
| `TestKillSwitchActivate` | `apex kill-switch "test reason"` | File created, subsequent run blocked |
| `TestKillSwitchResume` | Activate then `apex resume` | File removed, runs work again |
| `TestKillSwitchDuringRun` | Start run, create file mid-execution | Run interrupted |

### Snapshot (snapshot_test.go)

| Test | Setup | Assert |
|------|-------|--------|
| `TestSnapshotList` | Run a task (creates snapshot) | `apex snapshot list` shows entry |
| `TestSnapshotRestore` | Run, modify files, restore | Files restored to pre-run state |

### Status/History (status_test.go)

| Test | Setup | Assert |
|------|-------|--------|
| `TestStatusAfterRun` | Run a task | `apex status` shows the run |
| `TestHistoryAfterRun` | Run a task | `apex history` shows audit entries |

### Retry (retry_test.go)

| Test | Setup | Assert |
|------|-------|--------|
| `TestRetrySuccessOnSecondAttempt` | Mock fails once then succeeds | Task completes, 2 attempts logged |
| `TestRetryBackoffTiming` | Mock fails with delays | Backoff delay observed between attempts |

### Config (config_test.go)

| Test | Setup | Assert |
|------|-------|--------|
| `TestDefaultConfig` | No config.yaml | All defaults applied, runs normally |
| `TestCustomConfig` | Custom yaml with overrides | Overrides respected |

### Live Smoke (live_test.go) — `//go:build live`

| Test | Setup | Assert |
|------|-------|--------|
| `TestLiveRunSimple` | `apex run "echo hello world"` with real Claude | Exit 0, produces output |
| `TestLivePlan` | `apex plan "create a todo app"` | Outputs DAG preview |
| `TestLiveDoctor` | `apex doctor` | No errors |

## Helpers API

```go
type TestEnv struct {
    Home     string  // Isolated HOME directory
    WorkDir  string  // Git-initialized working directory
    ApexBin  string  // Path to compiled apex binary
    MockBin  string  // Path to mock_claude.sh
    ConfigPath string // Path to config.yaml
}

// runApex executes apex with args, returns stdout, stderr, exit code
func (e *TestEnv) runApex(t *testing.T, args ...string) (stdout, stderr string, exitCode int)

// runApexWithEnv executes apex with extra environment variables (for mock control)
func (e *TestEnv) runApexWithEnv(t *testing.T, env map[string]string, args ...string) (stdout, stderr string, exitCode int)

// writeConfig writes a custom config.yaml
func (e *TestEnv) writeConfig(t *testing.T, yaml string)

// fileExists checks if a file exists relative to Home
func (e *TestEnv) fileExists(t *testing.T, relPath string) bool

// readFile reads a file relative to Home
func (e *TestEnv) readFile(t *testing.T, relPath string) string
```

## Makefile Targets

```makefile
e2e:
	go test ./e2e/... -v -count=1 -timeout=120s

e2e-live:
	go test ./e2e/... -v -count=1 -tags=live -timeout=300s
```

## Report Output

Tests use Go's standard `testing` output. The `-v` flag provides per-test PASS/FAIL. For CI or scripted use:

```bash
go test ./e2e/... -v -count=1 -json > e2e/report.json
```

This produces JSON Lines format compatible with `gotestsum` and other Go test reporters.

## Design Decisions

1. **Why shell script mock, not Go binary?** — Simpler to maintain, no compilation needed, easy to control via env vars. The mock doesn't need complex logic.

2. **Why per-test isolation?** — Tests that modify filesystem state (kill switch, snapshots) must not interfere with each other. TempDir gives automatic cleanup.

3. **Why build tag for live tests?** — Live tests consume tokens and require Claude CLI installed. They should never run accidentally in CI or during development.

4. **Why TestMain for compilation?** — Building the binary once in TestMain avoids recompilation per test, keeping the suite fast.

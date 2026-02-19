# Phase 16: System Health Level — Design Document

> Date: 2026-02-19
> Status: Approved
> Architecture Ref: v11.0 §2.14 System Health Level — 全局降级状态机

## 1. Goal

Implement a global degradation state machine (GREEN/YELLOW/RED/CRITICAL) that evaluates multiple component statuses to provide a unified health view. Restrict operations based on health level: RED allows only LOW-risk operations; CRITICAL is fail-closed (only `doctor`/`resume`).

## 2. Package: `internal/health`

### 2.1 Core API

```go
// Level represents the system health level.
type Level int

const (
    GREEN    Level = iota // All components healthy
    YELLOW                // 1 important component degraded
    RED                   // 1 critical or 2+ important components degraded
    CRITICAL              // 2+ critical components degraded
)

// ComponentStatus represents the health status of a single component.
type ComponentStatus struct {
    Name     string // e.g. "audit_chain", "sandbox_available"
    Category string // "critical", "important", "optional"
    Healthy  bool
    Detail   string // Human-readable description
}

// Report is the result of a health evaluation.
type Report struct {
    Level      Level
    Components []ComponentStatus
}

// Evaluate runs all health checks and returns a comprehensive report.
func Evaluate(baseDir string) *Report
```

### 2.2 Component Health Contribution Matrix

Per architecture §2.14, components are categorized:

**Critical** (any single degradation → RED, 2+ → CRITICAL):

| Component | Check |
|-----------|-------|
| `audit_chain` | Hash chain integrity via `audit.Logger.Verify()` |
| `sandbox_available` | At least ulimit sandbox available via `sandbox.Detect()` |
| `config_valid` | `config.Load()` succeeds without error |

**Important** (1 degraded → YELLOW, 2+ degraded → RED):

| Component | Check |
|-----------|-------|
| `kill_switch` | Kill switch file does not exist |
| `audit_dir` | `~/.apex/audit/` directory exists and is writable |
| `memory_dir` | `~/.apex/memory/` directory exists and is writable |

**Optional** (degradation logged only, no level impact):

| Component | Check |
|-----------|-------|
| `git_repo` | Current working directory is a git repository |

> YAGNI: Only check components that are currently implemented. NLI, vector_search, Docker backend, credential_store deferred to future phases.

### 2.3 Health Level Determination

```
criticalFailed = count of unhealthy critical components
importantFailed = count of unhealthy important components

if criticalFailed >= 2:           CRITICAL
else if criticalFailed == 1:      RED
else if importantFailed >= 2:     RED
else if importantFailed == 1:     YELLOW
else:                             GREEN
```

## 3. Integration Points

### 3.1 `apex doctor` Enhancement

Append health report to existing doctor output:

```
System Health: GREEN ✓
  [✓] audit_chain        Hash chain intact
  [✓] sandbox_available  Ulimit sandbox available
  [✓] config_valid       Configuration loaded
  [✓] kill_switch        Not active
  [✓] audit_dir          Writable
  [✓] memory_dir         Writable
  [✓] git_repo           Git repository detected
```

When degraded:
```
System Health: RED ✗
  [✗] audit_chain        BROKEN at record #42
  [✓] sandbox_available  Ulimit sandbox available
  [✓] config_valid       Configuration loaded
  [✗] kill_switch        ACTIVE — use 'apex resume' to deactivate
  ...
```

### 3.2 `apex status` Enhancement

Add health level line to status output.

### 3.3 `apex run` Pre-flight Gate

Before task execution:
- If health == CRITICAL: refuse all tasks, print "System health CRITICAL — run 'apex doctor' to diagnose"
- If health == RED and risk > LOW: refuse, print "System health RED — only LOW-risk tasks allowed"
- Otherwise: proceed normally

## 4. Files

| File | Purpose |
|------|---------|
| `internal/health/health.go` | Level enum, types, Evaluate(), level determination |
| `internal/health/checks.go` | Individual component check functions |
| `internal/health/health_test.go` | Unit tests |
| `cmd/apex/doctor.go` | Integrate health report display |
| `cmd/apex/run.go` | Add health pre-flight gate |
| `cmd/apex/status.go` | Display health level |
| `e2e/health_test.go` | E2E tests |

## 5. Performance

- Health Level calculation: < 10ms (per architecture §2.14)
- `apex doctor` total: < 10s (per architecture performance targets)
- Individual checks should be non-blocking and fast

## 6. Out of Scope

- NLI service health check (no NLI implemented yet)
- Vector search health check (deferred)
- Docker/container backend check (deferred)
- Credential store check (deferred)
- Health level persistence to database (deferred — in-memory evaluation only)
- Dashboard / TUI display (Phase 17+ candidate)
- Automatic gc trigger on degradation (deferred)

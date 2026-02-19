# Phase 16: System Health Level — Implementation Plan

> Date: 2026-02-19
> Design Doc: `2026-02-19-phase16-system-health-level-design.md`
> Method: Subagent-Driven Development (TDD)

## Tasks

### Task 1: Health Core + Level Determination

**Create:** `internal/health/health.go`, `internal/health/health_test.go`

**Data model:**
```go
type Level int
const (GREEN Level = iota; YELLOW; RED; CRITICAL)

type ComponentStatus struct {
    Name     string
    Category string // "critical", "important", "optional"
    Healthy  bool
    Detail   string
}

type Report struct {
    Level      Level
    Components []ComponentStatus
}
```

**Functions:**
- `(l Level) String() string` — "GREEN", "YELLOW", "RED", "CRITICAL"
- `Determine(components []ComponentStatus) Level` — pure logic, no I/O
- `NewReport(components []ComponentStatus) *Report` — creates report with computed level

**Tests (TDD):**
1. `TestLevelString` — all 4 levels render correct strings
2. `TestDetermineAllHealthy` — all components healthy → GREEN
3. `TestDetermineOneImportantFailed` — 1 important fails → YELLOW
4. `TestDetermineTwoImportantFailed` — 2 important fail → RED
5. `TestDetermineOneCriticalFailed` — 1 critical fails → RED
6. `TestDetermineTwoCriticalFailed` — 2 critical fail → CRITICAL
7. `TestDetermineMixedFailures` — 1 critical + 1 important → RED (critical dominates)
8. `TestDetermineOptionalIgnored` — optional failures don't affect level
9. `TestNewReport` — report level matches Determine output

**Commit:** `feat(health): add health Level enum and Determine logic`

---

### Task 2: Component Health Checks

**Create:** `internal/health/checks.go`
**Append to:** `internal/health/health_test.go`

**Functions — each returns a `ComponentStatus`:**
- `CheckAuditChain(baseDir string) ComponentStatus` — creates audit.Logger, calls Verify()
- `CheckSandbox() ComponentStatus` — calls sandbox.Detect(), checks level >= Ulimit
- `CheckConfig(baseDir string) ComponentStatus` — loads config, checks no error
- `CheckKillSwitch(baseDir string) ComponentStatus` — checks kill switch file does not exist
- `CheckAuditDir(baseDir string) ComponentStatus` — checks audit dir exists and writable
- `CheckMemoryDir(baseDir string) ComponentStatus` — checks memory dir exists and writable
- `CheckGitRepo() ComponentStatus` — runs `git rev-parse --git-dir` in cwd
- `Evaluate(baseDir string) *Report` — runs all checks, returns report

**Tests:**
1. `TestCheckAuditChainHealthy` — valid audit dir → healthy
2. `TestCheckAuditChainNoDir` — no audit dir → unhealthy
3. `TestCheckSandbox` — sandbox.Detect() returns something → healthy
4. `TestCheckConfigValid` — valid config → healthy
5. `TestCheckKillSwitchInactive` — no kill switch file → healthy
6. `TestCheckKillSwitchActive` — kill switch file exists → unhealthy
7. `TestCheckDirWritable` — writable dir → healthy
8. `TestCheckDirMissing` — missing dir → unhealthy
9. `TestEvaluateAllHealthy` — all pass → GREEN report
10. `TestEvaluateWithDegradation` — force a failure → correct level

**Commit:** `feat(health): add component checks and Evaluate function`

---

### Task 3: Doctor Integration

**Modify:** `cmd/apex/doctor.go`

**Changes:**
- Import `internal/health`
- After existing checks, call `health.Evaluate(baseDir)`
- Print health report with colored status indicators
- Format: `System Health: GREEN ✓` / `RED ✗` with component details

**Commit:** `feat(cli): integrate health report into apex doctor`

---

### Task 4: Run Pre-flight Gate + Status Integration

**Modify:** `cmd/apex/run.go`, `cmd/apex/status.go`

**run.go changes:**
- After config load but before risk classification, evaluate health
- If CRITICAL: return error "System health CRITICAL — run 'apex doctor'"
- If RED and risk > LOW: return error "System health RED — only LOW-risk tasks allowed"

**status.go changes:**
- Add health level line to status output

**Commit:** `feat(cli): add health pre-flight gate to run and status`

---

### Task 5: E2E Tests

**Create:** `e2e/health_test.go`

**Tests:**
1. `TestDoctorShowsHealth` — `apex doctor` output contains "System Health:" and "GREEN"
2. `TestDoctorDegradedHealth` — create kill switch file, run doctor, verify YELLOW
3. `TestRunBlockedByCriticalHealth` — corrupt audit chain + create kill switch → run rejected
4. `TestStatusShowsHealth` — `apex status` output contains health level

**Commit:** `test(e2e): add system health level E2E tests`

---

### Task 6: Update PROGRESS.md

**Modify:** `PROGRESS.md`

- Add Phase 16 row as Done
- Update Current to Phase 17 — TBD
- Update test counts
- Add `internal/health` to Key Packages

**Commit:** `docs: mark Phase 16 System Health Level as complete`

## Summary

| Task | Files | Tests | Description |
|------|-------|-------|-------------|
| 1 | health.go, health_test.go | 9 | Core Level enum + Determine logic |
| 2 | checks.go, health_test.go | 10 | Component checks + Evaluate |
| 3 | doctor.go | — | Doctor integration |
| 4 | run.go, status.go | — | Pre-flight gate + status |
| 5 | health_test.go (e2e) | 4 | E2E tests |
| 6 | PROGRESS.md | — | Documentation |
| **Total** | **8 files** | **23 new tests** | |

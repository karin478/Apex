# Phase 17: Causal Chain / Tracing — Implementation Plan

> Date: 2026-02-20
> Design Doc: `2026-02-20-phase17-causal-chain-tracing-design.md`
> Method: Subagent-Driven Development (TDD)

## Tasks

### Task 1: TraceContext Package

**Create:** `internal/trace/trace.go`, `internal/trace/trace_test.go`

**Types:**
```go
type TraceContext struct {
    TraceID        string
    ParentActionID string
}
```

**Functions:**
- `NewTrace() TraceContext` — generates UUID v4 for TraceID, empty ParentActionID
- `(tc TraceContext) Child(actionID string) TraceContext` — returns new context with same TraceID, actionID as ParentActionID

**Tests:**
1. `TestNewTrace` — TraceID is non-empty UUID format, ParentActionID is empty
2. `TestChild` — child inherits TraceID, ParentActionID set to given actionID
3. `TestChildChain` — grandchild: root → child → grandchild all share same TraceID
4. `TestNewTraceUnique` — two calls produce different TraceIDs

**Commit:** `feat(trace): add TraceContext with NewTrace and Child`

---

### Task 2: Audit Record Extension + FindByTraceID

**Modify:** `internal/audit/logger.go`, `internal/audit/logger_test.go`

**Changes to Entry struct:**
```go
type Entry struct {
    // ... existing fields ...
    TraceID        string
    ParentActionID string
}
```

**Changes to Record struct:**
```go
type Record struct {
    // ... existing fields ...
    TraceID        string `json:"trace_id,omitempty"`
    ParentActionID string `json:"parent_action_id,omitempty"`
}
```

**Changes to Log():**
- Map `entry.TraceID` → `record.TraceID`
- Map `entry.ParentActionID` → `record.ParentActionID`
- These are set BEFORE hash computation (so hash covers trace fields)

**New method:**
```go
func (l *Logger) FindByTraceID(traceID string) ([]Record, error)
```
Scans all date-named .jsonl files, returns records where TraceID matches, sorted by timestamp ascending.

**Tests:**
1. `TestLogWithTraceFields` — log entry with TraceID + ParentActionID, read back, verify fields present
2. `TestLogTraceFieldsInHash` — hash covers trace fields (changing TraceID changes hash)
3. `TestFindByTraceID` — log 3 entries (2 with same trace, 1 different), FindByTraceID returns only matching 2
4. `TestFindByTraceIDEmpty` — no matching trace → empty slice
5. `TestFindByTraceIDOrder` — results sorted by timestamp

**Commit:** `feat(audit): add trace_id and parent_action_id to audit records`

---

### Task 3: Manifest Extension

**Modify:** `internal/manifest/manifest.go`, `internal/manifest/manifest_test.go`

**Changes:**
- Add `TraceID string` field to `Manifest` struct with `json:"trace_id,omitempty"` tag
- Add `ActionID string` field to `NodeResult` struct with `json:"action_id,omitempty"` tag

**Tests:**
1. `TestManifestTraceID` — create manifest with TraceID, save, load, verify field preserved
2. `TestNodeResultActionID` — node result with ActionID serializes correctly

**Commit:** `feat(manifest): add trace_id to manifest and action_id to node results`

---

### Task 4: Run Command Integration

**Modify:** `cmd/apex/run.go`

**Changes:**
1. Import `"github.com/lyndonlyu/apex/internal/trace"`
2. After config load (early in runTask), create root trace:
   ```go
   tc := trace.NewTrace()
   ```
3. Set manifest TraceID:
   ```go
   m.TraceID = tc.TraceID
   ```
4. For each DAG node audit log, pass trace fields:
   ```go
   logger.Log(audit.Entry{
       // ... existing fields ...
       TraceID:        tc.TraceID,
       ParentActionID: tc.ParentActionID,
   })
   ```
5. Print trace ID at start of run: `fmt.Printf("[trace: %s]\n", tc.TraceID[:8])`

**Commit:** `feat(cli): integrate trace context into run command`

---

### Task 5: Trace CLI Command

**Create:** `cmd/apex/trace.go`
**Modify:** `cmd/apex/main.go`

**Command:** `apex trace [run-id]`
- If run-id given: load manifest, get trace_id, find audit entries, display
- If no run-id: load most recent run's manifest
- Display format:
  ```
  Trace: <trace-id-short> (run: <run-id-short>)

  <action-id-short>  <task>  <outcome>  <duration>ms
  <action-id-short>  <task>  <outcome>  <duration>ms
  ```
- Register `traceCmd` in main.go init()

**Commit:** `feat(cli): add apex trace command for causal chain viewing`

---

### Task 6: E2E Tests

**Create:** `e2e/trace_test.go`

**Tests:**
1. `TestRunShowsTraceID` — run a task, verify stdout contains "[trace:"
2. `TestTraceCommand` — run a task, then `apex trace` (no args), verify it shows trace entries
3. `TestManifestContainsTraceID` — run a task, read manifest JSON, verify trace_id field exists

**Commit:** `test(e2e): add causal chain tracing E2E tests`

---

### Task 7: Update PROGRESS.md

**Modify:** `PROGRESS.md`

- Add Phase 17 row as Done
- Update Current to Phase 18 — TBD
- Update test counts
- Add `internal/trace` to Key Packages

**Commit:** `docs: mark Phase 17 Causal Chain / Tracing as complete`

## Summary

| Task | Files | Tests | Description |
|------|-------|-------|-------------|
| 1 | trace.go, trace_test.go | 4 | TraceContext core |
| 2 | logger.go, logger_test.go | 5 | Audit trace fields + FindByTraceID |
| 3 | manifest.go, manifest_test.go | 2 | Manifest trace extension |
| 4 | run.go | — | Run integration |
| 5 | trace.go (cmd), main.go | — | CLI command |
| 6 | trace_test.go (e2e) | 3 | E2E tests |
| 7 | PROGRESS.md | — | Documentation |
| **Total** | **10 files** | **14 new tests** | |

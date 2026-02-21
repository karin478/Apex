# Phase 49: Correctness & Verification Foundation — Design

**Goal:** Implement the correctness verification layer: Invariant I1-I9 framework, Memory Staged Commit pipeline, DAG state machine completion, and CANCELLED auto-rollback with quality grading.

**Architecture Reference:** v11.0 §2.2 (DAG lifecycle), §2.4 (Memory staged commit), §2.11 (Invariants I1-I9), §6a/6c

**Dependencies:** Phase 48 (filelock, writerq, outbox) must be complete. ✅

---

## 1. Invariant Verification Framework (`internal/invariant/`)

New package providing 9 correctness checkers with a unified Runner.

### Data Structures

```go
type CheckResult struct {
    ID      string // "I1" .. "I9"
    Name    string // human-readable name
    Status  string // PASS / FAIL / SKIP / ERROR
    Detail  string // details or error message
}

type Runner struct {
    db      *sql.DB
    baseDir string
}

func NewRunner(db *sql.DB, baseDir string) *Runner
func (r *Runner) RunAll() []CheckResult
func (r *Runner) Run(ids ...string) []CheckResult
```

### 9 Checkers

| ID | Name | Check | Data Source |
|----|------|-------|-------------|
| I1 | WAL-DB Consistency | WAL COMPLETED → DB actions COMPLETED | WAL JSONL + actions table |
| I2 | Artifact Reference | DB COMPLETED → artifact exists (or MISSING marked) | actions table + artifact store |
| I3 | No Hanging Actions | No STARTED actions older than 1h without terminal status | actions table timestamps |
| I4 | Idempotency | Same action_id has no duplicate executions | actions table GROUP BY |
| I5 | Trace Completeness | Every action traceable to root trace_id, no orphans | actions table + audit |
| I6 | Audit Hash Chain | Adjacent audit records pass prev_hash verification | audit JSONL files |
| I7 | Anchor Consistency | Daily anchor hash matches chain position hash | anchor files |
| I8 | Dual-DB Consistency | Runtime.db ↔ vectors.db sync status | SKIP (vectors.db compensation not yet implemented) |
| I9 | Lock Ordering | No process holds high-order lock while acquiring low-order | filelock audit events |

### Integration Points

- `apex doctor` adds "Invariant checks..." section after outbox health
- `run.go` calls `runner.RunAll()` at startup; FAIL results logged as warnings

---

## 2. Memory Staged Commit (`internal/staging/`)

New package adding a verification pipeline on top of existing `memory.Store`.

### Database Schema

staging_memories table in runtime.db:

```sql
CREATE TABLE IF NOT EXISTS staging_memories (
    id            TEXT PRIMARY KEY,
    content       TEXT NOT NULL,
    category      TEXT NOT NULL,        -- decision / fact / session
    source        TEXT NOT NULL,        -- run_id / manual
    staging_state TEXT NOT NULL DEFAULT 'PENDING',
    confidence    REAL NOT NULL DEFAULT 1.0,
    created_at    TEXT NOT NULL,
    committed_at  TEXT,
    expired_at    TEXT
)
```

### State Machine

```
PENDING ──→ VERIFIED ──→ COMMITTED (write to formal memory store)
   │
   ├──→ UNVERIFIED ──→ COMMITTED (confidence ×0.8)
   │
   ├──→ REJECTED (retained 30 days, then GC)
   │
   └──→ EXPIRED (timeout 1h without processing)
```

### NLI Stub (Keyword Matching)

Phase 49 uses keyword-based conflict detection as NLI stub:

- **CONTRADICTION**: New memory contains negation keywords ("不是"/"错误"/"废弃") AND >50% keyword overlap with existing memory
- **ENTAILMENT**: >80% keyword overlap → dedup (keep newer)
- **NEUTRAL**: Default → coexist

### API

```go
type Stager struct {
    db    *sql.DB
    store *memory.Store
}

func New(db *sql.DB, store *memory.Store) *Stager
func (s *Stager) Stage(content, category, source string) (id string, err error)
func (s *Stager) Verify(id string) error              // PENDING → VERIFIED
func (s *Stager) Reject(id string) error              // PENDING → REJECTED
func (s *Stager) Commit(id string) error              // VERIFIED/UNVERIFIED → COMMITTED
func (s *Stager) CommitAll() (int, error)              // batch commit all VERIFIED
func (s *Stager) ExpireStale(timeout time.Duration) (int, error)
func (s *Stager) ListPending() ([]StagingEntry, error)
```

### Integration Points

- `run.go`: After execution, `memory.SaveSession` replaced with `stager.Stage()` → auto-Verify → Commit
- `apex doctor`: Shows staging health (PENDING count)

---

## 3. DAG State Completion (extend `internal/dag/states.go`)

Add 7 new states to complete architecture v11.0 §2.2 F1 lifecycle.

### New States

```go
const (
    Ready       Status = 8   // Dependencies resolved, waiting for execution slot
    Retrying    Status = 9   // In exponential backoff, waiting for retry
    Resuming    Status = 10  // Resuming from Suspended (30s timeout)
    Replanning  Status = 11  // Change weight ≥1.5, needs re-plan (60s timeout)
    Invalidated Status = 12  // Artifact changed, needs re-execution
    Escalated   Status = 13  // Requires human intervention (terminal)
    NeedsHuman  Status = 14  // Explicit human approval required (terminal)
)
```

### New Transition Methods

| Method | Transition | Condition |
|--------|-----------|-----------|
| `MarkReady(id)` | Pending/Blocked → Ready | All dependencies Completed |
| `MarkRetrying(id)` | Failed → Retrying | Retry policy allows |
| `MarkResuming(id)` | Suspended → Resuming | Change weight < 1.5 |
| `MarkReplanning(id)` | Suspended → Replanning | Change weight ≥ 1.5 |
| `Invalidate(id)` | Completed → Invalidated | Artifact change trigger |
| `Requeue(id)` | Invalidated → Pending | Re-enqueue for execution |
| `Escalate(id)` | Retrying/Resuming/Replanning → Escalated | Timeout or max retries |
| `MarkNeedsHuman(id)` | Failed → NeedsHuman | Non-retriable error |

### Updated Terminal Set

```go
func IsTerminal(s Status) bool {
    return s == Completed || s == Failed || s == Cancelled ||
           s == Skipped || s == Escalated || s == NeedsHuman
}
```

### Design Constraints

- Existing 8 states and all 21 tests remain unchanged
- Purely additive (new file `states_extended.go` or append to `states.go`)
- All existing pool/execution logic continues to work with original states

---

## 4. CANCELLED Auto-Rollback (`internal/dag/rollback.go`)

Automatic snapshot rollback when a run is cancelled or fails, with quality grading.

### Rollback Quality

```go
type RollbackQuality string

const (
    QualityFull       RollbackQuality = "FULL"       // All changes rolled back
    QualityPartial    RollbackQuality = "PARTIAL"     // Some changes rolled back
    QualityStructural RollbackQuality = "STRUCTURAL"  // Structure only, content needs manual check
    QualityNone       RollbackQuality = "NONE"        // No snapshot available
)

type RollbackResult struct {
    Quality   RollbackQuality
    RunID     string
    Restored  int    // files restored
    Detail    string
}

func AttemptRollback(snapMgr *snapshot.Manager, runID string) RollbackResult
```

### Logic

1. On cancel/failure, check if snapshot exists for this run
2. If yes → call `snapMgr.Restore(runID)`, evaluate quality
3. If no → return `QualityNone`
4. Result written to audit log + manifest

### Integration Points

- `run.go`: On `killedBySwitch` or `d.HasFailure()`, attempt rollback
- `Cancel()` transition supports optional rollback callback
- `manifest.Manifest` gains `RollbackQuality` field

### Deferred (future phases)

- CANCELLED_ROLLBACK_FAILED state
- Automatic replan inheritance
- Blast radius limitation

---

## Testing Strategy

### Unit Tests (per package)

| Package | Tests |
|---------|-------|
| `internal/invariant/` | 9 checker tests (one per invariant) + RunAll test + Run(subset) test |
| `internal/staging/` | Stage + Verify + Commit + Reject + Expire + CommitAll + ListPending + NLI stub |
| `internal/dag/` (extended) | 7 new state tests + transition error tests + IsTerminal update |
| `internal/dag/rollback.go` | AttemptRollback with/without snapshot + quality grading |

### E2E Tests

- `TestInvariantChecksOnDoctor` — doctor shows invariant results
- `TestStagingMemoryOnRun` — run creates staging entry, auto-commits
- `TestRollbackOnFailure` — failed run attempts rollback

### Backward Compatibility

- All existing 527 unit tests pass
- All existing 139 E2E tests pass
- No modifications to existing APIs (purely additive)

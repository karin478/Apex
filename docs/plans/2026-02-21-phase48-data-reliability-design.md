# Phase 48: Data Reliability Foundation

**Date**: 2026-02-21
**Architecture Reference**: v11.0 §2.11 (Phase 6a)
**Approach**: Full implementation — all three components as one Phase

## Overview

Three interdependent components forming the data reliability foundation:

1. **Layered Locks** (`internal/filelock/`) — flock-based file locks with ordering enforcement
2. **DB Writer Queue** (`internal/writerq/`) — single-writer goroutine with batch merging
3. **Action Outbox** (`internal/outbox/`) — 7-step WAL protocol with startup reconciliation

Dependency chain: Outbox → Writer Queue → Locks

## 1. Layered Locks (`internal/filelock/`)

### Files
- `filelock.go` — flock wrapper + Lock Ordering runtime assertions
- `metadata.go` — lock metadata read/write (pid, timestamp, lock_version)
- `filelock_test.go`

### Interface

```go
type Lock struct {
    Path  string
    Order int      // 0=global, 1+=workspace (dict order)
    file  *os.File
}

type Manager struct {
    held []Lock
    mu   sync.Mutex
}

func NewManager() *Manager
func (m *Manager) AcquireGlobal(baseDir string) (*Lock, error)
func (m *Manager) AcquireWorkspace(wsDir string) (*Lock, error)
func (m *Manager) Release(lock *Lock) error
func (m *Manager) HeldLocks() []LockInfo
```

### Behavior
- **Lock Ordering**: Acquire workspace lock when holding no lock or global lock only. Violation → error + Audit LOCK_ORDER_VIOLATION
- **Non-blocking**: `flock(LOCK_EX | LOCK_NB)`, returns ErrLocked with holder PID
- **Metadata**: `.meta` file beside lock: `{pid, timestamp, lock_order_position, lock_version: 1}`
- **Crash recovery**: flock auto-releases; startup checks `.meta` PID liveness
- **Paths**: `~/.apex/runtime/apex.lock` (global), `{ws}/.apex/runtime/ws.lock` (workspace)

## 2. DB Writer Queue (`internal/writerq/`)

### Files
- `writerq.go` — single writer goroutine + batch channel
- `writerq_test.go`

### Interface

```go
type Op struct {
    SQL    string
    Args   []any
    Result chan error
}

type Queue struct {
    db      *sql.DB
    ops     chan Op
    stop    chan struct{}
    wg      sync.WaitGroup
    crashes int
}

func New(db *sql.DB, opts ...Option) *Queue
func (q *Queue) Submit(ctx context.Context, sql string, args ...any) error
func (q *Queue) SubmitBatch(ctx context.Context, ops []Op) error
func (q *Queue) Close() error
```

### Behavior
- **Single writer goroutine**: Drains channel every 50ms or when batch reaches 100 ops
- **Batch transaction**: All ops in one `BEGIN...COMMIT`
- **Back-pressure**: Channel capacity 1000, Submit blocks when full
- **Crash recovery**: Writer panic → auto-restart (max 3 times, 1s interval). All 3 fail → write Kill Switch
- **Read path unchanged**: Callers read directly via `db.Query()` (WAL allows concurrent reads)

### statedb Integration
- `statedb.SetState/InsertRun/UpdateRunStatus` route writes through Queue
- Read methods (`GetState/ListRuns`) unchanged (direct reads)
- New `statedb.NewWithQueue(db, queue)` constructor

## 3. Action Outbox (`internal/outbox/`)

### Files
- `outbox.go` — WAL file management + 7-step protocol
- `reconcile.go` — startup reconciliation
- `outbox_test.go`

### Interface

```go
type Entry struct {
    ActionID  string    `json:"action_id"`
    TraceID   string    `json:"trace_id"`
    Task      string    `json:"task"`
    Status    string    `json:"status"`    // STARTED | COMPLETED | FAILED
    Timestamp time.Time `json:"timestamp"`
    ResultRef string    `json:"result_ref,omitempty"`
}

type Outbox struct {
    walPath string
    writerq *writerq.Queue
    mu      sync.Mutex
}

func New(walPath string, q *writerq.Queue) *Outbox
func (o *Outbox) Begin(actionID, traceID, task string) error       // Step 1: WAL STARTED
func (o *Outbox) RecordStarted(actionID string) error              // Step 2: DB status=STARTED
func (o *Outbox) Complete(actionID, resultRef string) error        // Steps 4-6: DB+WAL COMPLETED
func (o *Outbox) Fail(actionID, errMsg string) error               // DB+WAL FAILED
func (o *Outbox) Reconcile() ([]Entry, error)                      // Startup: find orphan STARTED
```

### 7-Step Protocol
```
1. WAL append STARTED (fsync)           → o.Begin()
2. runtime.db action status=STARTED     → o.RecordStarted() via writerq
3. Execute task (caller does this)
4. runtime.db status=COMPLETED+result   → o.Complete() via writerq batch
5. Artifact write: .tmp/ → fsync → rename (caller handles)
6. WAL append COMPLETED (fsync)         → o.Complete()
7. Audit + manifest (caller handles, can be async)
```

### WAL File
- Path: `~/.apex/runtime/actions_wal.jsonl`
- Append-only JSONL, each line is an Entry
- `O_APPEND | O_WRONLY | O_CREATE`, fsync after each write

### Startup Reconciliation
- Scan WAL for entries where STARTED has no matching COMPLETED/FAILED
- Return orphan list → caller decides (NEEDS_HUMAN or retry)
- Also checks: DB actions with status=STARTED but no WAL COMPLETED

### New runtime.db Table

```sql
CREATE TABLE IF NOT EXISTS actions (
    action_id TEXT PRIMARY KEY,
    trace_id TEXT NOT NULL,
    task TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'STARTED',
    result_ref TEXT,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    error TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_actions_trace ON actions(trace_id);
CREATE INDEX idx_actions_status ON actions(status);
```

## 4. Integration with run.go

Current flow (simplified):
```
risk → governance → plan → pool.Execute → audit → manifest
```

New flow:
```
risk → governance → lock(global) → plan → lock(workspace)
  → for each node:
      outbox.Begin(actionID)
      outbox.RecordStarted(actionID)
      pool.Execute(node)
      outbox.Complete(actionID) or outbox.Fail(actionID)
  → audit → manifest
  → release locks
```

## 5. doctor Integration

`apex doctor` gains:
- Lock status: holder PID, hold duration, stale lock detection
- Outbox health: count of orphan STARTED entries
- Writer Queue: running/stopped status

## 6. Testing Strategy

| Test | Type | Validates |
|------|------|-----------|
| TestLockOrdering | Unit | Acquire global→workspace OK, reverse fails |
| TestLockNonBlocking | Unit | Second acquire returns ErrLocked |
| TestLockMetadata | Unit | PID/timestamp written and readable |
| TestLockStalePID | Unit | Dead PID detected as stale |
| TestWriterQueueBatch | Unit | Multiple ops batched in one transaction |
| TestWriterQueueBackpressure | Unit | Submit blocks when queue full |
| TestWriterQueueCrashRestart | Unit | Panic recovery + max 3 restarts |
| TestOutboxBeginComplete | Unit | WAL entries written with fsync |
| TestOutboxReconcile | Unit | Orphan STARTED detected |
| TestOutboxDBIntegration | Unit | actions table updated via writerq |
| TestE2EOutboxInRun | E2E | Full run creates WAL + actions entries |
| TestE2EDoctorLocks | E2E | Doctor reports lock status |

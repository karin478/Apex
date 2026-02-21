# Phase 42: Runtime State DB Foundation

> Design doc for Apex Agent CLI — centralized SQLite WAL database for runtime state persistence.

## Problem

The system persists data in scattered file formats: audit logs as JSONL, manifests as JSON, memory as markdown. There is no centralized database for runtime state (current run status, active mode, system flags). The existing migration system (`internal/migration`) is ready but unused. Without a unified DB layer, future features (DAG state tracking, run analytics, resource QoS) have no foundation.

## Solution

A `statedb` package that wraps `*sql.DB` with SQLite WAL mode, creates two foundational tables (`state` for key-value runtime state, `runs` for run records), and provides typed CRUD operations. This is the foundation that future phases build upon.

## Architecture

```
internal/statedb/
├── statedb.go       # DB, StateEntry, RunRecord, Open, CRUD operations
└── statedb_test.go  # 7 unit tests
```

## Key Types

### DB

```go
type DB struct {
    db   *sql.DB
    path string
}
```

### StateEntry

```go
type StateEntry struct {
    Key       string `json:"key"`
    Value     string `json:"value"`
    UpdatedAt string `json:"updated_at"` // RFC3339
}
```

### RunRecord

```go
type RunRecord struct {
    ID        string `json:"id"`
    Status    string `json:"status"`     // PENDING / RUNNING / COMPLETED / FAILED
    TaskCount int    `json:"task_count"`
    StartedAt string `json:"started_at"` // RFC3339
    EndedAt   string `json:"ended_at"`   // RFC3339 or empty
}
```

## Core Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `Open` | `(path string) (*DB, error)` | Opens SQLite with WAL mode, busy timeout 5s, foreign keys ON, creates tables |
| `(*DB) Close` | `() error` | Closes the database connection |
| `(*DB) Path` | `() string` | Returns the database file path |
| `(*DB) SetState` | `(key, value string) error` | Upsert key-value state entry |
| `(*DB) GetState` | `(key string) (StateEntry, error)` | Get state entry by key; ErrNotFound if missing |
| `(*DB) DeleteState` | `(key string) error` | Delete state entry by key |
| `(*DB) ListState` | `() ([]StateEntry, error)` | Returns all state entries sorted by key |
| `(*DB) InsertRun` | `(record RunRecord) error` | Inserts a new run record |
| `(*DB) GetRun` | `(id string) (RunRecord, error)` | Get run by ID; ErrNotFound if missing |
| `(*DB) UpdateRunStatus` | `(id, status string) error` | Updates run status; sets ended_at if COMPLETED/FAILED |
| `(*DB) ListRuns` | `(limit int) ([]RunRecord, error)` | Returns most recent runs, limit 0 = all |

Sentinel error:
- `var ErrNotFound = errors.New("statedb: not found")`

## Schema

```sql
CREATE TABLE IF NOT EXISTS state (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS runs (
    id         TEXT PRIMARY KEY,
    status     TEXT NOT NULL DEFAULT 'PENDING',
    task_count INTEGER NOT NULL DEFAULT 0,
    started_at TEXT NOT NULL,
    ended_at   TEXT NOT NULL DEFAULT ''
);
```

## SQLite Configuration

```sql
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
PRAGMA foreign_keys=ON;
```

## Design Decisions

### WAL Mode

Write-Ahead Logging allows concurrent reads during writes. Essential for a long-running agent that reads state while tasks execute.

### String Timestamps (RFC3339)

Consistent with the rest of the project (audit, manifest, progress all use string timestamps). Avoids SQLite datetime parsing complexity.

### Two Tables as Foundation

`state` provides a generic key-value store for system flags (active mode, health status, kill switch state). `runs` provides structured run tracking. Future phases add tables (dag_nodes, analytics, etc.) on top of this foundation.

### Sentinel ErrNotFound

Single error type for missing records. Callers check `errors.Is(err, statedb.ErrNotFound)`. Consistent with ErrProfileNotFound, ErrTaskNotFound patterns.

### Upsert for SetState

`INSERT OR REPLACE` semantics — idempotent, simple. No need for separate Create/Update for key-value state.

### Open Creates Tables

Tables are created in `Open()` with `CREATE TABLE IF NOT EXISTS`. No separate Init step. For future schema changes, integrate with `internal/migration`.

## CLI Commands

### `apex statedb status`
Shows DB path, file size, and row counts for each table.

### `apex statedb state list [--format json]`
Lists all key-value state entries.

### `apex statedb runs list [--limit 10] [--format json]`
Lists recent run records, most recent first.

## Unit Tests (7)

| Test | Description |
|------|-------------|
| `TestOpen` | Opens temp DB, verifies WAL mode via PRAGMA, Close works |
| `TestSetGetState` | Set then Get round-trip, verify all fields |
| `TestSetStateUpsert` | Set same key twice, Get returns latest value |
| `TestDeleteState` | Delete existing key, Get returns ErrNotFound |
| `TestListState` | Multiple entries returned sorted by key |
| `TestInsertGetRun` | Insert then Get round-trip, verify all fields |
| `TestUpdateRunStatusAndList` | Update status to COMPLETED, verify ended_at set; ListRuns returns correct order and limit |

## E2E Tests (3)

| Test | Description |
|------|-------------|
| `TestStateDBStatus` | CLI invocation → shows DB path and table info |
| `TestStateDBStateList` | CLI invocation → state list output |
| `TestStateDBRunsList` | CLI invocation → runs list output |

## Format Functions

| Function | Description |
|----------|-------------|
| `FormatStatus(path string, stateCount, runCount int) string` | DB status summary |
| `FormatStateList(entries []StateEntry) string` | Table: KEY / VALUE / UPDATED_AT |
| `FormatRunList(runs []RunRecord) string` | Table: ID / STATUS / TASKS / STARTED / ENDED |
| `FormatStateListJSON(entries []StateEntry) (string, error)` | JSON output |
| `FormatRunListJSON(runs []RunRecord) (string, error)` | JSON output |

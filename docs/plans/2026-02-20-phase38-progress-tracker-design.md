# Phase 38: Progress Tracker

> Design doc for Apex Agent CLI — structured task progress tracking and reporting.

## Problem

Long-running tasks (LONG_RUNNING / BATCH modes) provide no visibility into their progress. Users must wait until completion or failure with no intermediate feedback. This is especially painful for multi-step DAG executions that may take minutes.

## Solution

A `progress` package that tracks task progress through a Start → Update → Complete/Fail lifecycle. Each task gets a `ProgressReport` with percentage, phase, message, and timestamps. A `Tracker` manages all reports with thread-safe access.

## Architecture

```
internal/progress/
├── progress.go       # Status, ProgressReport, Tracker, lifecycle methods
└── progress_test.go  # 7 unit tests
```

## Key Types

### Status

```go
type Status string

const (
    StatusRunning   Status = "RUNNING"
    StatusCompleted Status = "COMPLETED"
    StatusFailed    Status = "FAILED"
)
```

### ProgressReport

```go
type ProgressReport struct {
    TaskID    string    `json:"task_id"`
    Phase     string    `json:"phase"`
    Percent   int       `json:"percent"`
    Message   string    `json:"message"`
    StartedAt time.Time `json:"started_at"`
    UpdatedAt time.Time `json:"updated_at"`
    Status    Status    `json:"status"`
}
```

### Tracker

```go
type Tracker struct {
    mu      sync.RWMutex
    reports map[string]*ProgressReport
}
```

## Core Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `NewTracker` | `() *Tracker` | Creates empty tracker |
| `(*Tracker) Start` | `(taskID, phase string) *ProgressReport` | Creates report: Status=RUNNING, Percent=0, sets StartedAt+UpdatedAt |
| `(*Tracker) Update` | `(taskID string, percent int, message string) error` | Updates percent (clamped 0-100) and message, sets UpdatedAt. Error if not found |
| `(*Tracker) Complete` | `(taskID, message string) error` | Sets Percent=100, Status=COMPLETED, updates message+UpdatedAt. Error if not found |
| `(*Tracker) Fail` | `(taskID, message string) error` | Sets Status=FAILED, keeps current percent, updates message+UpdatedAt. Error if not found |
| `(*Tracker) Get` | `(taskID string) (ProgressReport, error)` | Returns copy of report. ErrTaskNotFound if missing |
| `(*Tracker) List` | `() []ProgressReport` | Returns all reports sorted by UpdatedAt descending |

Sentinel error: `var ErrTaskNotFound = errors.New("progress: task not found")`

## Design Decisions

### Percent Clamping

`Update` clamps percent to 0-100 silently rather than returning an error. Callers shouldn't need to validate percentage values — the tracker is lenient on input.

### In-Memory Storage

Reports live only in memory. No file persistence (YAGNI). If persistence is needed later, a `Store` interface can be extracted.

### Thread Safety

`sync.RWMutex` on all Tracker methods, consistent with project patterns (Mode Selector, Connector, Event Runtime).

### Start Returns Report

`Start` returns the created `*ProgressReport` for immediate use. It does not error if the task already exists — it overwrites (restart semantics).

### List Sort Order

`List()` returns reports sorted by `UpdatedAt` descending (most recently updated first), providing a natural "what's happening now" view.

## CLI Commands

### `apex progress list [--format json]`
Lists all tracked tasks with their progress status.

### `apex progress show <task-id>`
Shows detailed progress for a specific task.

### `apex progress start <task-id> --phase <name>`
Manually starts tracking a task (for demo/testing purposes).

## Unit Tests (7)

| Test | Description |
|------|-------------|
| `TestNewTracker` | Creates empty tracker, List returns empty |
| `TestTrackerStart` | Start creates report with RUNNING status, Percent=0 |
| `TestTrackerUpdate` | Update changes percent and message; clamping at 0 and 100 |
| `TestTrackerUpdateNotFound` | Update on unknown task returns ErrTaskNotFound |
| `TestTrackerComplete` | Complete sets Percent=100, Status=COMPLETED |
| `TestTrackerFail` | Fail sets Status=FAILED, keeps current percent |
| `TestTrackerList` | Multiple tasks sorted by UpdatedAt descending |

## E2E Tests (3)

| Test | Description |
|------|-------------|
| `TestProgressList` | CLI invocation → empty list output |
| `TestProgressStart` | CLI start → task appears in output |
| `TestProgressShow` | CLI show on unknown task → error |

## Format Functions

| Function | Description |
|----------|-------------|
| `FormatProgressList(reports []ProgressReport) string` | Table: TASK_ID / PHASE / PERCENT / STATUS / UPDATED |
| `FormatProgressReport(report ProgressReport) string` | Detailed single-report display |
| `FormatProgressListJSON(reports []ProgressReport) (string, error)` | JSON output |

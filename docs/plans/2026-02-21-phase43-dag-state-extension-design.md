# Phase 43: DAG State Machine Extension

> Design doc for Apex Agent CLI — extend DAG from 4 states to 8 states with new transition methods.

## Problem

The DAG state machine has only 4 states (Pending/Running/Completed/Failed). There is no way to block nodes on dependencies explicitly, suspend execution, cancel individual nodes, or skip nodes due to upstream failures. The architecture requires a richer state machine.

## Solution

Extend the DAG package with 4 new states (Blocked/Suspended/Cancelled/Skipped) and corresponding transition methods. The extension is additive — all existing behavior and 14 tests continue to work unchanged. New states and methods are added in a separate file `states.go` within the same package.

## Architecture

```
internal/dag/
├── dag.go         # Existing: 4 states, core DAG operations (unchanged)
├── dag_test.go    # Existing: 14 tests (unchanged)
├── states.go      # NEW: 4 new states + transition methods
└── states_test.go # NEW: 7 unit tests for new states
```

## New States

| State | Value | Description |
|-------|-------|-------------|
| `Blocked` | 5 | Waiting on unresolved dependencies (explicit vs implicit in Pending) |
| `Suspended` | 6 | Paused by user/system intervention |
| `Cancelled` | 7 | Cancelled by user/system |
| `Skipped` | 8 | Skipped because dependency was cancelled/skipped |

## State Transitions

```
Pending ──→ Blocked     (MarkBlocked)
Pending ──→ Running     (existing MarkRunning)
Pending ──→ Suspended   (Suspend)
Pending ──→ Cancelled   (Cancel + cascade)
Blocked ──→ Pending     (Unblock)
Blocked ──→ Suspended   (Suspend)
Blocked ──→ Cancelled   (Cancel + cascade)
Running ──→ Completed   (existing MarkCompleted)
Running ──→ Failed      (existing MarkFailed + cascade)
Running ──→ Suspended   (Suspend)
Suspended → Pending     (Resume)
Cancelled → (terminal)
Skipped  → (terminal)
```

Terminal states: Completed, Failed, Cancelled, Skipped

## New Functions (in `states.go`)

| Function | Signature | Description |
|----------|-----------|-------------|
| `StatusString` | `(s Status) string` | Returns human-readable state name |
| `IsTerminal` | `(s Status) bool` | True for Completed/Failed/Cancelled/Skipped |
| `(*DAG) MarkBlocked` | `(id string) error` | Pending → Blocked |
| `(*DAG) Unblock` | `(id string) error` | Blocked → Pending |
| `(*DAG) Suspend` | `(id string) error` | Pending/Blocked/Running → Suspended |
| `(*DAG) Resume` | `(id string) error` | Suspended → Pending |
| `(*DAG) Cancel` | `(id string) error` | Any non-terminal → Cancelled, cascade skip dependents |
| `(*DAG) Skip` | `(id string) error` | Pending/Blocked → Skipped |
| `(*DAG) StatusCounts` | `() map[string]int` | Returns count per state name |

### Cancel Cascade

When a node is cancelled, all transitive dependents that are in Pending/Blocked state are marked as Skipped (not Cancelled — only explicitly cancelled nodes get Cancelled status).

### Error Handling

All transition methods return error for:
- Unknown node ID
- Invalid source state for the transition

Error format: `fmt.Errorf("dag: cannot %s node %s: current status is %s", action, id, StatusString(node.Status))`

## Unit Tests (7)

| Test | Description |
|------|-------------|
| `TestStatusString` | All 9 states return correct strings, unknown returns "Unknown" |
| `TestIsTerminal` | Completed/Failed/Cancelled/Skipped → true; others → false |
| `TestMarkBlockedUnblock` | Pending → Blocked → Pending round-trip |
| `TestSuspendResume` | Pending → Suspended → Pending; Running → Suspended; Blocked → Suspended |
| `TestCancel` | Non-terminal → Cancelled; error on terminal state |
| `TestCancelCascade` | Cancel parent → dependents become Skipped |
| `TestStatusCounts` | Correct counts for mixed-state DAG |

## E2E Tests

No new CLI commands for this phase — the DAG is used programmatically. E2E coverage comes from existing DAG-related commands.

## Design Decisions

### Additive Extension

New states use explicit values (5-8) rather than extending the iota. This ensures existing code (which uses 0-3) is completely unaffected. All 14 existing tests pass without modification.

### Separate File

`states.go` in the same package gives access to internal fields while keeping concerns separated. The existing `dag.go` is not modified at all.

### Cancel vs Skip Semantics

Cancel = explicit user/system action on the node itself. Skip = automatic consequence of a dependency being cancelled. This distinction makes audit trails clearer.

### StatusString as Free Function

Not a method on Status to keep the Status type as a simple int enum. Consistent with the project's pattern of using free functions (e.g., `LevelValue` in notify package).

# Phase 17: Causal Chain / Tracing — Design Document

> Date: 2026-02-20
> Status: Approved
> Architecture Ref: v11.0 §2.8 Reasoning Protocols, §2.9 Observability & Audit

## 1. Goal

Assign a `trace_id` to each user instruction. All DAG Actions/Events inherit this ID and record `parent_action_id`, forming a complete causal chain traceable to the root instruction. Satisfies Invariant I5: trace_id chain completeness.

## 2. Package: `internal/trace`

### 2.1 Core API

```go
// TraceContext carries tracing information through an execution.
type TraceContext struct {
    TraceID        string // root trace ID (assigned per user instruction)
    ParentActionID string // direct parent's action ID (empty for root)
}

// NewTrace generates a new root trace context with a fresh UUID trace_id.
func NewTrace() TraceContext

// Child creates a child trace context inheriting the trace_id
// with the given actionID as the new parent.
func (tc TraceContext) Child(actionID string) TraceContext
```

`NewTrace()` generates a UUID v4 for `TraceID` with empty `ParentActionID`.
`Child()` returns a new `TraceContext` with the same `TraceID` and the provided `actionID` as `ParentActionID`.

## 3. Audit Record Extension

### 3.1 Entry struct

Add two fields to `audit.Entry`:
```go
TraceID        string
ParentActionID string
```

### 3.2 Record struct

Add two fields to `audit.Record`:
```go
TraceID        string `json:"trace_id,omitempty"`
ParentActionID string `json:"parent_action_id,omitempty"`
```

### 3.3 Log() method

Map Entry fields to Record fields before hash computation:
```go
record.TraceID = entry.TraceID
record.ParentActionID = entry.ParentActionID
```

These fields are included in the hash chain (covered by computeHash).

### 3.4 Query by Trace ID

Add method to Logger:
```go
func (l *Logger) FindByTraceID(traceID string) ([]Record, error)
```

Scans all audit files, returns records matching the given trace_id, sorted by timestamp.

## 4. Manifest Extension

Add `TraceID` field to `manifest.Manifest`:
```go
TraceID string `json:"trace_id,omitempty"`
```

Add `ActionID` field to `manifest.NodeResult`:
```go
ActionID string `json:"action_id,omitempty"`
```

## 5. Run Command Integration

In `cmd/apex/run.go`:
1. After config load, create root trace: `tc := trace.NewTrace()`
2. Pass `tc.TraceID` to manifest
3. For each DAG node execution, generate an action_id (UUID) and pass `tc.TraceID` + `tc.ParentActionID` to audit logger
4. Create child trace for each node: `childTC := tc.Child(actionID)`

## 6. CLI: `apex trace <run-id>`

New command that:
1. Loads the manifest for the given run-id to get the trace_id
2. Calls `logger.FindByTraceID(traceID)` to get all related audit entries
3. Displays a tree view showing parent-child relationships:

```
Trace: abc123-def456 (run: 12345678)
├── [node-1] say hello          success  120ms
└── [node-2] list files         failure  200ms
```

If no run-id given, shows the most recent run's trace.

## 7. Files

| File | Purpose |
|------|---------|
| `internal/trace/trace.go` | TraceContext, NewTrace(), Child() |
| `internal/trace/trace_test.go` | Unit tests |
| `internal/audit/logger.go` | Entry/Record trace fields + FindByTraceID |
| `internal/audit/logger_test.go` | Trace-related tests |
| `internal/manifest/manifest.go` | TraceID + NodeResult.ActionID |
| `cmd/apex/run.go` | Integrate trace context |
| `cmd/apex/trace.go` | `apex trace` command |
| `cmd/apex/main.go` | Register traceCmd |
| `e2e/trace_test.go` | E2E tests |

## 8. Performance

- TraceContext creation: < 1ms (UUID generation only)
- FindByTraceID: linear scan of audit files (acceptable for current scale)

## 9. Out of Scope

- Causal Analyst reasoning protocol (future phase)
- Hypothesis Board / Evidence Scoring (future phase)
- Async runtime cross-process trace propagation (future phase)
- Trace visualization TUI / dashboard (future phase)
- Trace persistence to database (current: in-memory query of JSONL files)

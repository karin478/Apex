# Phase 21: Run Manifest Diffing — Design Document

> Date: 2026-02-20
> Status: Approved
> Architecture Ref: v11.0 §2.9 Observability & Audit / Run Manifest

## 1. Goal

Provide `apex run diff <id1> <id2>` to compare two run manifests side-by-side. Helps debug reproducibility issues by highlighting differences in model, effort, risk level, outcome, duration, and per-node results.

## 2. Extend: `internal/manifest`

### 2.1 Diff Types

```go
type FieldDiff struct {
    Field string `json:"field"`
    Left  string `json:"left"`
    Right string `json:"right"`
}

type NodeDiff struct {
    NodeID string      `json:"node_id"`
    Type   string      `json:"type"` // "changed", "left_only", "right_only"
    Fields []FieldDiff `json:"fields,omitempty"`
}

type DiffResult struct {
    LeftRunID  string      `json:"left_run_id"`
    RightRunID string      `json:"right_run_id"`
    Fields     []FieldDiff `json:"fields"`
    NodeDiffs  []NodeDiff  `json:"node_diffs"`
}
```

### 2.2 Diff Function

```go
func Diff(left, right *Manifest) *DiffResult
```

Compares top-level fields: Task, Model, Effort, RiskLevel, NodeCount, DurationMs, Outcome, TraceID.
Compares nodes by ID: detects changed, left_only, right_only nodes. For changed nodes, compares Status and Error fields.

### 2.3 Format Functions

```go
func FormatDiffHuman(d *DiffResult) string  // table format
func FormatDiffJSON(d *DiffResult) string   // JSON format
```

## 3. CLI: `apex run diff <id1> <id2>`

```
apex run diff abc123 def456              # human-readable table
apex run diff abc123 def456 --format json  # JSON output
```

Output example:
```
=== Run Diff: abc123 vs def456 ===

Field       | abc123          | def456
------------|-----------------|------------------
model       | claude-sonnet   | claude-opus
effort      | medium          | high
duration_ms | 1200            | 3400
outcome     | success         | failed

Node Differences:
  [changed] node-2: status success → failed
  [right_only] node-4: (not in abc123)
```

## 4. Non-Goals

- No DAG topology diffing (node-level only)
- No audit log content comparison
- No similarity scoring
- No three-way merge or common ancestor detection

# Phase 8 Human-in-the-Loop Approval Design

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scope | MVP interactive approval for HIGH risk | Unblock HIGH tasks without full TUI/file-based system |
| CRITICAL handling | Still rejected | Multi-person approval deferred to future phase |
| Approval granularity | Per-node | Users can skip risky nodes while approving safe ones |
| Testability | io.Reader/Writer injection | No real stdin dependency in tests |

## Current Behavior

| Risk Level | Behavior |
|------------|----------|
| LOW | Auto-approve |
| MEDIUM | Simple y/n prompt |
| HIGH | **Rejected** (too aggressive) |
| CRITICAL | Rejected |

## New Behavior

| Risk Level | Behavior |
|------------|----------|
| LOW | Auto-approve (unchanged) |
| MEDIUM | Simple y/n prompt (unchanged) |
| HIGH | **Interactive approval**: show DAG plan with per-node risk, user approves/skips/rejects |
| CRITICAL | Rejected (unchanged) |

## Architecture

### 1. Governance Changes (`internal/governance/risk.go`)

```go
// ShouldReject now only matches CRITICAL
func (r RiskLevel) ShouldReject() bool {
    return r >= CRITICAL
}

// New method for HIGH
func (r RiskLevel) ShouldRequireApproval() bool {
    return r == HIGH
}
```

### 2. Approval Package (`internal/approval`)

New package with interactive review flow:

```go
type Decision int

const (
    Approved Decision = iota
    Skipped
    Rejected
)

type NodeDecision struct {
    NodeID   string
    Decision Decision
    Reason   string
}

type Result struct {
    Approved bool           // Overall pass
    Nodes    []NodeDecision // Per-node decisions
}

type Reviewer struct {
    in  io.Reader
    out io.Writer
}

func NewReviewer(in io.Reader, out io.Writer) *Reviewer

// Review displays DAG plan and collects approval decisions.
// Each node is classified independently. Users can:
//   (a)pprove all — approve every node
//   (r)eview one-by-one — decide per node (approve/skip/reject)
//   (q)uit — reject all
func (r *Reviewer) Review(nodes []*dag.Node, classify func(string) RiskLevel) (*Result, error)
```

### 3. Interactive Flow

**Initial prompt:**
```
┌────────────────────────────────────────┐
│ Approval Required — 3 steps planned    │
├────────────────────────────────────────┤
│ [1] migrate database schema      HIGH  │
│ [2] update API endpoints       MEDIUM  │
│ [3] run integration tests         LOW  │
├────────────────────────────────────────┤
│ (a)pprove all / (r)eview / (q)uit     │
└────────────────────────────────────────┘
```

**Per-node review (if user picks 'r'):**
```
[1/3] migrate database schema (HIGH)
  (a)pprove / (s)kip / (r)eject all →
```

### 4. Integration into run.go

```go
// Replace current HIGH rejection block:
if risk.ShouldRequireApproval() {
    reviewer := approval.NewReviewer(os.Stdin, os.Stdout)
    result, err := reviewer.Review(d.NodeSlice(), governance.Classify)
    if err != nil { return err }
    if !result.Approved { return nil }
    // Remove skipped nodes from DAG
    for _, nd := range result.Nodes {
        if nd.Decision == approval.Skipped {
            d.RemoveNode(nd.NodeID)
        }
    }
}
```

### 5. DAG Changes (`internal/dag`)

Add helper methods:
- `NodeSlice() []*Node` — ordered slice for display
- `RemoveNode(id string)` — remove node and update dependencies

### 6. Audit Integration

Approval decisions logged as audit entries:
```go
logger.Log(audit.Entry{
    Task:      "approval",
    Outcome:   "approved",  // or "rejected", "partial"
    RiskLevel: risk.String(),
})
```

## Test Strategy

- `approval` package: inject `strings.Reader` as stdin, `bytes.Buffer` as stdout
- Test scenarios: approve all, review one-by-one, skip nodes, reject all, empty DAG
- `governance`: verify `ShouldRequireApproval()` and updated `ShouldReject()`
- `dag`: test `RemoveNode` with dependency updates

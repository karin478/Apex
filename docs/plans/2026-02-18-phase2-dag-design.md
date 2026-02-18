# Apex Agent — Phase 2 DAG + Agent Pool Design

**Date**: 2026-02-18
**Status**: Approved
**Author**: Lyndon + Claude Opus 4.6

## Overview

Phase 2 adds DAG-based task orchestration to Apex Agent. A user task is automatically decomposed by Claude into a DAG of subtasks, which are scheduled and executed in parallel where possible.

## Decisions Made

| Decision | Choice | Rationale |
|----------|--------|-----------|
| DAG complexity | Simplified (PENDING/RUNNING/COMPLETED/FAILED only) | Full state machine (SUSPENDED, RESUMING, etc.) deferred to Phase 6 |
| Task decomposition | LLM-driven (Claude analyzes and splits) | Core value proposition is automation |
| Simple tasks | Bypass planner, direct execution | Avoid wasting API call on trivial tasks |
| Approach | LLM auto-decompose DAG | Best user experience |
| TDD | Strict across all components | Required |

## Architecture

```
apex run "complex task"
  │
  ├── 1. Config + Governance (Phase 1)
  ├── 2. Planner: Claude analyzes → generates DAG JSON
  │     [{id:"analyze", task:"...", depends:[]},
  │      {id:"refactor", task:"...", depends:["analyze"]},
  │      {id:"test", task:"...", depends:["refactor"]}]
  ├── 3. Scheduler: topological sort → yield ready nodes
  │     PENDING → RUNNING → COMPLETED/FAILED
  ├── 4. Pool: concurrent workers execute nodes via Executor
  └── 5. Collector: aggregate results → Memory + Audit
```

## New Packages

```
internal/
├── planner/     # LLM task decomposition → DAG
├── dag/         # DAG data structure + scheduler
├── pool/        # Concurrent agent worker pool
└── (existing) config/ governance/ executor/ memory/ audit/
```

## Component Design

### Planner (`internal/planner/`)

```go
type Node struct {
    ID      string   `json:"id"`
    Task    string   `json:"task"`
    Depends []string `json:"depends"`
}

// Plan calls Claude to decompose a task into DAG nodes.
// Returns single-node list for simple tasks (no extra API call).
// For complex tasks, calls claude -p with a system prompt
// requesting JSON array output.
func Plan(ctx context.Context, exec *executor.Executor, task string) ([]Node, error)
```

- System prompt instructs Claude to return `[{"id":"...", "task":"...", "depends":[...]}]`
- Validates returned JSON
- Validates DAG (no cycles, all depends exist)
- Simple task heuristic: if task is < 50 words and no conjunctions (and/then/after), skip planner

### DAG (`internal/dag/`)

```go
type Status int
const (
    Pending Status = iota
    Running
    Completed
    Failed
)

type Node struct {
    ID       string
    Task     string
    Depends  []string
    Status   Status
    Result   string
    Error    string
}

type DAG struct {
    Nodes map[string]*Node
}

func New(nodes []planner.Node) (*DAG, error)  // Build DAG, validate
func (d *DAG) ReadyNodes() []*Node             // Nodes with all deps COMPLETED and self PENDING
func (d *DAG) MarkRunning(id string)
func (d *DAG) MarkCompleted(id string, result string)
func (d *DAG) MarkFailed(id string, err string) // Also cascade-fail dependents
func (d *DAG) IsComplete() bool                 // All nodes COMPLETED or FAILED
func (d *DAG) HasFailure() bool
func (d *DAG) Summary() string                  // Human-readable status
```

- Cascade failure: when a node fails, all downstream nodes are marked FAILED
- No cycles allowed (validated at construction)
- Thread-safe (mutex-protected for concurrent pool access)

### Pool (`internal/pool/`)

```go
type Pool struct {
    maxWorkers int
    executor   *executor.Executor
}

func New(maxWorkers int, exec *executor.Executor) *Pool
func (p *Pool) Execute(ctx context.Context, dag *dag.DAG) error
```

- Main loop: `ReadyNodes() → dispatch to workers → wait for completion → repeat`
- Uses `sync.WaitGroup` + channels for coordination
- Respects `max_concurrent` from config
- Each worker: `MarkRunning → Executor.Run → MarkCompleted/MarkFailed`

### Config Changes

```yaml
# ~/.apex/config.yaml additions
claude:
  timeout: 1800          # 30 min (was 600)
  long_task_timeout: 7200  # 2 hours (was 1800)
planner:
  model: claude-opus-4-6
  timeout: 120
pool:
  max_concurrent: 4
```

### New/Modified Commands

```bash
apex plan "task"    # NEW: decompose and preview DAG, don't execute
apex run "task"     # MODIFIED: decompose → schedule → execute → collect
```

## Data Flow

```
User task
  → Governance (risk check)
  → Planner (Claude decomposes → JSON nodes)
  → DAG (build + validate)
  → Pool (concurrent execution)
     → for each ready node:
        Executor (claude -p) → result
        → DAG.MarkCompleted/Failed
     → repeat until DAG.IsComplete()
  → Collector (summarize)
  → Memory (save session + decisions)
  → Audit (log each node + overall)
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Planner returns invalid JSON | Fallback to single-node execution |
| DAG has cycle | Error, refuse to execute |
| Node fails | Cascade-fail all dependents, continue independent branches |
| All nodes fail | Report failures, save to audit |
| Pool timeout | Cancel remaining nodes, report partial results |

## Out of Scope (Phase 2)

- SUSPENDED/RESUMING/REPLANNING states
- Snapshot/rollback per node
- Artifact Registry (inter-node artifact sharing)
- Cost Engine (token budget tracking)
- DAG persistence (resume across sessions)
- Dynamic DAG invalidation

# Phase 9 Dry-Run Mode Design

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Trigger | `apex run --dry-run` flag | Reuses existing run command, no new top-level command |
| Scope | Risk + DAG + context tokens + cost estimate | Show everything the user needs to decide before executing |
| Cost estimation | Simple model: tokens × price per 1K | Good enough for estimation, no API call needed |
| Approval preview | Show which nodes need approval, don't prompt | Dry-run is read-only |

## Architecture

### 1. `--dry-run` Flag on run Command

Add a `--dry-run` boolean flag to `runCmd`. When set, the command:
1. Classifies risk (same as normal)
2. Decomposes into DAG (same as normal)
3. Builds enriched contexts (same as normal)
4. Prints dry-run report instead of executing
5. Returns without snapshot/execution/audit/manifest

### 2. Cost Estimator (`internal/cost`)

Simple token-to-cost estimation:

```go
type Estimate struct {
    InputTokens  int
    OutputTokens int // estimated as fraction of input
    TotalCost    float64
    Model        string
    NodeCount    int
}

func EstimateRun(tasks map[string]string, model string) *Estimate
```

Model pricing (hardcoded for MVP, config-driven later):
- claude-sonnet: $3/1M input, $15/1M output
- claude-opus: $15/1M input, $75/1M output
- Default fallback: $3/1M input, $15/1M output

Output tokens estimated as 2x input tokens (typical for code generation).

### 3. Dry-Run Report Format

```
[DRY RUN] delete old migration files and deploy to staging

Risk: HIGH (approval required for execution)

Plan: 3 steps
  [1] delete old migration files              HIGH
  [2] run database migration                  HIGH
  [3] deploy to staging                       HIGH

Context:
  [1] 892 tokens
  [2] 1204 tokens
  [3] 756 tokens
  Budget: 2852/4096 (69%)

Cost estimate: ~$0.04 (3 calls, claude-sonnet-4-20250514)

No changes made. Run without --dry-run to execute.
```

### 4. Integration into run.go

After context enrichment, check `dryRun` flag:

```go
if dryRun {
    printDryRunReport(task, risk, d, enrichedTasks, cfg)
    return nil
}
```

This skips: runID generation, executor creation, kill switch watch, snapshot, execution, audit, manifest, memory save.

## Test Strategy

- `internal/cost`: unit tests for EstimateRun with different models
- `cmd/apex`: verify `--dry-run` flag exists and is parsed
- Integration: build binary, run `./bin/apex run --dry-run "test task"` (will fail at planner but verifies flag works)

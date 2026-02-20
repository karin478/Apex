# Phase 26: Aggregation Pipeline — Implementation Plan

**Date:** 2026-02-20
**Design:** `2026-02-20-phase26-aggregation-pipeline-design.md`
**Method:** Subagent-Driven Development

## Tasks

### Task 1: Aggregator Core (Pipeline + 3 Strategies)

**Files:** `internal/aggregator/pipeline.go`, `internal/aggregator/pipeline_test.go`
**Tests (7):**
1. `TestSummarize` — Concatenates inputs with node headers
2. `TestSummarizeEmpty` — Empty input returns error
3. `TestMerge` — Flattens multiple JSON arrays into one
4. `TestMergeDedup` — Dedup by key field (last wins)
5. `TestMergeSort` — Sort by sort field ascending
6. `TestReduce` — Correct Count/Sum/Min/Max/Avg computation
7. `TestReduceEmpty` — Empty input returns error

**Spec:**
- `Strategy` typed const: summarize, merge, reduce
- `Input` struct with NodeID, Content, Data
- `MergeOptions` struct with KeyField, SortField
- `ReduceStats` struct with Count, Sum, Min, Max, Avg
- `Result` struct with Strategy, Output, Data, InputCount, CreatedAt
- `Pipeline` struct with strategy, inputs slice, mergeOpt
- `NewPipeline(strategy)` creates pipeline
- `SetMergeOptions(opts)` configures merge behavior
- `Add(input)` appends input
- `Execute()` dispatches to strategy-specific function, returns Result
- Summarize: join Content with `--- [NodeID] ---` separator
- Merge: flatten Data arrays, dedup by KeyField if set, sort by SortField if set
- Reduce: extract float64 from each Data, compute stats
- Empty input → error "aggregator: no inputs"

### Task 2: Format Functions + CLI Command

**Files:** `internal/aggregator/format.go`, `internal/aggregator/format_test.go`, `cmd/apex/aggregate.go`, update `cmd/apex/main.go`
**Tests (2):**
1. `TestFormatResult` — Human-readable output for each strategy
2. `TestFormatResultJSON` — JSON indented output

**Spec:**
- `FormatResult(result *Result) string` — Strategy header + output text + input count
- `FormatResultJSON(result *Result) string` — json.MarshalIndent with 2-space indent
- CLI: `apex aggregate --strategy STR --file FILE [--key-field KEY] [--sort-field SORT] [--format json]`
- Reads input JSON array from file, creates pipeline, executes, formats
- Register `aggregateCmd` in rootCmd

### Task 3: E2E Tests

**Files:** `e2e/aggregate_test.go`
**Tests (3):**
1. `TestAggregateFromFile` — Summarize strategy with temp input file
2. `TestAggregateReduce` — Reduce strategy with numeric data
3. `TestAggregateInvalidStrategy` — Bad strategy name returns error

### Task 4: PROGRESS.md Update

Update PROGRESS.md with Phase 26 completion info.

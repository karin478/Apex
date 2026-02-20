# Phase 26: Aggregation Pipeline — Design Document

**Date:** 2026-02-20
**Status:** Approved (recommendation credit 4/10)

## Overview

Implement an Aggregation Pipeline (`internal/aggregator`) with three strategies for combining multi-agent outputs: Summarize (text compression), Structured Merge (JSON merge/dedup), and Statistical Reduce (numerical aggregation).

## Architecture

### Core Types

```go
type Strategy string

const (
    StrategySummarize Strategy = "summarize"
    StrategyMerge     Strategy = "merge"
    StrategyReduce    Strategy = "reduce"
)

type Input struct {
    NodeID  string      `json:"node_id"`
    Content string      `json:"content"`
    Data    interface{} `json:"data,omitempty"`
}

type MergeOptions struct {
    KeyField string `json:"key_field"`
    SortField string `json:"sort_field"`
}

type Result struct {
    Strategy   Strategy    `json:"strategy"`
    Output     string      `json:"output"`
    Data       interface{} `json:"data,omitempty"`
    InputCount int         `json:"input_count"`
    CreatedAt  time.Time   `json:"created_at"`
}

type ReduceStats struct {
    Count int     `json:"count"`
    Sum   float64 `json:"sum"`
    Min   float64 `json:"min"`
    Max   float64 `json:"max"`
    Avg   float64 `json:"avg"`
}
```

### Pipeline Operations

```go
type Pipeline struct {
    strategy Strategy
    inputs   []Input
    mergeOpt *MergeOptions
}

func NewPipeline(strategy Strategy) *Pipeline
func (p *Pipeline) SetMergeOptions(opts MergeOptions)
func (p *Pipeline) Add(input Input)
func (p *Pipeline) Execute() (*Result, error)
```

### Strategy Implementations

**Summarize:**
- Concatenates all Input.Content fields
- Separates with `--- [NodeID] ---` headers
- Output is the combined text with source attribution

**Structured Merge:**
- Each Input.Data must be a `[]interface{}` (array of objects)
- Flattens all arrays into one
- If KeyField set: dedup by that field (last wins)
- If SortField set: sort ascending by that field
- Result.Data contains merged array

**Statistical Reduce:**
- Each Input.Data must be a `float64` value
- Computes Count, Sum, Min, Max, Avg
- Result.Data contains ReduceStats
- Result.Output contains human-readable stats

### Format

- `FormatResult(result *Result) string` — Human-readable output
- `FormatResultJSON(result *Result) string` — JSON output

### CLI Command

```
apex aggregate --strategy <summarize|merge|reduce> --file <input.json> [--key-field KEY] [--sort-field SORT] [--format json]
```

Input file format:
```json
[
  {"node_id": "n1", "content": "text...", "data": ...},
  {"node_id": "n2", "content": "text...", "data": ...}
]
```

## Testing

| Test | Description |
|------|-------------|
| TestSummarize | Concatenates with headers |
| TestSummarizeEmpty | Empty input returns error |
| TestMerge | Flattens arrays |
| TestMergeDedup | Dedup by key field |
| TestMergeSort | Sort by sort field |
| TestReduce | Computes correct stats |
| TestReduceEmpty | Empty input returns error |
| TestFormatResult | Human-readable output |
| TestFormatResultJSON | JSON output |
| E2E: TestAggregateFromFile | End-to-end with file input |
| E2E: TestAggregateReduce | Reduce strategy E2E |
| E2E: TestAggregateInvalidStrategy | Error on bad strategy |

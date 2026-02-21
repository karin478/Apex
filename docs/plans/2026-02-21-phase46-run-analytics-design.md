# Phase 46: Run History Analytics

> Design doc for Apex Agent CLI — aggregate run data with success rates, duration stats, and failure pattern analysis.

## Problem

The system tracks individual run records (Phase 42 statedb) but provides no aggregate view. Users cannot see success rates over time, average durations, or common failure patterns. Without analytics, there is no data-driven feedback loop for improving agent reliability.

## Solution

An `analytics` package that reads run records from statedb, computes aggregate statistics (success rate, duration percentiles, failure patterns), and presents summary reports.

## Architecture

```
internal/analytics/
├── analytics.go       # RunSummary, Stats, Analyze functions
└── analytics_test.go  # 7 unit tests
```

## Key Types

### RunSummary

```go
type RunSummary struct {
    TotalRuns   int            `json:"total_runs"`
    ByStatus    map[string]int `json:"by_status"`    // COMPLETED: n, FAILED: n, etc.
    SuccessRate float64        `json:"success_rate"`  // 0.0-1.0
    FailureRate float64        `json:"failure_rate"`  // 0.0-1.0
}
```

### DurationStats

```go
type DurationStats struct {
    Count   int     `json:"count"`    // number of completed runs with valid duration
    MinSecs float64 `json:"min_secs"`
    MaxSecs float64 `json:"max_secs"`
    AvgSecs float64 `json:"avg_secs"`
    P50Secs float64 `json:"p50_secs"` // median
    P90Secs float64 `json:"p90_secs"`
}
```

### FailurePattern

```go
type FailurePattern struct {
    Status string `json:"status"` // FAILED
    Count  int    `json:"count"`
    Rate   float64 `json:"rate"` // fraction of total runs
}
```

### Report

```go
type Report struct {
    Summary   RunSummary       `json:"summary"`
    Duration  DurationStats    `json:"duration"`
    Failures  []FailurePattern `json:"failures"`
    Generated string           `json:"generated"` // RFC3339
}
```

## Core Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `Summarize` | `(runs []statedb.RunRecord) RunSummary` | Count by status, compute success/failure rates |
| `ComputeDuration` | `(runs []statedb.RunRecord) DurationStats` | Compute duration stats for completed runs with valid start/end times |
| `DetectFailures` | `(runs []statedb.RunRecord) []FailurePattern` | Group failures by status, compute rates |
| `GenerateReport` | `(runs []statedb.RunRecord) Report` | Combines Summarize + ComputeDuration + DetectFailures |

### Duration Calculation

For each run where Status=COMPLETED and EndedAt is non-empty:
1. Parse StartedAt and EndedAt as RFC3339
2. Duration = EndedAt - StartedAt in seconds
3. Skip runs with parse errors or zero/negative duration

Percentile calculation: sort durations, P50 = median, P90 = 90th percentile index.

### Success/Failure Rates

- SuccessRate = count(COMPLETED) / TotalRuns
- FailureRate = count(FAILED) / TotalRuns
- If TotalRuns == 0, both rates are 0.0

## Design Decisions

### Operates on []RunRecord

The analytics functions take a slice of RunRecord rather than directly accessing the database. This keeps the package decoupled from statedb and makes testing trivial (pass in test data).

### Float64 for Rates and Durations

Standard for statistical computations. Consistent with `internal/cost` package.

### No Database Dependency

The analytics package imports `statedb` only for the `RunRecord` type. It does not open databases or perform queries. The CLI command handles the DB interaction.

### Percentiles via Sort

Simple sort-based percentile calculation. P50 = element at index len/2, P90 = element at index 9*len/10. No external statistics library needed.

## CLI Commands

### `apex analytics report [--limit 100] [--format json]`
Generates a full analytics report from recent runs.

### `apex analytics summary [--limit 100]`
Shows just the run summary (counts and rates).

## Unit Tests (7)

| Test | Description |
|------|-------------|
| `TestSummarizeEmpty` | Empty slice → TotalRuns=0, rates=0.0 |
| `TestSummarize` | Mixed runs → correct counts and rates |
| `TestComputeDurationEmpty` | Empty/no-completed → Count=0, all zeros |
| `TestComputeDuration` | Multiple completed runs → correct min/max/avg/p50/p90 |
| `TestDetectFailures` | FAILED runs → correct count and rate |
| `TestGenerateReport` | Full report with all sections populated |
| `TestComputeDurationSkipsInvalid` | Runs with empty EndedAt or parse errors are skipped |

## E2E Tests (2)

| Test | Description |
|------|-------------|
| `TestAnalyticsReport` | CLI shows report (on empty DB, shows TotalRuns=0) |
| `TestAnalyticsSummary` | CLI shows summary |

## Format Functions

| Function | Description |
|----------|-------------|
| `FormatReport(report Report) string` | Full report display |
| `FormatSummary(summary RunSummary) string` | Summary display with rates |
| `FormatReportJSON(report Report) (string, error)` | JSON output |

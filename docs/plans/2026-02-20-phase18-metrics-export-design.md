# Phase 18: Metrics Export — Design Document

> Date: 2026-02-20
> Status: Approved
> Architecture Ref: v11.0 §2.9 Observability & Audit (Metrics Export)

## 1. Goal

Provide a unified metrics collection and export system that aggregates data from runs, DAG execution, cost estimation, health checks, and audit logs. Supports human-readable and JSONL output formats via `apex metrics` CLI command.

## 2. Package: `internal/metrics`

### 2.1 Core Types

```go
// Metric represents a single metric data point.
type Metric struct {
    Name      string            `json:"name"`
    Value     float64           `json:"value"`
    Labels    map[string]string `json:"labels,omitempty"`
    Timestamp string            `json:"timestamp"`
}

// Collector gathers metrics from various apex subsystems.
type Collector struct {
    baseDir string
}

func NewCollector(baseDir string) *Collector
func (c *Collector) Collect() ([]Metric, error)
```

### 2.2 Metric Categories

| Category | Metric Name | Source |
|----------|-------------|--------|
| Runs | `apex_runs_total` | manifest.Store.Recent() |
| Runs | `apex_runs_by_outcome{outcome=X}` | manifest outcomes |
| Runs | `apex_run_duration_ms_avg` | manifest durations |
| DAG | `apex_dag_nodes_total` | manifest node counts |
| DAG | `apex_dag_nodes_failed` | manifest failed nodes |
| Cost | `apex_cost_usd_estimated` | cost.EstimateRun over manifests |
| Health | `apex_health_level` | health.Evaluate() (0-3) |
| Audit | `apex_audit_entries_total` | audit log file scanning |
| Audit | `apex_audit_chain_valid` | audit.Logger.Verify() (1/0) |

### 2.3 Export Formats

- **Human** (default): table layout for terminal display
- **JSONL**: one metric per line for machine consumption

## 3. CLI: `apex metrics`

```
apex metrics                    # human-readable table
apex metrics --format jsonl     # machine-readable JSONL
apex metrics --since 24h        # time filter (default: all time)
```

Flags:
- `--format` string: `human` (default) or `jsonl`
- `--since` string: duration filter, e.g., `24h`, `7d` (default: all)

## 4. Data Flow

```
manifest.Store  ──┐
health.Evaluate ──┼──► Collector.Collect() ──► []Metric ──► Format ──► stdout
audit.Logger    ──┘
```

## 5. Non-Goals

- No Prometheus HTTP endpoint (future enhancement)
- No daemon / continuous collection
- No push-based export
- No time-series storage
- No alerting

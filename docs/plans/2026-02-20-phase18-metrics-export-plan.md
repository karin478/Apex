# Phase 18: Metrics Export — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Collect and export metrics from runs, DAG, health, and audit subsystems via `apex metrics` CLI command.

**Architecture:** New `internal/metrics` package with a `Collector` that reads from `manifest.Store`, `health.Evaluate`, and `audit.Logger`. CLI command formats output as human-readable table or JSONL.

**Tech Stack:** Go, Cobra CLI, Testify, encoding/json

---

## Task 1: Metrics Core — Metric Type + Collector

**Files:**
- Create: `internal/metrics/metrics.go`
- Create: `internal/metrics/metrics_test.go`

**Step 1: Write the test file**

```go
package metrics

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCollector(t *testing.T) {
	c := NewCollector(t.TempDir())
	assert.NotNil(t, c)
}

func TestCollectEmpty(t *testing.T) {
	dir := t.TempDir()
	// Create required subdirectories
	os.MkdirAll(filepath.Join(dir, "runs"), 0755)
	os.MkdirAll(filepath.Join(dir, "audit"), 0755)

	c := NewCollector(dir)
	metrics, err := c.Collect()
	require.NoError(t, err)

	// Should still produce health metric even with no runs
	found := false
	for _, m := range metrics {
		if m.Name == "apex_health_level" {
			found = true
		}
	}
	assert.True(t, found, "should always include health level metric")
}

func TestCollectWithRuns(t *testing.T) {
	dir := t.TempDir()
	runsDir := filepath.Join(dir, "runs")
	os.MkdirAll(filepath.Join(dir, "audit"), 0755)

	// Write a manifest
	runDir := filepath.Join(runsDir, "test-run-001")
	os.MkdirAll(runDir, 0755)
	manifest := `{
		"run_id": "test-run-001",
		"task": "say hello",
		"timestamp": "` + time.Now().UTC().Format(time.RFC3339) + `",
		"model": "mock-model",
		"effort": "low",
		"risk_level": "LOW",
		"node_count": 2,
		"duration_ms": 1500,
		"outcome": "success",
		"nodes": [
			{"id": "n1", "task": "step 1", "status": "completed"},
			{"id": "n2", "task": "step 2", "status": "completed"}
		]
	}`
	os.WriteFile(filepath.Join(runDir, "manifest.json"), []byte(manifest), 0644)

	c := NewCollector(dir)
	metrics, err := c.Collect()
	require.NoError(t, err)

	// Check runs_total
	var runsTotal *Metric
	for i, m := range metrics {
		if m.Name == "apex_runs_total" {
			runsTotal = &metrics[i]
		}
	}
	require.NotNil(t, runsTotal, "should have apex_runs_total metric")
	assert.Equal(t, float64(1), runsTotal.Value)

	// Check dag_nodes_total
	var nodesTotal *Metric
	for i, m := range metrics {
		if m.Name == "apex_dag_nodes_total" {
			nodesTotal = &metrics[i]
		}
	}
	require.NotNil(t, nodesTotal, "should have apex_dag_nodes_total metric")
	assert.Equal(t, float64(2), nodesTotal.Value)
}

func TestMetricJSON(t *testing.T) {
	m := Metric{
		Name:      "apex_runs_total",
		Value:     5,
		Labels:    map[string]string{"outcome": "success"},
		Timestamp: "2026-02-20T00:00:00Z",
	}
	assert.Equal(t, "apex_runs_total", m.Name)
	assert.Equal(t, float64(5), m.Value)
	assert.Equal(t, "success", m.Labels["outcome"])
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/metrics/...`
Expected: FAIL — package does not exist

**Step 3: Write implementation**

```go
package metrics

import (
	"path/filepath"
	"time"

	"github.com/lyndonlyu/apex/internal/audit"
	"github.com/lyndonlyu/apex/internal/health"
	"github.com/lyndonlyu/apex/internal/manifest"
)

// Metric represents a single metric data point.
type Metric struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
	Timestamp string            `json:"timestamp"`
}

// Collector gathers metrics from apex subsystems.
type Collector struct {
	baseDir string
}

// NewCollector creates a Collector rooted at baseDir (~/.apex).
func NewCollector(baseDir string) *Collector {
	return &Collector{baseDir: baseDir}
}

// Collect gathers all metrics from runs, health, and audit.
func (c *Collector) Collect() ([]Metric, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var metrics []Metric

	// Run metrics from manifest store
	runsDir := filepath.Join(c.baseDir, "runs")
	store := manifest.NewStore(runsDir)
	manifests, err := store.Recent(10000) // all
	if err == nil {
		metrics = append(metrics, c.runMetrics(manifests, now)...)
	}

	// Health metric
	report := health.Evaluate(c.baseDir)
	metrics = append(metrics, Metric{
		Name:      "apex_health_level",
		Value:     float64(report.Level),
		Labels:    map[string]string{"level": report.Level.String()},
		Timestamp: now,
	})

	// Audit metrics
	auditDir := filepath.Join(c.baseDir, "audit")
	logger, logErr := audit.NewLogger(auditDir)
	if logErr == nil {
		metrics = append(metrics, c.auditMetrics(logger, now)...)
	}

	return metrics, nil
}

func (c *Collector) runMetrics(manifests []*manifest.Manifest, now string) []Metric {
	var metrics []Metric

	metrics = append(metrics, Metric{
		Name: "apex_runs_total", Value: float64(len(manifests)), Timestamp: now,
	})

	// By outcome
	outcomes := map[string]int{}
	totalNodes := 0
	failedNodes := 0
	var totalDuration int64

	for _, m := range manifests {
		outcomes[m.Outcome]++
		totalNodes += m.NodeCount
		totalDuration += m.DurationMs
		for _, n := range m.Nodes {
			if n.Status == "failed" {
				failedNodes++
			}
		}
	}

	for outcome, count := range outcomes {
		metrics = append(metrics, Metric{
			Name: "apex_runs_by_outcome", Value: float64(count),
			Labels: map[string]string{"outcome": outcome}, Timestamp: now,
		})
	}

	if len(manifests) > 0 {
		metrics = append(metrics, Metric{
			Name: "apex_run_duration_ms_avg",
			Value: float64(totalDuration) / float64(len(manifests)),
			Timestamp: now,
		})
	}

	metrics = append(metrics, Metric{
		Name: "apex_dag_nodes_total", Value: float64(totalNodes), Timestamp: now,
	})
	metrics = append(metrics, Metric{
		Name: "apex_dag_nodes_failed", Value: float64(failedNodes), Timestamp: now,
	})

	return metrics
}

func (c *Collector) auditMetrics(logger *audit.Logger, now string) []Metric {
	var metrics []Metric

	records, err := logger.Recent(100000) // all
	if err == nil {
		metrics = append(metrics, Metric{
			Name: "apex_audit_entries_total", Value: float64(len(records)), Timestamp: now,
		})
	}

	valid, _, verifyErr := logger.Verify()
	chainVal := float64(0)
	if verifyErr == nil && valid {
		chainVal = 1
	}
	metrics = append(metrics, Metric{
		Name: "apex_audit_chain_valid", Value: chainVal, Timestamp: now,
	})

	return metrics
}
```

**Step 4: Run tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/metrics/...`
Expected: PASS (4 tests)

**Step 5: Commit**

```bash
git add internal/metrics/metrics.go internal/metrics/metrics_test.go
git commit -m "feat(metrics): add Collector with run, health, and audit metrics"
```

---

## Task 2: Format Functions — Human + JSONL

**Files:**
- Create: `internal/metrics/format.go`
- Modify: `internal/metrics/metrics_test.go` (add format tests)

**Step 1: Add format tests**

Append to `metrics_test.go`:

```go
func TestFormatHuman(t *testing.T) {
	metrics := []Metric{
		{Name: "apex_runs_total", Value: 5, Timestamp: "2026-02-20T00:00:00Z"},
		{Name: "apex_health_level", Value: 0, Labels: map[string]string{"level": "GREEN"}, Timestamp: "2026-02-20T00:00:00Z"},
	}
	output := FormatHuman(metrics)
	assert.Contains(t, output, "apex_runs_total")
	assert.Contains(t, output, "5")
	assert.Contains(t, output, "GREEN")
}

func TestFormatJSONL(t *testing.T) {
	metrics := []Metric{
		{Name: "apex_runs_total", Value: 3, Timestamp: "2026-02-20T00:00:00Z"},
	}
	output, err := FormatJSONL(metrics)
	require.NoError(t, err)
	assert.Contains(t, output, `"name":"apex_runs_total"`)
	assert.Contains(t, output, `"value":3`)
}
```

**Step 2: Run tests — expect FAIL**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/metrics/...`
Expected: FAIL — FormatHuman/FormatJSONL not defined

**Step 3: Write format.go**

```go
package metrics

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatHuman returns a human-readable table of metrics.
func FormatHuman(metrics []Metric) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-35s %12s  %s\n", "METRIC", "VALUE", "LABELS"))
	b.WriteString(strings.Repeat("-", 65) + "\n")

	for _, m := range metrics {
		labels := ""
		if len(m.Labels) > 0 {
			var parts []string
			for k, v := range m.Labels {
				parts = append(parts, fmt.Sprintf("%s=%s", k, v))
			}
			labels = strings.Join(parts, ", ")
		}

		// Format value: integer if whole number, else 2 decimal places
		valStr := fmt.Sprintf("%.0f", m.Value)
		if m.Value != float64(int64(m.Value)) {
			valStr = fmt.Sprintf("%.2f", m.Value)
		}

		b.WriteString(fmt.Sprintf("%-35s %12s  %s\n", m.Name, valStr, labels))
	}
	return b.String()
}

// FormatJSONL returns one JSON object per line.
func FormatJSONL(metrics []Metric) (string, error) {
	var b strings.Builder
	for _, m := range metrics {
		data, err := json.Marshal(m)
		if err != nil {
			return "", err
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	return b.String(), nil
}
```

**Step 4: Run tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/metrics/...`
Expected: PASS (6 tests)

**Step 5: Commit**

```bash
git add internal/metrics/format.go internal/metrics/metrics_test.go
git commit -m "feat(metrics): add human-readable and JSONL format functions"
```

---

## Task 3: CLI Command — `apex metrics`

**Files:**
- Create: `cmd/apex/metrics.go`
- Modify: `cmd/apex/main.go` (register command)

**Step 1: Write metrics.go**

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/metrics"
	"github.com/spf13/cobra"
)

var metricsFormat string

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Show system metrics",
	Long:  "Collect and display metrics from runs, DAG execution, health, and audit subsystems.",
	RunE:  showMetrics,
}

func init() {
	metricsCmd.Flags().StringVar(&metricsFormat, "format", "human", "Output format: human or jsonl")
}

func showMetrics(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, ".apex")

	collector := metrics.NewCollector(baseDir)
	collected, err := collector.Collect()
	if err != nil {
		return fmt.Errorf("metrics collection failed: %w", err)
	}

	switch metricsFormat {
	case "jsonl":
		output, fmtErr := metrics.FormatJSONL(collected)
		if fmtErr != nil {
			return fmt.Errorf("format error: %w", fmtErr)
		}
		fmt.Print(output)
	default:
		fmt.Print(metrics.FormatHuman(collected))
	}
	return nil
}
```

**Step 2: Register in main.go**

Add `rootCmd.AddCommand(metricsCmd)` after the `traceCmd` line in `cmd/apex/main.go`.

**Step 3: Build to verify compilation**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build ./cmd/apex/...`
Expected: Build succeeds (sqlite warnings OK)

**Step 4: Commit**

```bash
git add cmd/apex/metrics.go cmd/apex/main.go
git commit -m "feat(cli): add apex metrics command with human and JSONL output"
```

---

## Task 4: E2E Tests

**Files:**
- Create: `e2e/metrics_test.go`

**Step 1: Write E2E tests**

```go
package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetricsEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, code := env.runApex("metrics")
	assert.Equal(t, 0, code, "apex metrics should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "apex_health_level", "should show health level metric")
	assert.Contains(t, stdout, "METRIC", "should show table header")
}

func TestMetricsAfterRun(t *testing.T) {
	env := newTestEnv(t)

	// Run a task first
	env.runApex("run", "say hello")

	stdout, stderr, code := env.runApex("metrics")
	assert.Equal(t, 0, code, "apex metrics should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "apex_runs_total", "should show runs total")
	assert.Contains(t, stdout, "apex_dag_nodes_total", "should show dag nodes")
}

func TestMetricsJSONL(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, code := env.runApex("metrics", "--format", "jsonl")
	assert.Equal(t, 0, code, "apex metrics --format jsonl should exit 0; stderr=%s", stderr)

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.Greater(t, len(lines), 0, "should output at least one JSONL line")
	assert.True(t, strings.HasPrefix(lines[0], "{"), "JSONL line should start with {")
}
```

**Step 2: Run E2E tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && make e2e`
Expected: All tests PASS including 3 new metrics tests

**Step 3: Commit**

```bash
git add e2e/metrics_test.go
git commit -m "test(e2e): add metrics command E2E tests"
```

---

## Task 5: Update PROGRESS.md

**Files:**
- Modify: `PROGRESS.md`

**Changes:**
1. Add Phase 18 row to Completed Phases table
2. Update "Current" section to Phase 19
3. Update E2E test count
4. Add `internal/metrics` to Key Packages table

**Commit:**

```bash
git add PROGRESS.md
git commit -m "docs: mark Phase 18 Metrics Export as complete"
```

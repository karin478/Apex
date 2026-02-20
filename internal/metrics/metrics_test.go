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
	os.MkdirAll(filepath.Join(dir, "runs"), 0755)
	os.MkdirAll(filepath.Join(dir, "audit"), 0755)

	c := NewCollector(dir)
	metrics, err := c.Collect()
	require.NoError(t, err)

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

	var runsTotal *Metric
	for i, m := range metrics {
		if m.Name == "apex_runs_total" {
			runsTotal = &metrics[i]
		}
	}
	require.NotNil(t, runsTotal, "should have apex_runs_total metric")
	assert.Equal(t, float64(1), runsTotal.Value)

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

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

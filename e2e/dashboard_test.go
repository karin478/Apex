package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// apex dashboard
// ---------------------------------------------------------------------------

func TestDashboardEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, code := env.runApex("dashboard")
	require.Equal(t, 0, code, "apex dashboard should exit 0; stderr: %s", stderr)

	assert.Contains(t, stdout, "APEX SYSTEM DASHBOARD", "dashboard output should contain title")
	assert.Contains(t, stdout, "System Health", "dashboard output should contain System Health section")
	assert.Contains(t, stdout, "No runs recorded", "dashboard output should indicate no runs on fresh env")
}

func TestDashboardAfterRun(t *testing.T) {
	env := newTestEnv(t)

	// Run a task first so the dashboard has data to display
	_, _, code := env.runApex("run", "say hello")
	require.Equal(t, 0, code, "apex run should exit 0")

	stdout, stderr, code := env.runApex("dashboard")
	require.Equal(t, 0, code, "apex dashboard should exit 0; stderr: %s", stderr)

	assert.Contains(t, stdout, "APEX SYSTEM DASHBOARD", "dashboard output should contain title")
	assert.Contains(t, stdout, "Recent Runs", "dashboard output should contain Recent Runs section")
	assert.Contains(t, stdout, "say hello", "dashboard output should contain the task description")
}

func TestDashboardMarkdown(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, code := env.runApex("dashboard", "--format", "md")
	require.Equal(t, 0, code, "apex dashboard --format md should exit 0; stderr: %s", stderr)

	assert.Contains(t, stdout, "# Apex System Dashboard", "markdown output should contain H1 title")
	assert.Contains(t, stdout, "## System Health", "markdown output should contain H2 System Health section")
}

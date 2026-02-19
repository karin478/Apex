package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// apex status
// ---------------------------------------------------------------------------

func TestStatusAfterRun(t *testing.T) {
	env := newTestEnv(t)

	// Run a task first so status has something to show
	_, _, code := env.runApex("run", "say hello")
	require.Equal(t, 0, code, "apex run should exit 0")

	// Now check status
	stdout, stderr, code := env.runApex("status")
	require.Equal(t, 0, code, "apex status should exit 0; stderr: %s", stderr)

	assert.Contains(t, stdout, "say hello", "status output should contain the task description")
	assert.Contains(t, stdout, "success", "status output should show 'success' outcome")
}

func TestStatusEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, code := env.runApex("status")
	require.Equal(t, 0, code, "apex status should exit 0; stderr: %s", stderr)

	assert.Contains(t, stdout, "No runs found", "status output should indicate no runs")
}

// ---------------------------------------------------------------------------
// apex history
// ---------------------------------------------------------------------------

func TestHistoryAfterRun(t *testing.T) {
	env := newTestEnv(t)

	// Run a task first so history has something to show
	_, _, code := env.runApex("run", "say hello")
	require.Equal(t, 0, code, "apex run should exit 0")

	// Now check history
	stdout, stderr, code := env.runApex("history")
	require.Equal(t, 0, code, "apex history should exit 0; stderr: %s", stderr)

	assert.True(t, strings.Contains(stdout, "[OK]"),
		"history output should contain [OK]; got: %s", stdout)
}

func TestHistoryEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, code := env.runApex("history")
	require.Equal(t, 0, code, "apex history should exit 0; stderr: %s", stderr)

	assert.Contains(t, stdout, "No history yet", "history output should indicate no history")
}

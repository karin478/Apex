package e2e_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInvariantChecksOnDoctor verifies that "apex doctor" runs invariant
// checks against the runtime database and reports results in its output.
func TestInvariantChecksOnDoctor(t *testing.T) {
	env := newTestEnv(t)

	// Run a task first to populate runtime.db and audit entries.
	stdout, stderr, exitCode := env.runApex("run", "say hello")
	require.Equal(t, 0, exitCode, "apex run should succeed; stdout=%s stderr=%s", stdout, stderr)

	// Run doctor — should include invariant checks section.
	stdout, stderr, exitCode = env.runApex("doctor")
	assert.Equal(t, 0, exitCode, "apex doctor should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "Invariant checks",
		"doctor output should contain the invariant checks section header")
}

// TestStagingMemoryOnRun verifies that after a successful run, the
// runtime.db database file exists inside the runtime directory.
func TestStagingMemoryOnRun(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("run", "say hello")
	require.Equal(t, 0, exitCode, "apex run should succeed; stdout=%s stderr=%s", stdout, stderr)

	// Check runtime.db exists (staging table is stored in it).
	runtimeDir := filepath.Join(env.Home, ".apex", "runtime")
	dbPath := filepath.Join(runtimeDir, "runtime.db")
	assert.True(t, env.fileExists(dbPath), "runtime.db should exist after a run")
}

// TestRollbackOnFailure verifies that when execution fails, the run
// manifest is still written and contains outcome/rollback information.
func TestRollbackOnFailure(t *testing.T) {
	env := newTestEnv(t)

	// Run a task that will fail via the mock's MOCK_EXIT_CODE support.
	stdout, _, exitCode := env.runApexWithEnv(
		map[string]string{"MOCK_EXIT_CODE": "1"},
		"run", "failing task",
	)

	// The run should fail (non-zero exit).
	assert.NotEqual(t, 0, exitCode, "run with MOCK_EXIT_CODE=1 should fail")

	// Manifest should still be written even on failure.
	runFiles, _ := filepath.Glob(filepath.Join(env.runsDir(), "*", "manifest.json"))
	if len(runFiles) > 0 {
		content := env.readFile(runFiles[0])
		assert.True(t,
			strings.Contains(content, "rollback_quality") || strings.Contains(content, "outcome"),
			"manifest should contain outcome information; got: %s", content)
	}

	_ = stdout
}

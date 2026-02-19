package e2e_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunHappyPath(t *testing.T) {
	env := newTestEnv(t)

	stdout, _, exitCode := env.runApex("run", "say hello")

	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "Done")
	assert.Contains(t, stdout, "Risk level")

	// Verify audit file was created
	auditFiles, _ := filepath.Glob(filepath.Join(env.auditDir(), "*.jsonl"))
	assert.NotEmpty(t, auditFiles, "audit file should be created")

	// Verify manifest file was created (stored as {runsDir}/{runID}/manifest.json)
	runFiles, _ := filepath.Glob(filepath.Join(env.runsDir(), "*", "manifest.json"))
	assert.NotEmpty(t, runFiles, "manifest file should be created")
}

func TestRunDryRun(t *testing.T) {
	env := newTestEnv(t)

	stdout, _, exitCode := env.runApex("run", "--dry-run", "say hello")

	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "DRY RUN")
	assert.Contains(t, stdout, "No changes made")

	// Verify NO manifest created in dry-run mode
	runFiles, _ := filepath.Glob(filepath.Join(env.runsDir(), "*", "manifest.json"))
	assert.Empty(t, runFiles, "dry run should not create manifest")
}

func TestRunMultiNodeDAG(t *testing.T) {
	env := newTestEnv(t)

	multiDAG := `[{"id":"step1","task":"first step","depends":[]},{"id":"step2","task":"second step","depends":["step1"]},{"id":"step3","task":"third step","depends":["step1"]}]`

	stdout, _, exitCode := env.runApexWithEnv(
		map[string]string{"MOCK_PLANNER_RESPONSE": multiDAG},
		"run", "first do X then do Y and Z",
	)

	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "3 steps")
	assert.Contains(t, stdout, "Done")
}

func TestRunExecutionFailure(t *testing.T) {
	env := newTestEnv(t)

	_, _, exitCode := env.runApexWithEnv(
		map[string]string{
			"MOCK_EXIT_CODE": "2",
			"MOCK_STDERR":    "permission denied",
		},
		"run", "say hello",
	)

	// Exit code should be non-zero (execution error propagates)
	assert.NotEqual(t, 0, exitCode)
}

func TestRunNoArgs(t *testing.T) {
	env := newTestEnv(t)

	_, stderr, exitCode := env.runApex("run")

	assert.NotEqual(t, 0, exitCode)
	assert.Contains(t, stderr, "requires at least 1 arg")
}

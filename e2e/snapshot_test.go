package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotListEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, _, exitCode := env.runApex("snapshot", "list")

	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "No snapshots")
}

func TestSnapshotCreatedOnRun(t *testing.T) {
	env := newTestEnv(t)

	// Create a local change so git stash has something to snapshot
	err := os.WriteFile(filepath.Join(env.WorkDir, "hello.txt"), []byte("world"), 0644)
	require.NoError(t, err)

	stdout, _, exitCode := env.runApex("run", "say hello")

	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "Snapshot saved")
}

func TestSnapshotPersistsOnFailure(t *testing.T) {
	env := newTestEnv(t)

	// Create a local change so git stash has something to snapshot
	err := os.WriteFile(filepath.Join(env.WorkDir, "hello.txt"), []byte("world"), 0644)
	require.NoError(t, err)

	// Run with a forced failure
	_, _, exitCode := env.runApexWithEnv(
		map[string]string{
			"MOCK_EXIT_CODE": "2",
			"MOCK_STDERR":    "fatal error",
		},
		"run", "say hello",
	)

	assert.NotEqual(t, 0, exitCode)

	// Verify the snapshot persists (was not dropped)
	stdout, _, listExitCode := env.runApex("snapshot", "list")
	require.Equal(t, 0, listExitCode)
	assert.NotContains(t, stdout, "No snapshots", "snapshot should persist after failed run")
}

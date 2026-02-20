package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArtifactListEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("artifact", "list")

	assert.Equal(t, 0, exitCode, "apex artifact list should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "No artifacts stored", "empty store should report no artifacts")
}

func TestArtifactInfoNotFound(t *testing.T) {
	env := newTestEnv(t)

	_, stderr, exitCode := env.runApex("artifact", "info", "nonexistent")

	assert.NotEqual(t, 0, exitCode, "apex artifact info on missing hash should exit non-zero")
	assert.Contains(t, stderr, "not found", "stderr should mention 'not found'")
}

func TestArtifactGCDryRun(t *testing.T) {
	env := newTestEnv(t)

	// Set up a fake artifact store with an orphan artifact.
	artifactsDir := filepath.Join(env.Home, ".apex", "artifacts")
	blobDir := filepath.Join(artifactsDir, "blobs", "ab")
	require.NoError(t, os.MkdirAll(blobDir, 0755))

	// Write a fake blob file.
	blobPath := filepath.Join(blobDir, "ab1234567890")
	require.NoError(t, os.WriteFile(blobPath, []byte("fake blob data"), 0644))

	// Write index.json with one artifact entry referencing the orphan run.
	type indexEntry struct {
		Hash      string `json:"hash"`
		Name      string `json:"name"`
		RunID     string `json:"run_id"`
		NodeID    string `json:"node_id"`
		Size      int64  `json:"size"`
		CreatedAt string `json:"created_at"`
	}
	index := []indexEntry{
		{
			Hash:      "ab1234567890",
			Name:      "orphan-artifact.txt",
			RunID:     "orphan-run",
			NodeID:    "node-1",
			Size:      14,
			CreatedAt: "2025-01-01T00:00:00Z",
		},
	}
	indexData, err := json.MarshalIndent(index, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(artifactsDir, "index.json"), indexData, 0644))

	stdout, stderr, exitCode := env.runApex("artifact", "gc", "--dry-run")

	assert.Equal(t, 0, exitCode, "apex artifact gc --dry-run should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "orphan", "output should mention orphan artifacts")
	assert.Contains(t, stdout, "dry-run", "output should indicate dry-run mode")
}

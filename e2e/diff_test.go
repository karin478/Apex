package e2e_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// apex diff
// ---------------------------------------------------------------------------

// TestDiffMissingRun verifies that diffing two nonexistent runs produces a
// non-zero exit code and an error message mentioning "failed to load".
func TestDiffMissingRun(t *testing.T) {
	env := newTestEnv(t)

	_, stderr, code := env.runApex("diff", "nonexistent-1", "nonexistent-2")

	assert.NotEqual(t, 0, code, "apex diff with missing runs should exit non-zero")
	assert.Contains(t, stderr, "failed to load",
		"stderr should mention 'failed to load'; got: %s", stderr)
}

// TestDiffTwoRuns creates two fake manifests with different field values,
// runs `apex diff`, and verifies that the human-readable output contains
// the differing field names and their values.
func TestDiffTwoRuns(t *testing.T) {
	env := newTestEnv(t)

	// Create two manifests with differing Model and Outcome fields.
	for _, tc := range []struct {
		id      string
		model   string
		outcome string
	}{
		{"run-aaa", "sonnet", "success"},
		{"run-bbb", "opus", "failure"},
	} {
		dir := filepath.Join(env.runsDir(), tc.id)
		require.NoError(t, os.MkdirAll(dir, 0755))
		data := []byte(fmt.Sprintf(
			`{"run_id":"%s","model":"%s","outcome":"%s","nodes":[]}`,
			tc.id, tc.model, tc.outcome,
		))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0644))
	}

	stdout, stderr, code := env.runApex("diff", "run-aaa", "run-bbb")
	require.Equal(t, 0, code, "apex diff should exit 0; stderr: %s", stderr)

	// The human output should contain the field names that differ and their values.
	assert.Contains(t, stdout, "Model", "output should mention the Model field")
	assert.Contains(t, stdout, "sonnet", "output should contain left model value")
	assert.Contains(t, stdout, "opus", "output should contain right model value")
	assert.Contains(t, stdout, "Outcome", "output should mention the Outcome field")
	assert.Contains(t, stdout, "success", "output should contain left outcome value")
	assert.Contains(t, stdout, "failure", "output should contain right outcome value")
}

// TestDiffJSONFormat creates two fake manifests, runs `apex diff` with
// --format json, and verifies the output is valid JSON with expected keys.
func TestDiffJSONFormat(t *testing.T) {
	env := newTestEnv(t)

	// Create two manifests with differing Model fields.
	for _, tc := range []struct {
		id    string
		model string
	}{
		{"run-x", "sonnet"},
		{"run-y", "haiku"},
	} {
		dir := filepath.Join(env.runsDir(), tc.id)
		require.NoError(t, os.MkdirAll(dir, 0755))
		data := []byte(fmt.Sprintf(
			`{"run_id":"%s","model":"%s","outcome":"success","nodes":[]}`,
			tc.id, tc.model,
		))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0644))
	}

	stdout, stderr, code := env.runApex("diff", "run-x", "run-y", "--format", "json")
	require.Equal(t, 0, code, "apex diff --format json should exit 0; stderr: %s", stderr)

	// Parse the JSON output.
	var result map[string]interface{}
	err := json.Unmarshal([]byte(stdout), &result)
	require.NoError(t, err, "output should be valid JSON; got: %s", stdout)

	// Verify expected top-level keys exist.
	assert.Contains(t, result, "left_run_id", "JSON should contain left_run_id")
	assert.Contains(t, result, "right_run_id", "JSON should contain right_run_id")
	assert.Contains(t, result, "fields", "JSON should contain fields")

	// Verify the run IDs match.
	assert.Equal(t, "run-x", result["left_run_id"], "left_run_id should be run-x")
	assert.Equal(t, "run-y", result["right_run_id"], "right_run_id should be run-y")
}

package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Causal Chain / Tracing E2E Tests
// ---------------------------------------------------------------------------

// TestRunShowsTraceID verifies that "apex run" prints a [trace: ...] line
// at the start of its output.
func TestRunShowsTraceID(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, code := env.runApex("run", "say hello")
	_ = stderr
	_ = code

	assert.Contains(t, stdout, "[trace:", "run output should contain [trace: ...] line")
}

// TestTraceCommand verifies that "apex trace" (no args) shows trace entries
// for the most recent run.
func TestTraceCommand(t *testing.T) {
	env := newTestEnv(t)

	// First run a task to generate a manifest and audit entries
	env.runApex("run", "say hello")

	// Then run trace command
	stdout, stderr, code := env.runApex("trace")
	assert.Equal(t, 0, code, "apex trace should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "Trace:", "trace output should contain 'Trace:' header")
}

// TestManifestContainsTraceID verifies that after a run, the manifest JSON
// file contains a trace_id field.
func TestManifestContainsTraceID(t *testing.T) {
	env := newTestEnv(t)

	// Run a task
	env.runApex("run", "say hello")

	// Find the manifest file
	runsDir := env.runsDir()
	entries, err := os.ReadDir(runsDir)
	require.NoError(t, err, "should be able to read runs dir")
	require.NotEmpty(t, entries, "runs dir should contain at least one entry")

	// Find the first directory (run ID)
	var manifestPath string
	for _, entry := range entries {
		if entry.IsDir() {
			p := filepath.Join(runsDir, entry.Name(), "manifest.json")
			if _, statErr := os.Stat(p); statErr == nil {
				manifestPath = p
				break
			}
		}
	}
	require.NotEmpty(t, manifestPath, "should find a manifest.json file")

	// Read and parse the manifest
	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	traceID, ok := m["trace_id"]
	assert.True(t, ok, "manifest should contain trace_id field")
	assert.NotEmpty(t, traceID, "trace_id should not be empty")

	// Verify trace_id looks like a UUID (contains dashes)
	traceStr, isStr := traceID.(string)
	assert.True(t, isStr, "trace_id should be a string")
	assert.True(t, strings.Contains(traceStr, "-"), "trace_id should be a UUID format")
}

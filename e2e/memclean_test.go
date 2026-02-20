package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemoryCleanupDryRun verifies that "apex memory cleanup --dry-run"
// exits 0 and prints a dry-run preview with summary information when
// memory entries exist.
func TestMemoryCleanupDryRun(t *testing.T) {
	env := newTestEnv(t)

	// Create .claude/memory/ with a few .md files so the scan finds entries.
	claudeMemDir := filepath.Join(env.Home, ".claude", "memory", "facts")
	require.NoError(t, os.MkdirAll(claudeMemDir, 0755),
		"creating .claude/memory/facts should not fail")

	for _, name := range []string{"fact-one.md", "fact-two.md", "fact-three.md"} {
		require.NoError(t,
			os.WriteFile(filepath.Join(claudeMemDir, name), []byte("test content"), 0644),
			"writing test memory file should not fail")
	}

	stdout, stderr, exitCode := env.runApex("memory", "cleanup", "--dry-run")

	assert.Equal(t, 0, exitCode,
		"apex memory cleanup --dry-run should exit 0; stderr=%s", stderr)

	lower := strings.ToLower(stdout)
	assert.True(t, strings.Contains(lower, "dry run"),
		"stdout should contain 'Dry run' or 'dry run', got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "Scanned="),
		"stdout should contain summary info with 'Scanned=', got: %s", stdout)
}

// TestMemoryCleanupEmpty verifies that running cleanup on an empty memory
// directory prints "Nothing to clean" and exits 0.
func TestMemoryCleanupEmpty(t *testing.T) {
	env := newTestEnv(t)

	// Ensure .claude/memory/ exists but is empty (no .md or .jsonl files).
	claudeMemDir := filepath.Join(env.Home, ".claude", "memory")
	require.NoError(t, os.MkdirAll(claudeMemDir, 0755),
		"creating .claude/memory should not fail")

	stdout, stderr, exitCode := env.runApex("memory", "cleanup")

	assert.Equal(t, 0, exitCode,
		"apex memory cleanup should exit 0; stderr=%s", stderr)

	lower := strings.ToLower(stdout)
	assert.True(t, strings.Contains(lower, "nothing to clean"),
		"stdout should contain 'Nothing to clean' or 'nothing to clean', got: %s", stdout)
}

// TestMemoryCleanupRuns verifies that "apex memory cleanup" executes
// cleanly and exits with code 0.
func TestMemoryCleanupRuns(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("memory", "cleanup")
	_ = stdout // output is not relevant; we only check exit code

	assert.Equal(t, 0, exitCode,
		"apex memory cleanup should exit 0; stderr=%s", stderr)
}

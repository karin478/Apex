package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGCEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, code := env.runApex("gc")
	assert.Equal(t, 0, code, "apex gc should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "[GC]", "should show GC output")
}

func TestGCDryRun(t *testing.T) {
	env := newTestEnv(t)

	// Run a task first to create data
	env.runApex("run", "say hello")

	stdout, stderr, code := env.runApex("gc", "--dry-run", "--max-runs", "0", "--max-age", "0")
	assert.Equal(t, 0, code, "apex gc --dry-run should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "Dry run", "should indicate dry run mode")

	// Verify runs directory still has content (dry run didn't delete)
	runsDir := env.runsDir()
	assert.DirExists(t, runsDir, "runs dir should still exist after dry run")
}

func TestGCAfterRun(t *testing.T) {
	env := newTestEnv(t)

	// Run a task to create manifest
	env.runApex("run", "say hello")

	// GC with aggressive settings to clean everything
	stdout, stderr, code := env.runApex("gc", "--max-runs", "0", "--max-age", "0")
	assert.Equal(t, 0, code, "apex gc should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "Removed", "should report removed items")
}

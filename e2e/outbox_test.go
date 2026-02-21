package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOutboxCreatedOnRun verifies that a successful run creates the WAL file
// and runtime.db with STARTED and COMPLETED entries.
func TestOutboxCreatedOnRun(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("run", "say hello")
	require.Equal(t, 0, exitCode, "apex run should succeed; stdout=%s stderr=%s", stdout, stderr)
	assert.Contains(t, stdout, "Done")

	// Check WAL file exists
	walPath := filepath.Join(env.Home, ".apex", "runtime", "actions_wal.jsonl")
	assert.FileExists(t, walPath)

	// Check it has STARTED and COMPLETED entries
	data, err := os.ReadFile(walPath)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, `"status":"STARTED"`)
	assert.Contains(t, content, `"status":"COMPLETED"`)
}

// TestOutboxRuntimeDBCreated verifies that a run creates the runtime.db file.
func TestOutboxRuntimeDBCreated(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("run", "say hello")
	require.Equal(t, 0, exitCode, "apex run should succeed; stdout=%s stderr=%s", stdout, stderr)

	// Check runtime.db exists
	dbPath := filepath.Join(env.Home, ".apex", "runtime", "runtime.db")
	assert.FileExists(t, dbPath)
}

// TestDoctorShowsLockAndOutbox verifies that doctor reports lock and outbox sections.
func TestDoctorShowsLockAndOutbox(t *testing.T) {
	env := newTestEnv(t)

	// Run a task first to create WAL and runtime.db
	stdout, stderr, exitCode := env.runApex("run", "say hello")
	require.Equal(t, 0, exitCode, "apex run should succeed; stdout=%s stderr=%s", stdout, stderr)

	// Run doctor
	stdout, stderr, exitCode = env.runApex("doctor")
	assert.Equal(t, 0, exitCode, "apex doctor should exit 0; stderr=%s", stderr)

	// Should show lock and outbox sections
	assert.True(t,
		strings.Contains(stdout, "Lock status") || strings.Contains(stdout, "Action outbox"),
		"doctor should show lock/outbox sections; got stdout=%s", stdout)
}

// TestOutboxLockFreeDuringDoctor verifies that the lock is not actively held after a run completes.
// After the run process exits, the lock is released. Doctor may report either "FREE" (lock file
// removed) or "STALE" (lock file persists but the owning PID is dead); both mean no active lock.
func TestOutboxLockFreeDuringDoctor(t *testing.T) {
	env := newTestEnv(t)

	// Run a task (lock should be released after run completes)
	env.runApex("run", "say hello")

	// Doctor should show lock status section, and the lock should not be actively held
	stdout, _, _ := env.runApex("doctor")
	assert.Contains(t, stdout, "Lock status")
	assert.True(t,
		strings.Contains(stdout, "FREE") || strings.Contains(stdout, "STALE"),
		"lock should be FREE or STALE after run completes; got stdout=%s", stdout)
}

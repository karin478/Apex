package e2e_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKillSwitchActivateAndResume verifies the full lifecycle:
//  1. apex kill-switch activates and creates the file
//  2. apex run is blocked while kill switch is active
//  3. apex resume deactivates and removes the file
//  4. apex run succeeds again after resume
func TestKillSwitchActivateAndResume(t *testing.T) {
	env := newTestEnv(t)

	// Step 1: Activate the kill switch
	stdout, _, code := env.runApex("kill-switch", "testing emergency")
	assert.Equal(t, 0, code, "kill-switch should exit 0")
	assert.Contains(t, stdout, "ACTIVATED", "stdout should contain ACTIVATED")
	require.True(t, env.fileExists(env.killSwitchPath()), "kill switch file should exist")

	// Step 2: Run should be blocked by kill switch
	_, _, code = env.runApex("run", "say hello")
	assert.NotEqual(t, 0, code, "run should exit non-zero when kill switch is active")

	// Step 3: Resume (deactivate) the kill switch
	stdout, _, code = env.runApex("resume")
	assert.Equal(t, 0, code, "resume should exit 0")
	assert.Contains(t, stdout, "DEACTIVATED", "stdout should contain DEACTIVATED")
	assert.False(t, env.fileExists(env.killSwitchPath()), "kill switch file should be removed")

	// Step 4: Run should work again after resume
	stdout, _, code = env.runApex("run", "say hello")
	assert.Equal(t, 0, code, "run should exit 0 after resume")
	assert.Contains(t, stdout, "Done", "stdout should contain Done")
}

// TestKillSwitchAlreadyActive verifies that activating an already-active
// kill switch reports the existing state rather than failing.
func TestKillSwitchAlreadyActive(t *testing.T) {
	env := newTestEnv(t)

	// Pre-create the kill switch file manually
	err := os.WriteFile(env.killSwitchPath(), []byte("pre-existing"), 0644)
	require.NoError(t, err, "failed to pre-create kill switch file")

	// Activate again — should report already active
	stdout, _, code := env.runApex("kill-switch", "again")
	assert.Equal(t, 0, code, "kill-switch should exit 0 even when already active")
	assert.Contains(t, stdout, "already active", "stdout should indicate kill switch is already active")
}

// TestResumeWhenNotActive verifies that resuming when no kill switch is
// active succeeds gracefully with an informative message.
func TestResumeWhenNotActive(t *testing.T) {
	env := newTestEnv(t)

	// Ensure no kill switch file exists
	assert.False(t, env.fileExists(env.killSwitchPath()), "precondition: no kill switch file")

	// Resume should succeed and report no active kill switch
	stdout, _, code := env.runApex("resume")
	assert.Equal(t, 0, code, "resume should exit 0 when no kill switch is active")
	assert.Contains(t, stdout, "No kill switch active", "stdout should indicate no kill switch is active")
}

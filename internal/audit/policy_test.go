package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPolicyTrackerNewFile verifies that the first check records a new file
// and returns a change with an empty OldChecksum.
func TestPolicyTrackerNewFile(t *testing.T) {
	stateDir := t.TempDir()
	tracker := NewPolicyTracker(stateDir)

	// Create a temporary config file
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte("key: value\n"), 0o644))

	changes, err := tracker.Check([]string{configFile})
	require.NoError(t, err)

	require.Len(t, changes, 1)
	assert.Equal(t, configFile, changes[0].File)
	assert.Empty(t, changes[0].OldChecksum, "OldChecksum should be empty for new files")
	assert.NotEmpty(t, changes[0].NewChecksum)
	assert.NotEmpty(t, changes[0].Timestamp)
}

// TestPolicyTrackerNoChange verifies that a second check with the same content
// returns no changes.
func TestPolicyTrackerNoChange(t *testing.T) {
	stateDir := t.TempDir()
	tracker := NewPolicyTracker(stateDir)

	configFile := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte("key: value\n"), 0o644))

	// First check — records the file
	_, err := tracker.Check([]string{configFile})
	require.NoError(t, err)

	// Second check — no modifications, expect no changes
	changes, err := tracker.Check([]string{configFile})
	require.NoError(t, err)
	assert.Empty(t, changes, "expected no changes when file content is unchanged")
}

// TestPolicyTrackerDetectsChange verifies that modifying a file between checks
// produces a change with both old and new checksums populated.
func TestPolicyTrackerDetectsChange(t *testing.T) {
	stateDir := t.TempDir()
	tracker := NewPolicyTracker(stateDir)

	configFile := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte("key: value\n"), 0o644))

	// First check
	firstChanges, err := tracker.Check([]string{configFile})
	require.NoError(t, err)
	require.Len(t, firstChanges, 1)
	originalChecksum := firstChanges[0].NewChecksum

	// Modify the file
	require.NoError(t, os.WriteFile(configFile, []byte("key: updated\n"), 0o644))

	// Second check — should detect the change
	changes, err := tracker.Check([]string{configFile})
	require.NoError(t, err)

	require.Len(t, changes, 1)
	assert.Equal(t, configFile, changes[0].File)
	assert.Equal(t, originalChecksum, changes[0].OldChecksum)
	assert.NotEqual(t, originalChecksum, changes[0].NewChecksum)
	assert.NotEmpty(t, changes[0].Timestamp)
}

// TestPolicyTrackerState verifies that State() returns nil initially
// and returns populated data after Check() has been called.
func TestPolicyTrackerState(t *testing.T) {
	stateDir := t.TempDir()
	tracker := NewPolicyTracker(stateDir)

	// State should be nil before any check
	state, err := tracker.State()
	require.NoError(t, err)
	assert.Nil(t, state, "State() should return nil before any check")

	// Create and check a file
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte("key: value\n"), 0o644))

	_, err = tracker.Check([]string{configFile})
	require.NoError(t, err)

	// State should now be populated
	state, err = tracker.State()
	require.NoError(t, err)
	require.Len(t, state, 1)
	assert.Equal(t, configFile, state[0].Path)
	assert.NotEmpty(t, state[0].Checksum)
}

// TestPolicyTrackerSkipsMissingFile verifies that a nonexistent file path
// is silently skipped — no error, no changes.
func TestPolicyTrackerSkipsMissingFile(t *testing.T) {
	stateDir := t.TempDir()
	tracker := NewPolicyTracker(stateDir)

	changes, err := tracker.Check([]string{"/nonexistent/path/config.yaml"})
	require.NoError(t, err)
	assert.Empty(t, changes, "expected no changes for missing files")
}

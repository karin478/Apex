package audit

import (
	"os"
	"path/filepath"
	"strings"
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

// TestFormatPolicyChangesEmpty verifies that nil/empty changes produce
// the "No policy changes detected." message.
func TestFormatPolicyChangesEmpty(t *testing.T) {
	result := FormatPolicyChanges(nil)
	assert.Equal(t, "No policy changes detected.\n", result)

	result = FormatPolicyChanges([]PolicyChange{})
	assert.Equal(t, "No policy changes detected.\n", result)
}

// TestFormatPolicyChangesWithData verifies that a non-empty change list
// renders a table with the header, file basename, and truncated checksums.
func TestFormatPolicyChangesWithData(t *testing.T) {
	changes := []PolicyChange{
		{
			File:        "/etc/configs/config.yaml",
			OldChecksum: "aabbccddeeff112233445566",
			NewChecksum: "5544332211ffeeddccbbaa99",
			Timestamp:   "2026-02-20T10:00:00Z",
		},
	}

	result := FormatPolicyChanges(changes)

	assert.True(t, strings.Contains(result, "Policy Change History"),
		"expected output to contain header")
	assert.True(t, strings.Contains(result, "config.yaml"),
		"expected output to contain file basename")
	assert.True(t, strings.Contains(result, "aabbccddee..."),
		"expected old checksum to be truncated to 10 chars + ...")
	assert.True(t, strings.Contains(result, "5544332211..."),
		"expected new checksum to be truncated to 10 chars + ...")
	// Verify full checksums are NOT present
	assert.False(t, strings.Contains(result, "aabbccddeeff112233445566"),
		"full old checksum should not appear in output")

	// Verify the full path is NOT displayed (only basename)
	assert.False(t, strings.Contains(result, "/etc/configs/config.yaml"),
		"full file path should not appear; only basename expected")
}

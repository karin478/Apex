package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemoryExportEmpty verifies that exporting from an empty memory
// directory produces valid JSON with version and an empty entries array.
func TestMemoryExportEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("memory", "export")

	assert.Equal(t, 0, exitCode, "apex memory export should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, `"entries"`, "export output should contain entries field")
	assert.Contains(t, stdout, `"version"`, "export output should contain version field")
}

// TestMemoryImportFile verifies that importing a JSON file with one entry
// creates the corresponding memory file and reports Added: 1.
func TestMemoryImportFile(t *testing.T) {
	env := newTestEnv(t)

	// Create a valid export JSON file with one entry.
	exportJSON := `{
  "version": "1",
  "exported_at": "2026-01-01T00:00:00Z",
  "count": 1,
  "entries": [
    {
      "key": "facts/test-fact.md",
      "value": "test content",
      "category": "facts",
      "created_at": "2026-01-01T00:00:00Z"
    }
  ]
}`

	importFile := filepath.Join(env.Home, "import.json")
	require.NoError(t, os.WriteFile(importFile, []byte(exportJSON), 0644),
		"writing import file should not fail")

	stdout, stderr, exitCode := env.runApex("memory", "import", importFile)

	assert.Equal(t, 0, exitCode, "apex memory import should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "Added=1", "import should report 1 added entry")
}

// TestMemoryImportSkip verifies that importing the same file twice with
// the default "skip" strategy reports all entries as skipped on the second run.
func TestMemoryImportSkip(t *testing.T) {
	env := newTestEnv(t)

	// Create a valid export JSON file with one entry.
	exportJSON := `{
  "version": "1",
  "exported_at": "2026-01-01T00:00:00Z",
  "count": 1,
  "entries": [
    {
      "key": "facts/test-fact.md",
      "value": "test content",
      "category": "facts",
      "created_at": "2026-01-01T00:00:00Z"
    }
  ]
}`

	importFile := filepath.Join(env.Home, "import.json")
	require.NoError(t, os.WriteFile(importFile, []byte(exportJSON), 0644),
		"writing import file should not fail")

	// First import — should add the entry.
	stdout1, stderr1, exitCode1 := env.runApex("memory", "import", importFile)
	require.Equal(t, 0, exitCode1, "first import should exit 0; stderr=%s", stderr1)
	require.Contains(t, stdout1, "Added=1", "first import should add 1 entry")

	// Second import with default skip strategy — should skip the entry.
	stdout2, stderr2, exitCode2 := env.runApex("memory", "import", importFile, "--strategy", "skip")

	assert.Equal(t, 0, exitCode2, "second import should exit 0; stderr=%s", stderr2)
	assert.Contains(t, stdout2, "Skipped=1", "second import should report 1 skipped entry")
}

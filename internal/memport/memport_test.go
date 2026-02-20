package memport

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: populate a temp memory directory with sample files.
func seedMemDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// decisions/test-decision.md — includes YAML frontmatter
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "decisions"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "decisions", "test-decision.md"),
		[]byte("---\ntype: decision\nslug: test-decision\n---\n\n# Test Decision\n\nChose approach A over B.\n"),
		0644,
	))

	// facts/test-fact.md
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "facts"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "facts", "test-fact.md"),
		[]byte("# Test Fact\n\nProject uses Go 1.25.\n"),
		0644,
	))

	// sessions/test-session.jsonl
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sessions"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "sessions", "test-session.jsonl"),
		[]byte(`{"timestamp":"2026-01-15T10:00:00Z","task":"init","result":"ok"}`+"\n"),
		0644,
	))

	return dir
}

func TestExportAll(t *testing.T) {
	dir := seedMemDir(t)

	data, err := Export(dir, "")
	require.NoError(t, err)

	assert.Equal(t, "1", data.Version)
	assert.NotEmpty(t, data.ExportedAt)
	assert.Equal(t, 3, data.Count)
	assert.Len(t, data.Entries, 3)

	// Verify all three categories are present.
	cats := map[string]bool{}
	for _, e := range data.Entries {
		cats[e.Category] = true
		assert.NotEmpty(t, e.Key)
		assert.NotEmpty(t, e.Value)
		assert.NotEmpty(t, e.CreatedAt)
	}
	assert.True(t, cats["decisions"], "should contain decisions")
	assert.True(t, cats["facts"], "should contain facts")
	assert.True(t, cats["sessions"], "should contain sessions")
}

func TestExportByCategory(t *testing.T) {
	dir := seedMemDir(t)

	data, err := Export(dir, "decisions")
	require.NoError(t, err)

	assert.Equal(t, 1, data.Count)
	require.Len(t, data.Entries, 1)
	assert.Equal(t, "decisions", data.Entries[0].Category)
	assert.Contains(t, data.Entries[0].Value, "Test Decision")
}

func TestExportEmpty(t *testing.T) {
	dir := t.TempDir() // empty directory, no files

	data, err := Export(dir, "")
	require.NoError(t, err)

	assert.Equal(t, 0, data.Count)
	assert.Empty(t, data.Entries)
}

func TestImportNew(t *testing.T) {
	// Export from a seeded dir, then import into a fresh dir.
	srcDir := seedMemDir(t)
	data, err := Export(srcDir, "")
	require.NoError(t, err)

	dstDir := t.TempDir()
	result, err := Import(dstDir, data, MergeSkip)
	require.NoError(t, err)

	assert.Equal(t, 3, result.Added)
	assert.Equal(t, 0, result.Skipped)
	assert.Equal(t, 0, result.Overwritten)

	// Verify files actually exist on disk.
	for _, entry := range data.Entries {
		content, err := os.ReadFile(filepath.Join(dstDir, entry.Key))
		require.NoError(t, err)
		assert.Equal(t, entry.Value, string(content))
	}
}

func TestImportSkip(t *testing.T) {
	dir := seedMemDir(t)
	data, err := Export(dir, "")
	require.NoError(t, err)

	// Import into the same dir where files already exist — strategy=skip.
	result, err := Import(dir, data, MergeSkip)
	require.NoError(t, err)

	assert.Equal(t, 0, result.Added)
	assert.Equal(t, 3, result.Skipped)
	assert.Equal(t, 0, result.Overwritten)
}

func TestImportOverwrite(t *testing.T) {
	dir := seedMemDir(t)
	data, err := Export(dir, "")
	require.NoError(t, err)

	// Mutate entry values so we can verify overwrite actually wrote new content.
	for i := range data.Entries {
		data.Entries[i].Value = "overwritten content for " + data.Entries[i].Key
	}

	result, err := Import(dir, data, MergeOverwrite)
	require.NoError(t, err)

	assert.Equal(t, 0, result.Added)
	assert.Equal(t, 0, result.Skipped)
	assert.Equal(t, 3, result.Overwritten)

	// Verify files contain the new content.
	for _, entry := range data.Entries {
		content, err := os.ReadFile(filepath.Join(dir, entry.Key))
		require.NoError(t, err)
		assert.Equal(t, "overwritten content for "+entry.Key, string(content))
	}
}

func TestWriteAndReadFile(t *testing.T) {
	dir := seedMemDir(t)
	original, err := Export(dir, "")
	require.NoError(t, err)

	// Write to a temp JSON file, then read it back.
	outPath := filepath.Join(t.TempDir(), "export.json")
	require.NoError(t, WriteFile(outPath, original))

	loaded, err := ReadFile(outPath)
	require.NoError(t, err)

	assert.Equal(t, original.Version, loaded.Version)
	assert.Equal(t, original.ExportedAt, loaded.ExportedAt)
	assert.Equal(t, original.Count, loaded.Count)
	require.Len(t, loaded.Entries, len(original.Entries))

	for i, entry := range original.Entries {
		assert.Equal(t, entry.Key, loaded.Entries[i].Key)
		assert.Equal(t, entry.Value, loaded.Entries[i].Value)
		assert.Equal(t, entry.Category, loaded.Entries[i].Category)
		assert.Equal(t, entry.CreatedAt, loaded.Entries[i].CreatedAt)
	}
}

func TestImportNilData(t *testing.T) {
	dir := t.TempDir()
	result, err := Import(dir, nil, MergeSkip)
	assert.Nil(t, result)
	assert.EqualError(t, err, "memport: nil export data")
}

func TestExportNonExistentDir(t *testing.T) {
	data, err := Export("/tmp/nonexistent-memport-dir-12345", "")
	require.NoError(t, err)
	assert.Equal(t, 0, data.Count)
	assert.Empty(t, data.Entries)
}

package memory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)
	assert.NotNil(t, store)
}

func TestSaveDecision(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)

	err = store.SaveDecision("auth-refactor", "Chose JWT over session-based auth for stateless scaling.")
	require.NoError(t, err)

	// Verify file exists in decisions/
	files, _ := filepath.Glob(filepath.Join(dir, "decisions", "*-auth-refactor.md"))
	assert.Len(t, files, 1)

	content, _ := os.ReadFile(files[0])
	assert.Contains(t, string(content), "auth-refactor")
	assert.Contains(t, string(content), "JWT over session-based")
}

func TestSaveFact(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)

	err = store.SaveFact("go-version", "Project uses Go 1.25")
	require.NoError(t, err)

	files, _ := filepath.Glob(filepath.Join(dir, "facts", "*-go-version.md"))
	assert.Len(t, files, 1)
}

func TestSaveSession(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)

	err = store.SaveSession("sess-001", "refactor auth", "Done. Used JWT.")
	require.NoError(t, err)

	path := filepath.Join(dir, "sessions", "sess-001.jsonl")
	assert.FileExists(t, path)

	content, _ := os.ReadFile(path)
	assert.Contains(t, string(content), "refactor auth")
}

func TestSearch(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)

	store.SaveDecision("redis-migration", "Migrated from Redis 6 to Redis 7 for ACL support.")
	store.SaveFact("redis-version", "Production runs Redis 7.2")
	store.SaveDecision("db-choice", "Chose PostgreSQL for main database.")

	results, err := store.Search("redis")
	require.NoError(t, err)
	assert.Len(t, results, 2, "should find 2 files mentioning redis")
}

func TestSearchNoResults(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)

	results, err := store.Search("nonexistent-term-xyz")
	require.NoError(t, err)
	assert.Empty(t, results)
}

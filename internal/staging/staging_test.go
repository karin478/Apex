package staging

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/lyndonlyu/apex/internal/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T) (*Stager, *sql.DB) {
	t.Helper()
	dir := t.TempDir()

	db, err := sql.Open("sqlite3", filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	memDir := filepath.Join(dir, "memory")
	store, err := memory.NewStore(memDir)
	require.NoError(t, err)

	stager, err := New(db, store)
	require.NoError(t, err)
	return stager, db
}

func TestStageCreatesEntry(t *testing.T) {
	s, _ := setupTest(t)

	id, err := s.Stage("test memory content", "fact", "run-001")
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	entries, err := s.ListPending()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, id, entries[0].ID)
	assert.Equal(t, "PENDING", entries[0].StagingState)
	assert.Equal(t, 1.0, entries[0].Confidence)
}

func TestVerifyAndCommit(t *testing.T) {
	s, _ := setupTest(t)

	id, err := s.Stage("verified memory", "decision", "run-002")
	require.NoError(t, err)

	err = s.Verify(id)
	require.NoError(t, err)

	err = s.Commit(id)
	require.NoError(t, err)

	// Should no longer be in pending list
	entries, err := s.ListPending()
	require.NoError(t, err)
	assert.Len(t, entries, 0)
}

func TestReject(t *testing.T) {
	s, _ := setupTest(t)

	id, err := s.Stage("bad memory", "fact", "run-003")
	require.NoError(t, err)

	err = s.Reject(id)
	require.NoError(t, err)

	entries, err := s.ListPending()
	require.NoError(t, err)
	assert.Len(t, entries, 0)
}

func TestExpireStale(t *testing.T) {
	s, db := setupTest(t)

	// Insert a stale entry manually (created 2h ago)
	oldTime := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO staging_memories (id, content, category, source, staging_state, confidence, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"stale-1", "old content", "fact", "run-old", "PENDING", 1.0, oldTime,
	)
	require.NoError(t, err)

	expired, err := s.ExpireStale(1 * time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, expired)
}

func TestCommitAll(t *testing.T) {
	s, _ := setupTest(t)

	s.Stage("mem1", "fact", "run-1")
	id2, _ := s.Stage("mem2", "fact", "run-1")

	// Verify only one
	s.Verify(id2)

	committed, err := s.CommitAll()
	require.NoError(t, err)
	assert.Equal(t, 1, committed)
}

func TestCommitUnverified(t *testing.T) {
	s, db := setupTest(t)

	id, err := s.Stage("unverified mem", "fact", "run-1")
	require.NoError(t, err)

	// Mark as UNVERIFIED directly
	_, err = db.Exec(`UPDATE staging_memories SET staging_state = 'UNVERIFIED', confidence = 0.8 WHERE id = ?`, id)
	require.NoError(t, err)

	err = s.Commit(id)
	require.NoError(t, err)
}

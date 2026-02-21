package invariant

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	// Create actions table (same schema as outbox)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS actions (
		action_id TEXT PRIMARY KEY,
		trace_id TEXT NOT NULL,
		task TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'STARTED',
		result_ref TEXT,
		started_at TEXT NOT NULL,
		completed_at TEXT,
		error TEXT,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	require.NoError(t, err)
	return db
}

func TestRunAllEmptyDB(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	r := NewRunner(db, dir)

	results := r.RunAll()
	// Should have 9 results, all PASS or SKIP
	assert.Len(t, results, 9)
	for _, res := range results {
		assert.Contains(t, []string{"PASS", "SKIP"}, res.Status,
			"checker %s should PASS or SKIP on empty db, got %s: %s", res.ID, res.Status, res.Detail)
	}
}

func TestRunSubset(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	r := NewRunner(db, dir)

	results := r.Run("I1", "I3")
	assert.Len(t, results, 2)
	assert.Equal(t, "I1", results[0].ID)
	assert.Equal(t, "I3", results[1].ID)
}

func TestI1WALDBConsistencyPass(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	r := NewRunner(db, dir)

	// No WAL file → PASS (nothing to verify)
	result := r.Run("I1")
	require.Len(t, result, 1)
	assert.Equal(t, "PASS", result[0].Status)
}

func TestI3NoHangingActionsPass(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	r := NewRunner(db, dir)

	result := r.Run("I3")
	require.Len(t, result, 1)
	assert.Equal(t, "PASS", result[0].Status)
}

func TestI3HangingActionFails(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()

	// Insert a STARTED action with old timestamp (>1h ago)
	_, err := db.Exec(
		`INSERT INTO actions (action_id, trace_id, task, status, started_at) VALUES (?, ?, ?, ?, ?)`,
		"old-action", "trace-1", "stale task", "STARTED", "2020-01-01T00:00:00Z",
	)
	require.NoError(t, err)

	r := NewRunner(db, dir)
	result := r.Run("I3")
	require.Len(t, result, 1)
	assert.Equal(t, "FAIL", result[0].Status)
	assert.Contains(t, result[0].Detail, "1 hanging")
}

func TestI4IdempotencyPass(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()

	// Insert unique action_ids
	_, err := db.Exec(
		`INSERT INTO actions (action_id, trace_id, task, status, started_at) VALUES (?, ?, ?, ?, ?)`,
		"a1", "t1", "task1", "COMPLETED", "2026-01-01T00:00:00Z",
	)
	require.NoError(t, err)

	r := NewRunner(db, dir)
	result := r.Run("I4")
	require.Len(t, result, 1)
	assert.Equal(t, "PASS", result[0].Status)
}

func TestI5TraceCompletenessPass(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	r := NewRunner(db, dir)

	result := r.Run("I5")
	require.Len(t, result, 1)
	assert.Equal(t, "PASS", result[0].Status)
}

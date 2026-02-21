package outbox

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lyndonlyu/apex/internal/writerq"
)

func setupOutbox(t *testing.T) (*Outbox, *sql.DB) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "runtime.db")
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	db.Exec("PRAGMA journal_mode=WAL")
	t.Cleanup(func() { db.Close() })

	q := writerq.New(db)
	t.Cleanup(func() { q.Close() })

	walPath := filepath.Join(dir, "actions_wal.jsonl")
	ob, err := New(walPath, db, q)
	require.NoError(t, err)
	return ob, db
}

func TestBeginWritesWALStarted(t *testing.T) {
	ob, _ := setupOutbox(t)

	err := ob.Begin("act-1", "trace-1", "echo hello")
	require.NoError(t, err)

	entries, err := ob.ReadWAL()
	require.NoError(t, err)
	require.Len(t, entries, 1)

	assert.Equal(t, "act-1", entries[0].ActionID)
	assert.Equal(t, "trace-1", entries[0].TraceID)
	assert.Equal(t, "echo hello", entries[0].Task)
	assert.Equal(t, StatusStarted, entries[0].Status)
	assert.NotEmpty(t, entries[0].Timestamp)
}

func TestRecordStartedInsertsDB(t *testing.T) {
	ob, db := setupOutbox(t)

	err := ob.Begin("act-1", "trace-1", "echo hello")
	require.NoError(t, err)

	err = ob.RecordStarted("act-1", "trace-1", "echo hello")
	require.NoError(t, err)

	// writerq batches writes asynchronously
	time.Sleep(100 * time.Millisecond)

	var status string
	err = db.QueryRow("SELECT status FROM actions WHERE action_id = ?", "act-1").Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, StatusStarted, status)
}

func TestCompleteUpdatesDBAndWAL(t *testing.T) {
	ob, db := setupOutbox(t)

	err := ob.Begin("act-1", "trace-1", "echo hello")
	require.NoError(t, err)
	err = ob.RecordStarted("act-1", "trace-1", "echo hello")
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	err = ob.Complete("act-1", "")
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// Check DB
	var status string
	err = db.QueryRow("SELECT status FROM actions WHERE action_id = ?", "act-1").Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, status)

	// Check WAL
	entries, err := ob.ReadWAL()
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, StatusStarted, entries[0].Status)
	assert.Equal(t, StatusCompleted, entries[1].Status)
}

func TestFailUpdatesDBAndWAL(t *testing.T) {
	ob, db := setupOutbox(t)

	err := ob.Begin("act-1", "trace-1", "echo hello")
	require.NoError(t, err)
	err = ob.RecordStarted("act-1", "trace-1", "echo hello")
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	err = ob.Fail("act-1", "error msg")
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// Check DB status and error
	var status, errField string
	err = db.QueryRow("SELECT status, error FROM actions WHERE action_id = ?", "act-1").Scan(&status, &errField)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, status)
	assert.Equal(t, "error msg", errField)

	// Check WAL
	entries, err := ob.ReadWAL()
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, StatusStarted, entries[0].Status)
	assert.Equal(t, StatusFailed, entries[1].Status)
}

func TestReconcileFindsOrphans(t *testing.T) {
	ob, _ := setupOutbox(t)

	// act-1: started + completed
	err := ob.Begin("act-1", "trace-1", "task-1")
	require.NoError(t, err)
	err = ob.RecordStarted("act-1", "trace-1", "task-1")
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	err = ob.Complete("act-1", "")
	require.NoError(t, err)

	// act-2: started only (orphan)
	err = ob.Begin("act-2", "trace-2", "task-2")
	require.NoError(t, err)
	err = ob.RecordStarted("act-2", "trace-2", "task-2")
	require.NoError(t, err)

	// act-3: started + failed
	err = ob.Begin("act-3", "trace-3", "task-3")
	require.NoError(t, err)
	err = ob.RecordStarted("act-3", "trace-3", "task-3")
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	err = ob.Fail("act-3", "some error")
	require.NoError(t, err)

	orphans, err := ob.Reconcile()
	require.NoError(t, err)
	require.Len(t, orphans, 1)
	assert.Equal(t, "act-2", orphans[0].ActionID)
}

func TestReconcileEmptyWAL(t *testing.T) {
	ob, _ := setupOutbox(t)

	// No WAL file exists yet — Reconcile should return empty slice, no error
	orphans, err := ob.Reconcile()
	require.NoError(t, err)
	assert.Empty(t, orphans)
}

func TestMultipleActionsInWAL(t *testing.T) {
	ob, _ := setupOutbox(t)

	// Begin 5 actions
	for i := 1; i <= 5; i++ {
		id := "act-" + string(rune('0'+i))
		err := ob.Begin(id, "trace-"+string(rune('0'+i)), "task-"+string(rune('0'+i)))
		require.NoError(t, err)
		err = ob.RecordStarted(id, "trace-"+string(rune('0'+i)), "task-"+string(rune('0'+i)))
		require.NoError(t, err)
	}
	time.Sleep(100 * time.Millisecond)

	// Complete 3 (act-1, act-2, act-3)
	for i := 1; i <= 3; i++ {
		id := "act-" + string(rune('0'+i))
		err := ob.Complete(id, "")
		require.NoError(t, err)
	}

	// Fail 1 (act-4)
	err := ob.Fail("act-4", "failed")
	require.NoError(t, err)

	// Leave act-5 as orphan

	entries, err := ob.ReadWAL()
	require.NoError(t, err)
	// 5 STARTED + 3 COMPLETED + 1 FAILED = 9 entries
	assert.Len(t, entries, 9)

	// Verify orphan
	orphans, err := ob.Reconcile()
	require.NoError(t, err)
	require.Len(t, orphans, 1)
	assert.Equal(t, "act-5", orphans[0].ActionID)
}

func TestCompleteResultRef(t *testing.T) {
	ob, db := setupOutbox(t)

	err := ob.Begin("act-1", "trace-1", "echo hello")
	require.NoError(t, err)
	err = ob.RecordStarted("act-1", "trace-1", "echo hello")
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	err = ob.Complete("act-1", "ref-123")
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	var resultRef sql.NullString
	err = db.QueryRow("SELECT result_ref FROM actions WHERE action_id = ?", "act-1").Scan(&resultRef)
	require.NoError(t, err)
	assert.True(t, resultRef.Valid)
	assert.Equal(t, "ref-123", resultRef.String)
}

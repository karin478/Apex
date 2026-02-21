package statedb

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/lyndonlyu/apex/internal/writerq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := Open(dbPath)
	require.NoError(t, err)
	require.NotNil(t, db)

	// Verify WAL mode is active.
	var journalMode string
	err = db.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	require.NoError(t, err)
	assert.Equal(t, "wal", journalMode)

	assert.Equal(t, dbPath, db.Path())

	err = db.Close()
	require.NoError(t, err)
}

func TestSetGetState(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer db.Close()

	before := time.Now().UTC().Add(-1 * time.Second)

	err = db.SetState("agent.mode", "autonomous")
	require.NoError(t, err)

	entry, err := db.GetState("agent.mode")
	require.NoError(t, err)
	assert.Equal(t, "agent.mode", entry.Key)
	assert.Equal(t, "autonomous", entry.Value)

	// Verify updated_at is a valid RFC3339 timestamp in a reasonable range.
	ts, err := time.Parse(time.RFC3339, entry.UpdatedAt)
	require.NoError(t, err)
	assert.True(t, ts.After(before), "updated_at should be after test start")
	assert.True(t, ts.Before(time.Now().UTC().Add(1*time.Second)), "updated_at should be before now+1s")

	// Verify ErrNotFound for missing key.
	_, err = db.GetState("nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestSetStateUpsert(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer db.Close()

	err = db.SetState("version", "1.0")
	require.NoError(t, err)

	err = db.SetState("version", "2.0")
	require.NoError(t, err)

	entry, err := db.GetState("version")
	require.NoError(t, err)
	assert.Equal(t, "2.0", entry.Value)
}

func TestDeleteState(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer db.Close()

	err = db.SetState("temp", "value")
	require.NoError(t, err)

	err = db.DeleteState("temp")
	require.NoError(t, err)

	_, err = db.GetState("temp")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestListState(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer db.Close()

	// Insert entries in non-alphabetical order.
	err = db.SetState("charlie", "3")
	require.NoError(t, err)
	err = db.SetState("alpha", "1")
	require.NoError(t, err)
	err = db.SetState("bravo", "2")
	require.NoError(t, err)

	entries, err := db.ListState()
	require.NoError(t, err)
	require.Len(t, entries, 3)

	// Verify sorted by key.
	assert.Equal(t, "alpha", entries[0].Key)
	assert.Equal(t, "1", entries[0].Value)
	assert.Equal(t, "bravo", entries[1].Key)
	assert.Equal(t, "2", entries[1].Value)
	assert.Equal(t, "charlie", entries[2].Key)
	assert.Equal(t, "3", entries[2].Value)
}

func TestInsertGetRun(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	record := RunRecord{
		ID:        "run-001",
		Status:    "PENDING",
		TaskCount: 5,
		StartedAt: now,
		EndedAt:   "",
	}

	err = db.InsertRun(record)
	require.NoError(t, err)

	got, err := db.GetRun("run-001")
	require.NoError(t, err)
	assert.Equal(t, record.ID, got.ID)
	assert.Equal(t, record.Status, got.Status)
	assert.Equal(t, record.TaskCount, got.TaskCount)
	assert.Equal(t, record.StartedAt, got.StartedAt)
	assert.Equal(t, record.EndedAt, got.EndedAt)

	// Verify ErrNotFound for missing run.
	_, err = db.GetRun("nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestUpdateRunStatusAndList(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer db.Close()

	// Insert three runs with different started_at timestamps.
	runs := []RunRecord{
		{ID: "run-a", Status: "RUNNING", TaskCount: 2, StartedAt: "2025-01-01T10:00:00Z", EndedAt: ""},
		{ID: "run-b", Status: "RUNNING", TaskCount: 3, StartedAt: "2025-01-01T11:00:00Z", EndedAt: ""},
		{ID: "run-c", Status: "RUNNING", TaskCount: 1, StartedAt: "2025-01-01T12:00:00Z", EndedAt: ""},
	}
	for _, r := range runs {
		err := db.InsertRun(r)
		require.NoError(t, err)
	}

	// Update run-b to COMPLETED.
	err = db.UpdateRunStatus("run-b", "COMPLETED")
	require.NoError(t, err)

	got, err := db.GetRun("run-b")
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", got.Status)
	assert.NotEmpty(t, got.EndedAt, "ended_at should be set for COMPLETED status")

	// Verify ended_at is a valid RFC3339 timestamp.
	_, err = time.Parse(time.RFC3339, got.EndedAt)
	require.NoError(t, err)

	// ListRuns returns most recent first (by started_at DESC).
	all, err := db.ListRuns(0)
	require.NoError(t, err)
	require.Len(t, all, 3)
	assert.Equal(t, "run-c", all[0].ID)
	assert.Equal(t, "run-b", all[1].ID)
	assert.Equal(t, "run-a", all[2].ID)

	// ListRuns with limit.
	limited, err := db.ListRuns(2)
	require.NoError(t, err)
	require.Len(t, limited, 2)
	assert.Equal(t, "run-c", limited[0].ID)
	assert.Equal(t, "run-b", limited[1].ID)

	// UpdateRunStatus on nonexistent run returns ErrNotFound.
	err = db.UpdateRunStatus("nonexistent", "FAILED")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestSetStateViaQueue(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer db.Close()

	q := writerq.New(db.RawDB())
	defer q.Close()
	db.SetQueue(q)

	err = db.SetState("key1", "value1")
	require.NoError(t, err)

	entry, err := db.GetState("key1")
	require.NoError(t, err)
	assert.Equal(t, "value1", entry.Value)
}

func TestInsertRunViaQueue(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer db.Close()

	q := writerq.New(db.RawDB())
	defer q.Close()
	db.SetQueue(q)

	now := time.Now().UTC().Format(time.RFC3339)
	err = db.InsertRun(RunRecord{
		ID:        "run-q1",
		Status:    "RUNNING",
		TaskCount: 3,
		StartedAt: now,
	})
	require.NoError(t, err)

	got, err := db.GetRun("run-q1")
	require.NoError(t, err)
	assert.Equal(t, "RUNNING", got.Status)
}

func TestDeleteStateViaQueue(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer db.Close()

	q := writerq.New(db.RawDB())
	defer q.Close()
	db.SetQueue(q)

	err = db.SetState("temp-q", "val")
	require.NoError(t, err)

	err = db.DeleteState("temp-q")
	require.NoError(t, err)

	_, err = db.GetState("temp-q")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestUpdateRunStatusViaQueue(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer db.Close()

	q := writerq.New(db.RawDB())
	defer q.Close()
	db.SetQueue(q)

	now := time.Now().UTC().Format(time.RFC3339)
	err = db.InsertRun(RunRecord{
		ID:        "run-q2",
		Status:    "RUNNING",
		TaskCount: 2,
		StartedAt: now,
	})
	require.NoError(t, err)

	// Update to COMPLETED — should set ended_at
	err = db.UpdateRunStatus("run-q2", "COMPLETED")
	require.NoError(t, err)

	got, err := db.GetRun("run-q2")
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", got.Status)
	assert.NotEmpty(t, got.EndedAt)

	// Insert another run and update to RUNNING — no ended_at change
	err = db.InsertRun(RunRecord{
		ID:        "run-q3",
		Status:    "PENDING",
		TaskCount: 1,
		StartedAt: now,
	})
	require.NoError(t, err)

	err = db.UpdateRunStatus("run-q3", "RUNNING")
	require.NoError(t, err)

	got2, err := db.GetRun("run-q3")
	require.NoError(t, err)
	assert.Equal(t, "RUNNING", got2.Status)
	assert.Empty(t, got2.EndedAt)
}

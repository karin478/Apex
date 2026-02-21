package writerq

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	_, _ = db.Exec("PRAGMA journal_mode=WAL")
	_, err = db.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSubmitSingleOp(t *testing.T) {
	db := openTestDB(t)
	q := New(db)
	defer q.Close()

	err := q.Submit(context.Background(), "INSERT INTO items (name) VALUES (?)", "alpha")
	require.NoError(t, err)

	// Read the row back directly.
	var name string
	err = db.QueryRow("SELECT name FROM items WHERE name = ?", "alpha").Scan(&name)
	require.NoError(t, err)
	assert.Equal(t, "alpha", name)
}

func TestSubmitConcurrent(t *testing.T) {
	db := openTestDB(t)
	q := New(db)
	defer q.Close()

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			errs[idx] = q.Submit(context.Background(), "INSERT INTO items (name) VALUES (?)", "item")
		}(i)
	}

	wg.Wait()
	// Allow any in-flight batch to commit.
	time.Sleep(100 * time.Millisecond)

	for i, err := range errs {
		require.NoError(t, err, "submit %d failed", i)
	}

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, n, count)
}

func TestSubmitContextCancelled(t *testing.T) {
	db := openTestDB(t)
	q := New(db)
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := q.Submit(ctx, "INSERT INTO items (name) VALUES (?)", "never")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestCloseFlushes(t *testing.T) {
	db := openTestDB(t)
	q := New(db)

	err := q.Submit(context.Background(), "INSERT INTO items (name) VALUES (?)", "flushed")
	require.NoError(t, err)

	// Close immediately — must flush the committed op.
	err = q.Close()
	require.NoError(t, err)

	var name string
	err = db.QueryRow("SELECT name FROM items WHERE name = ?", "flushed").Scan(&name)
	require.NoError(t, err)
	assert.Equal(t, "flushed", name)
}

func TestBackpressureDoesNotDeadlock(t *testing.T) {
	db := openTestDB(t)
	q := New(db)
	defer q.Close()

	const n = 1000
	done := make(chan struct{})

	go func() {
		defer close(done)
		var wg sync.WaitGroup
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func() {
				defer wg.Done()
				_ = q.Submit(context.Background(), "INSERT INTO items (name) VALUES (?)", "bp")
			}()
		}
		wg.Wait()
	}()

	select {
	case <-done:
		// All ops completed without deadlock.
	case <-time.After(10 * time.Second):
		t.Fatal("deadlock: 1000 ops did not complete within 10s")
	}

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, n, count)
}

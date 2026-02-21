# Phase 48: Data Reliability Foundation — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement layered file locks, DB writer queue, and action outbox WAL protocol as the data reliability foundation (architecture v11.0 §2.11 Phase 6a).

**Architecture:** Three new packages (`internal/filelock/`, `internal/writerq/`, `internal/outbox/`) built bottom-up: locks first (no deps), then writer queue (uses locks for kill switch), then outbox (uses writer queue). Integration into `cmd/apex/run.go` and `cmd/apex/doctor.go` at the end.

**Tech Stack:** Go, `syscall.Flock`, `database/sql`, `github.com/mattn/go-sqlite3`, `encoding/json`

---

### Task 1: Layered File Locks — Core Lock/Unlock

**Files:**
- Create: `internal/filelock/filelock.go`
- Create: `internal/filelock/filelock_test.go`

**Step 1: Write the failing test**

```go
package filelock

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcquireAndReleaseGlobal(t *testing.T) {
	dir := t.TempDir()
	m := NewManager()

	lock, err := m.AcquireGlobal(dir)
	require.NoError(t, err)
	assert.Equal(t, 0, lock.Order)
	assert.FileExists(t, filepath.Join(dir, "apex.lock"))

	err = m.Release(lock)
	require.NoError(t, err)
	assert.Empty(t, m.HeldLocks())
}

func TestAcquireAndReleaseWorkspace(t *testing.T) {
	dir := t.TempDir()
	m := NewManager()

	lock, err := m.AcquireWorkspace(dir)
	require.NoError(t, err)
	assert.Equal(t, 1, lock.Order)
	assert.FileExists(t, filepath.Join(dir, "ws.lock"))

	err = m.Release(lock)
	require.NoError(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/filelock/... -v -run TestAcquire`
Expected: FAIL — package does not exist

**Step 3: Write minimal implementation**

```go
package filelock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

var (
	ErrLocked        = errors.New("filelock: already locked by another process")
	ErrOrderViolation = errors.New("filelock: lock ordering violation")
)

const LockVersion = 1

type Lock struct {
	Path  string
	Order int // 0=global, 1+=workspace
	file  *os.File
}

type LockInfo struct {
	Path     string `json:"path"`
	Order    int    `json:"order"`
	PID      int    `json:"pid"`
	AcquiredAt string `json:"acquired_at"`
}

type Meta struct {
	PID       int    `json:"pid"`
	Timestamp string `json:"timestamp"`
	Order     int    `json:"lock_order_position"`
	Version   int    `json:"lock_version"`
}

type Manager struct {
	held []*Lock
	mu   sync.Mutex
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) AcquireGlobal(baseDir string) (*Lock, error) {
	return m.acquire(baseDir, "apex.lock", 0)
}

func (m *Manager) AcquireWorkspace(wsDir string) (*Lock, error) {
	m.mu.Lock()
	// Check ordering: must hold global (order=0) or nothing
	for _, h := range m.held {
		if h.Order > 0 {
			m.mu.Unlock()
			return nil, fmt.Errorf("%w: holding workspace lock %s (order=%d) while acquiring another workspace lock", ErrOrderViolation, h.Path, h.Order)
		}
	}
	m.mu.Unlock()
	return m.acquire(wsDir, "ws.lock", 1)
}

func (m *Manager) acquire(dir, filename string, order int) (*Lock, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("filelock: mkdir: %w", err)
	}

	lockPath := filepath.Join(dir, filename)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("filelock: open: %w", err)
	}

	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			pid := readMetaPID(lockPath)
			return nil, fmt.Errorf("%w (holder PID: %d)", ErrLocked, pid)
		}
		return nil, fmt.Errorf("filelock: flock: %w", err)
	}

	lock := &Lock{Path: lockPath, Order: order, file: f}

	// Write metadata
	writeMeta(lockPath, Meta{
		PID:       os.Getpid(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Order:     order,
		Version:   LockVersion,
	})

	m.mu.Lock()
	m.held = append(m.held, lock)
	m.mu.Unlock()

	return lock, nil
}

func (m *Manager) Release(lock *Lock) error {
	if lock == nil || lock.file == nil {
		return nil
	}

	err := syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN)
	lock.file.Close()

	// Remove metadata
	os.Remove(lock.Path + ".meta")

	m.mu.Lock()
	for i, h := range m.held {
		if h == lock {
			m.held = append(m.held[:i], m.held[i+1:]...)
			break
		}
	}
	m.mu.Unlock()

	return err
}

func (m *Manager) HeldLocks() []LockInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	var infos []LockInfo
	for _, h := range m.held {
		infos = append(infos, LockInfo{
			Path:  h.Path,
			Order: h.Order,
			PID:   os.Getpid(),
		})
	}
	return infos
}

func writeMeta(lockPath string, meta Meta) {
	data, _ := json.Marshal(meta)
	os.WriteFile(lockPath+".meta", data, 0644)
}

func readMetaPID(lockPath string) int {
	data, err := os.ReadFile(lockPath + ".meta")
	if err != nil {
		return 0
	}
	var meta Meta
	json.Unmarshal(data, &meta)
	return meta.PID
}

// IsStale checks if a lock's metadata indicates a dead process.
func IsStale(lockPath string) bool {
	pid := readMetaPID(lockPath)
	if pid == 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return true
	}
	// On Unix, FindProcess always succeeds; check with signal 0
	err = proc.Signal(syscall.Signal(0))
	return err != nil
}

// ReadMeta reads lock metadata from the .meta file beside a lock.
func ReadMeta(lockPath string) (Meta, error) {
	data, err := os.ReadFile(lockPath + ".meta")
	if err != nil {
		return Meta{}, err
	}
	var meta Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return Meta{}, err
	}
	return meta, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/filelock/... -v -run TestAcquire`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/filelock/filelock.go internal/filelock/filelock_test.go
git commit -m "feat(filelock): add layered file lock with flock and metadata"
```

---

### Task 2: Lock Ordering Enforcement

**Files:**
- Modify: `internal/filelock/filelock_test.go`

**Step 1: Write the failing test**

```go
func TestLockOrderingGlobalThenWorkspace(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "global")
	wsDir := filepath.Join(dir, "ws")
	m := NewManager()

	global, err := m.AcquireGlobal(globalDir)
	require.NoError(t, err)
	defer m.Release(global)

	ws, err := m.AcquireWorkspace(wsDir)
	require.NoError(t, err)
	defer m.Release(ws)
}

func TestLockOrderingWorkspaceWithoutGlobalOK(t *testing.T) {
	dir := t.TempDir()
	m := NewManager()

	ws, err := m.AcquireWorkspace(dir)
	require.NoError(t, err)
	defer m.Release(ws)
}

func TestLockOrderingTwoWorkspacesFails(t *testing.T) {
	dir := t.TempDir()
	ws1Dir := filepath.Join(dir, "ws1")
	ws2Dir := filepath.Join(dir, "ws2")
	m := NewManager()

	ws1, err := m.AcquireWorkspace(ws1Dir)
	require.NoError(t, err)
	defer m.Release(ws1)

	_, err = m.AcquireWorkspace(ws2Dir)
	assert.ErrorIs(t, err, ErrOrderViolation)
}

func TestNonBlockingReturnsErrLocked(t *testing.T) {
	dir := t.TempDir()
	m1 := NewManager()
	m2 := NewManager()

	lock, err := m1.AcquireGlobal(dir)
	require.NoError(t, err)
	defer m1.Release(lock)

	_, err = m2.AcquireGlobal(dir)
	assert.ErrorIs(t, err, ErrLocked)
}

func TestStaleLockDetection(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "apex.lock")

	// Write meta with a PID that definitely doesn't exist
	writeMeta(lockPath, Meta{
		PID:       999999999,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Order:     0,
		Version:   LockVersion,
	})

	assert.True(t, IsStale(lockPath))
}
```

**Step 2: Run test to verify it fails or passes**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/filelock/... -v -run "TestLockOrdering|TestNonBlocking|TestStale"`
Expected: All PASS (logic already in Task 1 impl)

**Step 3: No additional code needed — ordering already implemented**

**Step 4: Run full filelock tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/filelock/... -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/filelock/filelock_test.go
git commit -m "test(filelock): add lock ordering, non-blocking, and stale detection tests"
```

---

### Task 3: DB Writer Queue — Core Implementation

**Files:**
- Create: `internal/writerq/writerq.go`
- Create: `internal/writerq/writerq_test.go`

**Step 1: Write the failing test**

```go
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
	_, err = db.Exec("PRAGMA journal_mode=WAL")
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)")
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

	// Read directly — WAL allows concurrent reads
	var name string
	err = db.QueryRow("SELECT name FROM items WHERE name = ?", "alpha").Scan(&name)
	require.NoError(t, err)
	assert.Equal(t, "alpha", name)
}

func TestSubmitBatchMultipleOps(t *testing.T) {
	db := openTestDB(t)
	q := New(db)
	defer q.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			q.Submit(context.Background(), "INSERT INTO items (name) VALUES (?)", fmt.Sprintf("item-%d", n))
		}(i)
	}
	wg.Wait()

	// Allow flush
	time.Sleep(100 * time.Millisecond)

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 50, count)
}

func TestSubmitContextCancelled(t *testing.T) {
	db := openTestDB(t)
	q := New(db)
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := q.Submit(ctx, "INSERT INTO items (name) VALUES (?)", "never")
	assert.Error(t, err)
}

func TestCloseFlushes(t *testing.T) {
	db := openTestDB(t)
	q := New(db)

	q.Submit(context.Background(), "INSERT INTO items (name) VALUES (?)", "flush-test")
	q.Close()

	var name string
	err := db.QueryRow("SELECT name FROM items WHERE name = ?", "flush-test").Scan(&name)
	require.NoError(t, err)
	assert.Equal(t, "flush-test", name)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/writerq/... -v`
Expected: FAIL — package does not exist

**Step 3: Write minimal implementation**

```go
package writerq

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

const (
	DefaultQueueSize  = 1000
	DefaultFlushMs    = 50
	DefaultMaxBatch   = 100
	MaxCrashRestarts  = 3
)

type Op struct {
	SQL    string
	Args   []any
	result chan error
}

type Queue struct {
	db       *sql.DB
	ops      chan Op
	stop     chan struct{}
	done     chan struct{}
	mu       sync.Mutex
	closed   bool
	crashes  int
	killPath string // path to kill switch file, written on fatal crash
}

type Option func(*Queue)

func WithKillSwitchPath(path string) Option {
	return func(q *Queue) { q.killPath = path }
}

func New(db *sql.DB, opts ...Option) *Queue {
	q := &Queue{
		db:   db,
		ops:  make(chan Op, DefaultQueueSize),
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	for _, opt := range opts {
		opt(q)
	}
	go q.writerLoop()
	return q
}

func (q *Queue) Submit(ctx context.Context, sqlStr string, args ...any) error {
	op := Op{
		SQL:    sqlStr,
		Args:   args,
		result: make(chan error, 1),
	}

	select {
	case q.ops <- op:
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-op.result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *Queue) Close() error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return nil
	}
	q.closed = true
	q.mu.Unlock()

	close(q.stop)
	<-q.done
	return nil
}

func (q *Queue) writerLoop() {
	defer close(q.done)
	ticker := time.NewTicker(time.Duration(DefaultFlushMs) * time.Millisecond)
	defer ticker.Stop()

	var batch []Op

	flush := func() {
		if len(batch) == 0 {
			return
		}
		q.executeBatch(batch)
		batch = batch[:0]
	}

	for {
		select {
		case op := <-q.ops:
			batch = append(batch, op)
			if len(batch) >= DefaultMaxBatch {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-q.stop:
			// Drain remaining ops
			for {
				select {
				case op := <-q.ops:
					batch = append(batch, op)
				default:
					flush()
					return
				}
			}
		}
	}
}

func (q *Queue) executeBatch(batch []Op) {
	defer func() {
		if r := recover(); r != nil {
			q.crashes++
			for _, op := range batch {
				op.result <- fmt.Errorf("writerq: writer panic: %v", r)
			}
			if q.crashes < MaxCrashRestarts {
				time.Sleep(time.Second)
				go q.writerLoop()
			} else if q.killPath != "" {
				// Fatal: write kill switch
				writeKillSwitch(q.killPath)
			}
		}
	}()

	tx, err := q.db.Begin()
	if err != nil {
		for _, op := range batch {
			op.result <- fmt.Errorf("writerq: begin tx: %w", err)
		}
		return
	}

	for _, op := range batch {
		_, execErr := tx.Exec(op.SQL, op.Args...)
		if execErr != nil {
			tx.Rollback()
			// Fail the bad op, retry the rest
			op.result <- fmt.Errorf("writerq: exec: %w", execErr)
			// Re-enqueue remaining ops
			for _, remaining := range batch {
				if remaining.result != op.result {
					select {
					case remaining.result <- fmt.Errorf("writerq: batch rolled back due to: %w", execErr):
					default:
					}
				}
			}
			return
		}
	}

	if err := tx.Commit(); err != nil {
		for _, op := range batch {
			op.result <- fmt.Errorf("writerq: commit: %w", err)
		}
		return
	}

	for _, op := range batch {
		op.result <- nil
	}
}

func writeKillSwitch(path string) {
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte("writerq: fatal crash after max restarts"), 0644)
}
```

Note: The `writeKillSwitch` function needs `"os"` and `"path/filepath"` imports added.

**Step 4: Run test to verify it passes**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/writerq/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/writerq/writerq.go internal/writerq/writerq_test.go
git commit -m "feat(writerq): add single-writer DB queue with batch merging and crash recovery"
```

---

### Task 4: Action Outbox — WAL Protocol

**Files:**
- Create: `internal/outbox/outbox.go`
- Create: `internal/outbox/outbox_test.go`

**Step 1: Write the failing test**

```go
package outbox

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/lyndonlyu/apex/internal/writerq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupOutbox(t *testing.T) (*Outbox, *sql.DB) {
	t.Helper()
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
	assert.Equal(t, StatusStarted, entries[0].Status)
}

func TestCompleteWritesDBAndWAL(t *testing.T) {
	ob, db := setupOutbox(t)

	ob.Begin("act-2", "trace-2", "echo test")
	ob.RecordStarted("act-2")
	err := ob.Complete("act-2", "result-ref-1")
	require.NoError(t, err)

	// Check DB
	var status string
	err = db.QueryRow("SELECT status FROM actions WHERE action_id = ?", "act-2").Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, status)

	// Check WAL has both STARTED and COMPLETED
	entries, err := ob.ReadWAL()
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, StatusStarted, entries[0].Status)
	assert.Equal(t, StatusCompleted, entries[1].Status)
}

func TestFailWritesDBAndWAL(t *testing.T) {
	ob, db := setupOutbox(t)

	ob.Begin("act-3", "trace-3", "failing task")
	ob.RecordStarted("act-3")
	err := ob.Fail("act-3", "connection timeout")
	require.NoError(t, err)

	var status, errMsg string
	err = db.QueryRow("SELECT status, error FROM actions WHERE action_id = ?", "act-3").Scan(&status, &errMsg)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, status)
	assert.Equal(t, "connection timeout", errMsg)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/outbox/... -v`
Expected: FAIL — package does not exist

**Step 3: Write implementation**

```go
package outbox

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/lyndonlyu/apex/internal/writerq"
)

const (
	StatusStarted   = "STARTED"
	StatusCompleted = "COMPLETED"
	StatusFailed    = "FAILED"
)

type Entry struct {
	ActionID  string `json:"action_id"`
	TraceID   string `json:"trace_id"`
	Task      string `json:"task"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	ResultRef string `json:"result_ref,omitempty"`
	Error     string `json:"error,omitempty"`
}

type Outbox struct {
	walPath string
	db      *sql.DB
	writerq *writerq.Queue
	mu      sync.Mutex
}

func New(walPath string, db *sql.DB, q *writerq.Queue) (*Outbox, error) {
	// Create actions table
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS actions (
		action_id    TEXT PRIMARY KEY,
		trace_id     TEXT NOT NULL,
		task         TEXT NOT NULL,
		status       TEXT NOT NULL DEFAULT 'STARTED',
		result_ref   TEXT,
		started_at   TEXT NOT NULL,
		completed_at TEXT,
		error        TEXT,
		created_at   TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return nil, fmt.Errorf("outbox: create table: %w", err)
	}
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_actions_trace ON actions(trace_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_actions_status ON actions(status)`)

	return &Outbox{walPath: walPath, db: db, writerq: q}, nil
}

// Begin writes a STARTED entry to the WAL (Step 1).
func (o *Outbox) Begin(actionID, traceID, task string) error {
	return o.appendWAL(Entry{
		ActionID:  actionID,
		TraceID:   traceID,
		Task:      task,
		Status:    StatusStarted,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// RecordStarted inserts a STARTED record into the actions table via writerq (Step 2).
func (o *Outbox) RecordStarted(actionID string) error {
	return o.writerq.Submit(context.Background(),
		`INSERT OR IGNORE INTO actions (action_id, trace_id, task, status, started_at)
		 SELECT ?, trace_id, task, 'STARTED', ? FROM (SELECT ? as trace_id, ? as task)`,
		actionID, time.Now().UTC().Format(time.RFC3339), "", "")
}

// RecordStartedDirect inserts a STARTED record with full info (for callers who have trace/task).
func (o *Outbox) RecordStartedDirect(actionID, traceID, task string) error {
	return o.writerq.Submit(context.Background(),
		`INSERT OR IGNORE INTO actions (action_id, trace_id, task, status, started_at) VALUES (?, ?, ?, 'STARTED', ?)`,
		actionID, traceID, task, time.Now().UTC().Format(time.RFC3339))
}

// Complete updates DB to COMPLETED and appends COMPLETED to WAL (Steps 4+6).
func (o *Outbox) Complete(actionID, resultRef string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	err := o.writerq.Submit(context.Background(),
		`UPDATE actions SET status = ?, result_ref = ?, completed_at = ? WHERE action_id = ?`,
		StatusCompleted, resultRef, now, actionID)
	if err != nil {
		return fmt.Errorf("outbox: db complete: %w", err)
	}

	return o.appendWAL(Entry{
		ActionID:  actionID,
		Status:    StatusCompleted,
		ResultRef: resultRef,
		Timestamp: now,
	})
}

// Fail updates DB to FAILED and appends FAILED to WAL.
func (o *Outbox) Fail(actionID, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	err := o.writerq.Submit(context.Background(),
		`UPDATE actions SET status = ?, error = ?, completed_at = ? WHERE action_id = ?`,
		StatusFailed, errMsg, now, actionID)
	if err != nil {
		return fmt.Errorf("outbox: db fail: %w", err)
	}

	return o.appendWAL(Entry{
		ActionID:  actionID,
		Status:    StatusFailed,
		Error:     errMsg,
		Timestamp: now,
	})
}

// ReadWAL reads all entries from the WAL file.
func (o *Outbox) ReadWAL() ([]Entry, error) {
	f, err := os.Open(o.walPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}

// Reconcile finds orphan STARTED entries with no matching COMPLETED/FAILED.
func (o *Outbox) Reconcile() ([]Entry, error) {
	entries, err := o.ReadWAL()
	if err != nil {
		return nil, err
	}

	completed := make(map[string]bool)
	for _, e := range entries {
		if e.Status == StatusCompleted || e.Status == StatusFailed {
			completed[e.ActionID] = true
		}
	}

	var orphans []Entry
	for _, e := range entries {
		if e.Status == StatusStarted && !completed[e.ActionID] {
			orphans = append(orphans, e)
		}
	}
	return orphans, nil
}

func (o *Outbox) appendWAL(entry Entry) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	f, err := os.OpenFile(o.walPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("outbox: open wal: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("outbox: marshal: %w", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("outbox: write: %w", err)
	}

	return f.Sync()
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/outbox/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/outbox/outbox.go internal/outbox/outbox_test.go
git commit -m "feat(outbox): add action outbox WAL protocol with 7-step sequencing"
```

---

### Task 5: Outbox Reconciliation Tests

**Files:**
- Modify: `internal/outbox/outbox_test.go`

**Step 1: Write the failing test**

```go
func TestReconcileFindsOrphans(t *testing.T) {
	ob, _ := setupOutbox(t)

	// act-1: complete lifecycle
	ob.Begin("act-1", "trace-1", "task 1")
	ob.RecordStartedDirect("act-1", "trace-1", "task 1")
	ob.Complete("act-1", "ref-1")

	// act-2: started but never completed (orphan)
	ob.Begin("act-2", "trace-2", "task 2")
	ob.RecordStartedDirect("act-2", "trace-2", "task 2")

	// act-3: started and failed (not orphan)
	ob.Begin("act-3", "trace-3", "task 3")
	ob.RecordStartedDirect("act-3", "trace-3", "task 3")
	ob.Fail("act-3", "some error")

	orphans, err := ob.Reconcile()
	require.NoError(t, err)
	require.Len(t, orphans, 1)
	assert.Equal(t, "act-2", orphans[0].ActionID)
}

func TestReconcileEmptyWAL(t *testing.T) {
	ob, _ := setupOutbox(t)
	orphans, err := ob.Reconcile()
	require.NoError(t, err)
	assert.Empty(t, orphans)
}
```

**Step 2: Run test**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/outbox/... -v -run TestReconcile`
Expected: PASS

**Step 3: No additional implementation needed — Reconcile already written**

**Step 4: Run full outbox tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/outbox/... -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/outbox/outbox_test.go
git commit -m "test(outbox): add reconciliation tests for orphan detection"
```

---

### Task 6: Integrate Writer Queue into statedb

**Files:**
- Modify: `internal/statedb/statedb.go`
- Modify: `internal/statedb/statedb_test.go`

**Step 1: Write the failing test**

Add to `statedb_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/statedb/... -v -run TestSetStateViaQueue`
Expected: FAIL — `RawDB()` and `SetQueue()` don't exist

**Step 3: Add Queue support to statedb**

Add to `statedb.go`:

```go
// Add field to DB struct:
//   queue *writerq.Queue

// RawDB returns the underlying sql.DB for use with writerq.
func (d *DB) RawDB() *sql.DB {
	return d.db
}

// SetQueue sets an optional writer queue. When set, write operations
// (SetState, InsertRun, UpdateRunStatus, DeleteState) route through the queue.
func (d *DB) SetQueue(q *writerq.Queue) {
	d.queue = q
}
```

Modify `SetState` to route through queue when available:

```go
func (d *DB) SetState(key, value string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if d.queue != nil {
		return d.queue.Submit(context.Background(),
			`INSERT OR REPLACE INTO state (key, value, updated_at) VALUES (?, ?, ?)`,
			key, value, now)
	}
	_, err := d.db.Exec(
		`INSERT OR REPLACE INTO state (key, value, updated_at) VALUES (?, ?, ?)`,
		key, value, now,
	)
	if err != nil {
		return fmt.Errorf("statedb: set state: %w", err)
	}
	return nil
}
```

Apply the same pattern to `InsertRun`, `UpdateRunStatus`, and `DeleteState`.

**Step 4: Run test to verify it passes**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/statedb/... -v`
Expected: All PASS (existing tests use direct writes, new test uses queue)

**Step 5: Commit**

```bash
git add internal/statedb/statedb.go internal/statedb/statedb_test.go
git commit -m "feat(statedb): integrate optional writer queue for serialized writes"
```

---

### Task 7: Integrate Outbox + Locks into run.go

**Files:**
- Modify: `cmd/apex/run.go`

**Step 1: No separate test — this is integration wiring**

Add imports and wire outbox into the execution pipeline. Key changes:

1. After config load, open statedb + create writer queue
2. After governance checks, acquire global lock
3. Before pool.Execute, wrap each node with outbox.Begin/RecordStarted
4. After each node completes, call outbox.Complete or outbox.Fail
5. Release locks at end (deferred)
6. Run outbox.Reconcile at startup, log orphans

The pool currently executes nodes internally. We wrap the result callback by modifying how we log audit entries — the outbox calls happen alongside existing audit logic.

**Step 2: Implement integration**

In `runTask()`, after config and dirs setup:

```go
// Open statedb + writer queue
runtimeDir := filepath.Join(cfg.BaseDir, "runtime")
os.MkdirAll(runtimeDir, 0755)
sdb, sdbErr := statedb.Open(filepath.Join(runtimeDir, "runtime.db"))
if sdbErr != nil {
    return fmt.Errorf("statedb: %w", sdbErr)
}
defer sdb.Close()

wq := writerq.New(sdb.RawDB(), writerq.WithKillSwitchPath(killSwitchPath()))
defer wq.Close()
sdb.SetQueue(wq)

// Action outbox
walPath := filepath.Join(runtimeDir, "actions_wal.jsonl")
ob, obErr := outbox.New(walPath, sdb.RawDB(), wq)
if obErr != nil {
    fmt.Fprintf(os.Stderr, "warning: outbox init failed: %v\n", obErr)
}

// Reconcile orphan actions from previous crash
if ob != nil {
    if orphans, recErr := ob.Reconcile(); recErr == nil && len(orphans) > 0 {
        fmt.Fprintf(os.Stderr, "warning: %d orphan action(s) from previous run (NEEDS_HUMAN)\n", len(orphans))
    }
}

// Acquire global lock
lockMgr := filelock.NewManager()
globalLock, lockErr := lockMgr.AcquireGlobal(runtimeDir)
if lockErr != nil {
    return fmt.Errorf("cannot acquire lock: %w", lockErr)
}
defer lockMgr.Release(globalLock)
```

In the node audit loop (after execution), add outbox calls:

```go
// Before pool.Execute: register all nodes in outbox
if ob != nil {
    for id := range d.Nodes {
        actionID := nodeActionIDs[id]
        ob.Begin(actionID, tc.TraceID, d.Nodes[id].Task)
        ob.RecordStartedDirect(actionID, tc.TraceID, d.Nodes[id].Task)
    }
}

// After pool.Execute: complete/fail each node in outbox
if ob != nil {
    for _, n := range d.Nodes {
        actionID := nodeActionIDs[n.ID]
        if n.Status == dag.Failed {
            ob.Fail(actionID, n.Error)
        } else {
            ob.Complete(actionID, "")
        }
    }
}
```

**Step 3: Build and verify**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build ./cmd/apex/`
Expected: Build succeeds

**Step 4: Run existing E2E tests to verify no regression**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./e2e/... -v -count=1 -timeout=300s 2>&1 | tail -5`
Expected: `ok` with all tests passing

**Step 5: Commit**

```bash
git add cmd/apex/run.go
git commit -m "feat(run): integrate filelock, writerq, and outbox into execution pipeline"
```

---

### Task 8: Doctor Integration

**Files:**
- Modify: `cmd/apex/doctor.go`

**Step 1: Add lock and outbox checks to doctor**

After existing hash chain verification, add:

```go
// Lock status
fmt.Print("Lock status........ ")
runtimeDir := filepath.Join(home, ".apex", "runtime")
globalLockPath := filepath.Join(runtimeDir, "apex.lock")
if filelock.IsStale(globalLockPath) {
    meta, _ := filelock.ReadMeta(globalLockPath)
    fmt.Printf("STALE (PID %d no longer running)\n", meta.PID)
} else if meta, err := filelock.ReadMeta(globalLockPath); err == nil {
    fmt.Printf("held by PID %d since %s\n", meta.PID, meta.Timestamp)
} else {
    fmt.Println("FREE")
}

// Outbox reconciliation
fmt.Print("Action outbox...... ")
walPath := filepath.Join(runtimeDir, "actions_wal.jsonl")
if _, err := os.Stat(walPath); err == nil {
    sdb, _ := statedb.Open(filepath.Join(runtimeDir, "runtime.db"))
    if sdb != nil {
        defer sdb.Close()
        wq := writerq.New(sdb.RawDB())
        defer wq.Close()
        ob, _ := outbox.New(walPath, sdb.RawDB(), wq)
        if ob != nil {
            orphans, _ := ob.Reconcile()
            if len(orphans) == 0 {
                fmt.Println("OK (no orphan actions)")
            } else {
                fmt.Printf("WARNING: %d orphan STARTED action(s)\n", len(orphans))
                for _, o := range orphans {
                    fmt.Printf("  %s: %s\n", o.ActionID[:8], o.Task)
                }
            }
        }
    }
} else {
    fmt.Println("SKIP (no WAL file)")
}
```

**Step 2: Build and verify**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build ./cmd/apex/`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add cmd/apex/doctor.go
git commit -m "feat(doctor): add lock status and outbox health checks"
```

---

### Task 9: E2E Tests for Outbox and Locks

**Files:**
- Modify: `e2e/outbox_test.go` (create new)

**Step 1: Write E2E tests**

```go
package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutboxCreatedOnRun(t *testing.T) {
	env := newTestEnv(t)
	stdout, _ := env.runApex("run", "echo hello")

	assert.Contains(t, stdout, "Done")

	// Check WAL file exists
	walPath := filepath.Join(env.homeDir, ".apex", "runtime", "actions_wal.jsonl")
	assert.FileExists(t, walPath)

	// Check it has STARTED and COMPLETED entries
	data, err := os.ReadFile(walPath)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, `"status":"STARTED"`)
	assert.Contains(t, content, `"status":"COMPLETED"`)
}

func TestOutboxRuntimeDBActions(t *testing.T) {
	env := newTestEnv(t)
	env.runApex("run", "echo hello")

	// Check runtime.db has actions table with entries
	dbPath := filepath.Join(env.homeDir, ".apex", "runtime", "runtime.db")
	assert.FileExists(t, dbPath)
}

func TestDoctorShowsLockAndOutbox(t *testing.T) {
	env := newTestEnv(t)
	// Run a task first to create WAL
	env.runApex("run", "echo hello")

	stdout, _ := env.runApex("doctor")
	// Should show lock and outbox sections
	assert.True(t,
		strings.Contains(stdout, "Lock status") || strings.Contains(stdout, "Action outbox"),
		"doctor should show lock/outbox sections")
}
```

**Step 2: Run E2E tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./e2e/... -v -count=1 -timeout=300s -run "TestOutbox|TestDoctorShowsLock"`
Expected: PASS

**Step 3: Commit**

```bash
git add e2e/outbox_test.go
git commit -m "test(e2e): add outbox and lock integration tests"
```

---

### Task 10: Run Full Test Suite + Update PROGRESS.md

**Files:**
- Modify: `PROGRESS.md`

**Step 1: Run all unit tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/... -count=1`
Expected: All packages PASS

**Step 2: Run all E2E tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./e2e/... -v -count=1 -timeout=300s`
Expected: All PASS

**Step 3: Update PROGRESS.md**

Add Phase 48 to the completed phases table:

```markdown
| 48 | Data Reliability Foundation | `2026-02-21-phase48-data-reliability-design.md` | Done |
```

Add new packages to the Key Packages table:

```markdown
| `internal/filelock` | Layered file locks with flock, ordering enforcement, and stale detection |
| `internal/writerq` | Single-writer DB queue with 50ms batch merging, back-pressure, and crash recovery |
| `internal/outbox` | Action outbox WAL protocol with 7-step sequencing and startup reconciliation |
```

**Step 4: Commit everything**

```bash
git add PROGRESS.md
git commit -m "docs: mark Phase 48 Data Reliability Foundation as complete"
```

---

Plan complete and saved to `docs/plans/2026-02-21-phase48-data-reliability-plan.md`. Two execution options:

**1. Subagent-Driven (this session)** — I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** — Open new session with executing-plans, batch execution with checkpoints

Which approach?
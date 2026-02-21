// Package outbox implements a 7-step WAL protocol for tracking action side effects.
// It combines a write-ahead log (JSONL file) with a SQLite actions table to provide
// durable, reconcilable tracking of action lifecycle events (STARTED, COMPLETED, FAILED).
package outbox

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/lyndonlyu/apex/internal/writerq"
)

// Status constants for action lifecycle tracking.
const (
	StatusStarted   = "STARTED"
	StatusCompleted = "COMPLETED"
	StatusFailed    = "FAILED"
)

// Entry represents a single WAL entry, JSON-serializable.
type Entry struct {
	ActionID  string `json:"action_id"`
	TraceID   string `json:"trace_id"`
	Task      string `json:"task"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	ResultRef string `json:"result_ref,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Outbox tracks action side effects using a WAL file and SQLite database.
type Outbox struct {
	walPath string
	db      *sql.DB
	writerq *writerq.Queue
	mu      sync.Mutex
}

// New creates a new Outbox, initializing the actions table and indexes in the database.
func New(walPath string, db *sql.DB, q *writerq.Queue) (*Outbox, error) {
	createTable := `CREATE TABLE IF NOT EXISTS actions (
		action_id TEXT PRIMARY KEY,
		trace_id TEXT NOT NULL,
		task TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'STARTED',
		result_ref TEXT,
		started_at TEXT NOT NULL,
		completed_at TEXT,
		error TEXT,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`
	if _, err := db.Exec(createTable); err != nil {
		return nil, err
	}

	idxTrace := `CREATE INDEX IF NOT EXISTS idx_actions_trace ON actions(trace_id)`
	if _, err := db.Exec(idxTrace); err != nil {
		return nil, err
	}

	idxStatus := `CREATE INDEX IF NOT EXISTS idx_actions_status ON actions(status)`
	if _, err := db.Exec(idxStatus); err != nil {
		return nil, err
	}

	return &Outbox{
		walPath: walPath,
		db:      db,
		writerq: q,
	}, nil
}

// Begin appends a STARTED entry to the WAL file with fsync (Step 1 of the WAL protocol).
func (o *Outbox) Begin(actionID, traceID, task string) error {
	entry := Entry{
		ActionID:  actionID,
		TraceID:   traceID,
		Task:      task,
		Status:    StatusStarted,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	return o.appendWAL(entry)
}

// RecordStarted inserts a STARTED record into the actions table via writerq (Step 2).
func (o *Outbox) RecordStarted(actionID, traceID, task string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return o.writerq.Submit(
		context.Background(),
		`INSERT INTO actions (action_id, trace_id, task, status, started_at) VALUES (?, ?, ?, ?, ?)`,
		actionID, traceID, task, StatusStarted, now,
	)
}

// Complete updates the action to COMPLETED in the database via writerq and appends
// a COMPLETED entry to the WAL with fsync (Steps 4+6).
func (o *Outbox) Complete(actionID, resultRef string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if err := o.writerq.Submit(
		context.Background(),
		`UPDATE actions SET status = ?, result_ref = ?, completed_at = ? WHERE action_id = ?`,
		StatusCompleted, resultRef, now, actionID,
	); err != nil {
		return err
	}

	entry := Entry{
		ActionID:  actionID,
		Status:    StatusCompleted,
		Timestamp: now,
		ResultRef: resultRef,
	}
	return o.appendWAL(entry)
}

// Fail updates the action to FAILED in the database via writerq and appends
// a FAILED entry to the WAL.
func (o *Outbox) Fail(actionID, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if err := o.writerq.Submit(
		context.Background(),
		`UPDATE actions SET status = ?, error = ?, completed_at = ? WHERE action_id = ?`,
		StatusFailed, errMsg, now, actionID,
	); err != nil {
		return err
	}

	entry := Entry{
		ActionID:  actionID,
		Status:    StatusFailed,
		Timestamp: now,
		Error:     errMsg,
	}
	return o.appendWAL(entry)
}

// ReadWAL reads all entries from the WAL JSONL file.
func (o *Outbox) ReadWAL() ([]Entry, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

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
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// Reconcile finds orphan STARTED entries in the WAL that have no matching
// COMPLETED or FAILED entry.
func (o *Outbox) Reconcile() ([]Entry, error) {
	entries, err := o.ReadWAL()
	if err != nil {
		return nil, err
	}

	// Track which action IDs have been resolved (COMPLETED or FAILED).
	resolved := make(map[string]bool)
	for _, e := range entries {
		if e.Status == StatusCompleted || e.Status == StatusFailed {
			resolved[e.ActionID] = true
		}
	}

	// Collect orphan STARTED entries.
	var orphans []Entry
	for _, e := range entries {
		if e.Status == StatusStarted && !resolved[e.ActionID] {
			orphans = append(orphans, e)
		}
	}
	return orphans, nil
}

// appendWAL appends a JSON entry followed by a newline to the WAL file, then fsyncs.
func (o *Outbox) appendWAL(entry Entry) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	f, err := os.OpenFile(o.walPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}

	return f.Sync()
}

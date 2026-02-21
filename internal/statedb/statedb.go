package statedb

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var ErrNotFound = errors.New("statedb: not found")

type DB struct {
	db   *sql.DB
	path string
}

type StateEntry struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt string `json:"updated_at"` // RFC3339
}

type RunRecord struct {
	ID        string `json:"id"`
	Status    string `json:"status"`     // PENDING / RUNNING / COMPLETED / FAILED
	TaskCount int    `json:"task_count"`
	StartedAt string `json:"started_at"` // RFC3339
	EndedAt   string `json:"ended_at"`   // RFC3339 or empty
}

// Open creates or opens a SQLite database at path with WAL mode,
// busy timeout of 5 seconds, and foreign keys enabled. It creates
// the state and runs tables if they do not already exist.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("statedb: open: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("statedb: ping: %w", err)
	}

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("statedb: %s: %w", p, err)
		}
	}

	tables := []string{
		`CREATE TABLE IF NOT EXISTS state (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS runs (
			id         TEXT PRIMARY KEY,
			status     TEXT NOT NULL DEFAULT 'PENDING',
			task_count INTEGER NOT NULL DEFAULT 0,
			started_at TEXT NOT NULL,
			ended_at   TEXT NOT NULL DEFAULT ''
		)`,
	}
	for _, ddl := range tables {
		if _, err := db.Exec(ddl); err != nil {
			db.Close()
			return nil, fmt.Errorf("statedb: create table: %w", err)
		}
	}

	return &DB{db: db, path: path}, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// Path returns the database file path.
func (d *DB) Path() string {
	return d.path
}

// SetState upserts a key-value state entry. The updated_at timestamp
// is set to the current UTC time in RFC3339 format.
func (d *DB) SetState(key, value string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := d.db.Exec(
		`INSERT OR REPLACE INTO state (key, value, updated_at) VALUES (?, ?, ?)`,
		key, value, now,
	)
	if err != nil {
		return fmt.Errorf("statedb: set state: %w", err)
	}
	return nil
}

// GetState retrieves a state entry by key. Returns ErrNotFound if the
// key does not exist.
func (d *DB) GetState(key string) (StateEntry, error) {
	var e StateEntry
	err := d.db.QueryRow(
		`SELECT key, value, updated_at FROM state WHERE key = ?`, key,
	).Scan(&e.Key, &e.Value, &e.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return StateEntry{}, ErrNotFound
		}
		return StateEntry{}, fmt.Errorf("statedb: get state: %w", err)
	}
	return e, nil
}

// DeleteState removes a state entry by key.
func (d *DB) DeleteState(key string) error {
	_, err := d.db.Exec(`DELETE FROM state WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("statedb: delete state: %w", err)
	}
	return nil
}

// ListState returns all state entries sorted by key.
func (d *DB) ListState() ([]StateEntry, error) {
	rows, err := d.db.Query(`SELECT key, value, updated_at FROM state ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("statedb: list state: %w", err)
	}
	defer rows.Close()

	var entries []StateEntry
	for rows.Next() {
		var e StateEntry
		if err := rows.Scan(&e.Key, &e.Value, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("statedb: scan state: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("statedb: rows state: %w", err)
	}
	return entries, nil
}

// InsertRun inserts a new run record.
func (d *DB) InsertRun(record RunRecord) error {
	_, err := d.db.Exec(
		`INSERT INTO runs (id, status, task_count, started_at, ended_at) VALUES (?, ?, ?, ?, ?)`,
		record.ID, record.Status, record.TaskCount, record.StartedAt, record.EndedAt,
	)
	if err != nil {
		return fmt.Errorf("statedb: insert run: %w", err)
	}
	return nil
}

// GetRun retrieves a run record by ID. Returns ErrNotFound if the ID
// does not exist.
func (d *DB) GetRun(id string) (RunRecord, error) {
	var r RunRecord
	err := d.db.QueryRow(
		`SELECT id, status, task_count, started_at, ended_at FROM runs WHERE id = ?`, id,
	).Scan(&r.ID, &r.Status, &r.TaskCount, &r.StartedAt, &r.EndedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RunRecord{}, ErrNotFound
		}
		return RunRecord{}, fmt.Errorf("statedb: get run: %w", err)
	}
	return r, nil
}

// UpdateRunStatus updates the status of a run record. If the new status
// is COMPLETED or FAILED, ended_at is set to the current UTC time in
// RFC3339 format.
func (d *DB) UpdateRunStatus(id, status string) error {
	endedAt := ""
	if status == "COMPLETED" || status == "FAILED" {
		endedAt = time.Now().UTC().Format(time.RFC3339)
	}

	var result sql.Result
	var err error
	if endedAt != "" {
		result, err = d.db.Exec(
			`UPDATE runs SET status = ?, ended_at = ? WHERE id = ?`,
			status, endedAt, id,
		)
	} else {
		result, err = d.db.Exec(
			`UPDATE runs SET status = ? WHERE id = ?`,
			status, id,
		)
	}
	if err != nil {
		return fmt.Errorf("statedb: update run status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("statedb: rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ListRuns returns the most recent run records ordered by started_at
// descending. If limit is 0, all records are returned.
func (d *DB) ListRuns(limit int) ([]RunRecord, error) {
	query := `SELECT id, status, task_count, started_at, ended_at FROM runs ORDER BY started_at DESC`

	var rows *sql.Rows
	var err error
	if limit > 0 {
		rows, err = d.db.Query(query+" LIMIT ?", limit)
	} else {
		rows, err = d.db.Query(query)
	}
	if err != nil {
		return nil, fmt.Errorf("statedb: list runs: %w", err)
	}
	defer rows.Close()

	var records []RunRecord
	for rows.Next() {
		var r RunRecord
		if err := rows.Scan(&r.ID, &r.Status, &r.TaskCount, &r.StartedAt, &r.EndedAt); err != nil {
			return nil, fmt.Errorf("statedb: scan run: %w", err)
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("statedb: rows runs: %w", err)
	}
	return records, nil
}

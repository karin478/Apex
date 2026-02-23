// Package staging implements a memory verification pipeline with staged commit.
package staging

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lyndonlyu/apex/internal/memory"
)

// StagingEntry represents a memory in the staging pipeline.
type StagingEntry struct {
	ID           string  `json:"id"`
	Content      string  `json:"content"`
	Category     string  `json:"category"`
	Source       string  `json:"source"`
	StagingState string  `json:"staging_state"`
	Confidence   float64 `json:"confidence"`
	CreatedAt    string  `json:"created_at"`
	CommittedAt  string  `json:"committed_at,omitempty"`
	ExpiredAt    string  `json:"expired_at,omitempty"`
}

// Stager manages the memory staging pipeline.
type Stager struct {
	db    *sql.DB
	store *memory.Store
}

// New creates a Stager, initializing the staging_memories table.
func New(db *sql.DB, store *memory.Store) (*Stager, error) {
	createTable := `CREATE TABLE IF NOT EXISTS staging_memories (
		id            TEXT PRIMARY KEY,
		content       TEXT NOT NULL,
		category      TEXT NOT NULL,
		source        TEXT NOT NULL,
		staging_state TEXT NOT NULL DEFAULT 'PENDING',
		confidence    REAL NOT NULL DEFAULT 1.0,
		created_at    TEXT NOT NULL,
		committed_at  TEXT,
		expired_at    TEXT
	)`
	if _, err := db.Exec(createTable); err != nil {
		return nil, fmt.Errorf("staging: create table: %w", err)
	}
	return &Stager{db: db, store: store}, nil
}

// Stage inserts a new memory candidate into the staging pipeline.
func (s *Stager) Stage(content, category, source string) (string, error) {
	id := fmt.Sprintf("stg-%s", uuid.New().String())
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO staging_memories (id, content, category, source, staging_state, confidence, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, content, category, source, "PENDING", 1.0, now,
	)
	if err != nil {
		return "", fmt.Errorf("staging: insert: %w", err)
	}
	return id, nil
}

// Verify transitions a staging entry from PENDING to VERIFIED.
func (s *Stager) Verify(id string) error {
	result, err := s.db.Exec(
		`UPDATE staging_memories SET staging_state = 'VERIFIED' WHERE id = ? AND staging_state = 'PENDING'`,
		id,
	)
	if err != nil {
		return fmt.Errorf("staging: verify: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("staging: entry %s not found or not PENDING", id)
	}
	return nil
}

// Reject transitions a staging entry from PENDING to REJECTED.
func (s *Stager) Reject(id string) error {
	result, err := s.db.Exec(
		`UPDATE staging_memories SET staging_state = 'REJECTED' WHERE id = ? AND staging_state = 'PENDING'`,
		id,
	)
	if err != nil {
		return fmt.Errorf("staging: reject: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("staging: entry %s not found or not PENDING", id)
	}
	return nil
}

// Commit transitions a VERIFIED or UNVERIFIED entry to COMMITTED and writes to formal memory store.
func (s *Stager) Commit(id string) error {
	var content, category string
	var confidence float64
	err := s.db.QueryRow(
		`SELECT content, category, confidence FROM staging_memories WHERE id = ? AND staging_state IN ('VERIFIED', 'UNVERIFIED')`,
		id,
	).Scan(&content, &category, &confidence)
	if err != nil {
		return fmt.Errorf("staging: commit lookup: %w", err)
	}

	slug := fmt.Sprintf("staged-%s", id)
	if len(slug) > 40 {
		slug = slug[:40]
	}
	switch category {
	case "decision":
		if err := s.store.SaveDecision(slug, content); err != nil {
			return fmt.Errorf("staging: commit write: %w", err)
		}
	case "fact":
		if err := s.store.SaveFact(slug, content); err != nil {
			return fmt.Errorf("staging: commit write: %w", err)
		}
	case "session":
		if err := s.store.SaveSession(slug, "staged-commit", content); err != nil {
			return fmt.Errorf("staging: commit write: %w", err)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec(
		`UPDATE staging_memories SET staging_state = 'COMMITTED', committed_at = ? WHERE id = ?`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("staging: commit update: %w", err)
	}
	return nil
}

// CommitAll commits all VERIFIED entries and returns the count.
func (s *Stager) CommitAll() (int, error) {
	rows, err := s.db.Query(
		`SELECT id FROM staging_memories WHERE staging_state = 'VERIFIED'`,
	)
	if err != nil {
		return 0, fmt.Errorf("staging: commit all query: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}

	committed := 0
	for _, id := range ids {
		if err := s.Commit(id); err == nil {
			committed++
		}
	}
	return committed, nil
}

// ExpireStale marks PENDING entries older than timeout as EXPIRED.
func (s *Stager) ExpireStale(timeout time.Duration) (int, error) {
	threshold := time.Now().UTC().Add(-timeout).Format(time.RFC3339)
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(
		`UPDATE staging_memories SET staging_state = 'EXPIRED', expired_at = ? WHERE staging_state = 'PENDING' AND created_at < ?`,
		now, threshold,
	)
	if err != nil {
		return 0, fmt.Errorf("staging: expire: %w", err)
	}
	rows, _ := result.RowsAffected()
	return int(rows), nil
}

// ListPending returns all entries in PENDING state.
func (s *Stager) ListPending() ([]StagingEntry, error) {
	rows, err := s.db.Query(
		`SELECT id, content, category, source, staging_state, confidence, created_at FROM staging_memories WHERE staging_state = 'PENDING' ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("staging: list pending: %w", err)
	}
	defer rows.Close()

	var entries []StagingEntry
	for rows.Next() {
		var e StagingEntry
		if err := rows.Scan(&e.ID, &e.Content, &e.Category, &e.Source, &e.StagingState, &e.Confidence, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("staging: scan: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

package vectordb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	sqlite_vec.Auto()
}

// VectorDB wraps a SQLite database with sqlite-vec extension for vector
// similarity search. It maintains two tables: a metadata table (vec_meta)
// storing memory_id and text, and a virtual vec0 table (vec_memories)
// storing embeddings indexed for KNN search.
type VectorDB struct {
	db         *sql.DB
	dimensions int
}

// VectorResult represents a single result from a vector similarity search.
type VectorResult struct {
	MemoryID string
	Distance float32
	Text     string
}

// Open creates or opens a SQLite database at dbPath with sqlite-vec support.
// The dimensions parameter specifies the embedding vector size.
func Open(dbPath string, dimensions int) (*VectorDB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Force a connection to be established so the file is created.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS vec_meta (
		memory_id TEXT PRIMARY KEY,
		text TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create meta table: %w", err)
	}

	createVec := fmt.Sprintf(
		`CREATE VIRTUAL TABLE IF NOT EXISTS vec_memories USING vec0(embedding float[%d])`,
		dimensions,
	)
	_, err = db.Exec(createVec)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create vec table: %w", err)
	}

	return &VectorDB{db: db, dimensions: dimensions}, nil
}

// Close closes the underlying database connection.
func (v *VectorDB) Close() error {
	return v.db.Close()
}

// Index inserts or updates a memory entry with its embedding vector.
// If a memory with the same memoryID already exists, it is replaced.
func (v *VectorDB) Index(ctx context.Context, memoryID string, text string, embedding []float32) error {
	// Remove existing entry if present (handles upsert).
	if err := v.Delete(memoryID); err != nil {
		return fmt.Errorf("delete existing: %w", err)
	}

	_, err := v.db.ExecContext(ctx,
		`INSERT INTO vec_meta (memory_id, text, created_at) VALUES (?, ?, ?)`,
		memoryID, text, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert meta: %w", err)
	}

	var rowID int64
	err = v.db.QueryRowContext(ctx,
		`SELECT rowid FROM vec_meta WHERE memory_id = ?`, memoryID,
	).Scan(&rowID)
	if err != nil {
		return fmt.Errorf("get rowid: %w", err)
	}

	serialized, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return fmt.Errorf("serialize embedding: %w", err)
	}

	_, err = v.db.ExecContext(ctx,
		`INSERT INTO vec_memories (rowid, embedding) VALUES (?, ?)`,
		rowID, serialized,
	)
	if err != nil {
		return fmt.Errorf("insert vector: %w", err)
	}

	return nil
}

// Search performs a KNN vector similarity search, returning up to topK results
// ordered by increasing distance from the query vector.
func (v *VectorDB) Search(ctx context.Context, query []float32, topK int) ([]VectorResult, error) {
	serialized, err := sqlite_vec.SerializeFloat32(query)
	if err != nil {
		return nil, fmt.Errorf("serialize query: %w", err)
	}

	rows, err := v.db.QueryContext(ctx,
		`SELECT v.rowid, v.distance, m.memory_id, m.text
		 FROM (
		   SELECT rowid, distance
		   FROM vec_memories
		   WHERE embedding MATCH ?
		   AND k = ?
		 ) v
		 JOIN vec_meta m ON m.rowid = v.rowid
		 ORDER BY v.distance`,
		serialized, topK,
	)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()

	var results []VectorResult
	for rows.Next() {
		var rowID int64
		var r VectorResult
		if err := rows.Scan(&rowID, &r.Distance, &r.MemoryID, &r.Text); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// Delete removes a memory entry and its associated embedding by memoryID.
// Returns nil if the memoryID does not exist.
func (v *VectorDB) Delete(memoryID string) error {
	var rowID int64
	err := v.db.QueryRow(`SELECT rowid FROM vec_meta WHERE memory_id = ?`, memoryID).Scan(&rowID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("get rowid for delete: %w", err)
	}

	_, err = v.db.Exec(`DELETE FROM vec_memories WHERE rowid = ?`, rowID)
	if err != nil {
		return fmt.Errorf("delete vector: %w", err)
	}

	_, err = v.db.Exec(`DELETE FROM vec_meta WHERE memory_id = ?`, memoryID)
	if err != nil {
		return fmt.Errorf("delete meta: %w", err)
	}

	return err
}

// Count returns the number of indexed memory entries.
func (v *VectorDB) Count() (int, error) {
	var count int
	err := v.db.QueryRow(`SELECT COUNT(*) FROM vec_meta`).Scan(&count)
	return count, err
}

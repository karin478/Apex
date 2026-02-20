// Package migration provides schema migration support for SQLite databases.
// It tracks schema versions using SQLite's PRAGMA user_version, supports
// sequential migration registration, backup before migration, and plans
// for pending migrations.
package migration

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"time"
)

// Migration represents a single schema migration step.
type Migration struct {
	Version     int    `json:"version"`
	Description string `json:"description"`
	SQL         string `json:"sql"`
}

// MigrationResult describes what happened during a Migrate call.
type MigrationResult struct {
	FromVersion int    `json:"from_version"`
	ToVersion   int    `json:"to_version"`
	Applied     int    `json:"applied"`
	BackupPath  string `json:"backup_path"`
}

// Registry holds an ordered list of migrations.
type Registry struct {
	migrations []Migration
}

// NewRegistry creates an empty migration registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Add registers a migration. The version must be sequential (len(migrations)+1).
func (r *Registry) Add(version int, description, sql string) error {
	expected := len(r.migrations) + 1
	if version != expected {
		return fmt.Errorf("expected version %d, got %d", expected, version)
	}
	r.migrations = append(r.migrations, Migration{
		Version:     version,
		Description: description,
		SQL:         sql,
	})
	return nil
}

// Latest returns the highest registered migration version, or 0 if empty.
func (r *Registry) Latest() int {
	return len(r.migrations)
}

// GetVersion reads the current schema version from the database using PRAGMA user_version.
func GetVersion(db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("get user_version: %w", err)
	}
	return version, nil
}

// SetVersion sets the schema version in the database using PRAGMA user_version.
func SetVersion(db *sql.DB, version int) error {
	_, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", version))
	if err != nil {
		return fmt.Errorf("set user_version to %d: %w", version, err)
	}
	return nil
}

// Backup creates a copy of the database file at {dbPath}.bak.{unix_timestamp}.
// It returns the path of the backup file.
func Backup(dbPath string) (string, error) {
	backupPath := fmt.Sprintf("%s.bak.%d", dbPath, time.Now().Unix())

	src, err := os.Open(dbPath)
	if err != nil {
		return "", fmt.Errorf("open source db for backup: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return "", fmt.Errorf("create backup file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("copy db to backup: %w", err)
	}

	if err := dst.Sync(); err != nil {
		return "", fmt.Errorf("sync backup file: %w", err)
	}

	return backupPath, nil
}

// Migrate applies all pending migrations to the database.
//
// Behavior:
//  1. Read the current version via PRAGMA user_version.
//  2. If already at the latest version, return immediately (no backup).
//  3. Create a backup of the database file.
//  4. Execute each pending migration's SQL and update user_version after each.
//  5. Return a MigrationResult summarizing what was done.
func (r *Registry) Migrate(db *sql.DB, dbPath string) (*MigrationResult, error) {
	current, err := GetVersion(db)
	if err != nil {
		return nil, err
	}

	// Already up to date.
	if current == r.Latest() {
		return &MigrationResult{
			FromVersion: current,
			ToVersion:   current,
			Applied:     0,
		}, nil
	}

	// Backup before applying migrations.
	backupPath, err := Backup(dbPath)
	if err != nil {
		return nil, fmt.Errorf("pre-migration backup: %w", err)
	}

	applied := 0
	for _, m := range r.migrations {
		if m.Version <= current {
			continue
		}
		if _, err := db.Exec(m.SQL); err != nil {
			return nil, fmt.Errorf("migration v%d (%s): %w", m.Version, m.Description, err)
		}
		if err := SetVersion(db, m.Version); err != nil {
			return nil, fmt.Errorf("set version after migration v%d: %w", m.Version, err)
		}
		applied++
	}

	return &MigrationResult{
		FromVersion: current,
		ToVersion:   r.Latest(),
		Applied:     applied,
		BackupPath:  backupPath,
	}, nil
}

// Plan returns all migrations that have not yet been applied (Version > current).
func (r *Registry) Plan(db *sql.DB) ([]Migration, error) {
	current, err := GetVersion(db)
	if err != nil {
		return nil, err
	}

	var pending []Migration
	for _, m := range r.migrations {
		if m.Version > current {
			pending = append(pending, m)
		}
	}
	return pending, nil
}

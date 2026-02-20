package migration

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewRegistry verifies that a new registry starts empty with Latest() == 0.
func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	assert.NotNil(t, r)
	assert.Equal(t, 0, r.Latest())
}

// TestRegistryAdd adds 2 migrations and verifies Latest() == 2 and correct order.
func TestRegistryAdd(t *testing.T) {
	r := NewRegistry()

	err := r.Add(1, "create users", "CREATE TABLE users (id INTEGER PRIMARY KEY);")
	require.NoError(t, err)

	err = r.Add(2, "create posts", "CREATE TABLE posts (id INTEGER PRIMARY KEY);")
	require.NoError(t, err)

	assert.Equal(t, 2, r.Latest())

	// Verify order: first migration is version 1, second is version 2.
	assert.Equal(t, 1, r.migrations[0].Version)
	assert.Equal(t, "create users", r.migrations[0].Description)
	assert.Equal(t, 2, r.migrations[1].Version)
	assert.Equal(t, "create posts", r.migrations[1].Description)
}

// TestRegistryAddInvalid verifies that non-sequential version returns an error.
func TestRegistryAddInvalid(t *testing.T) {
	r := NewRegistry()

	// Adding version 2 first (should be 1) must fail.
	err := r.Add(2, "skip", "SELECT 1;")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected version 1")

	// Adding version 1 succeeds.
	err = r.Add(1, "first", "SELECT 1;")
	require.NoError(t, err)

	// Adding version 3 (should be 2) must fail.
	err = r.Add(3, "skip again", "SELECT 1;")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected version 2")
}

// TestGetSetVersion opens an in-memory SQLite DB and does a set/get user_version round-trip.
func TestGetSetVersion(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Default version should be 0.
	v, err := GetVersion(db)
	require.NoError(t, err)
	assert.Equal(t, 0, v)

	// Set version to 5 and read it back.
	err = SetVersion(db, 5)
	require.NoError(t, err)

	v, err = GetVersion(db)
	require.NoError(t, err)
	assert.Equal(t, 5, v)

	// Set version to 42 and read it back.
	err = SetVersion(db, 42)
	require.NoError(t, err)

	v, err = GetVersion(db)
	require.NoError(t, err)
	assert.Equal(t, 42, v)
}

// TestBackup creates a temp DB file, backs it up, and verifies the backup exists with matching content.
func TestBackup(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a real SQLite DB with some data.
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE t (id INTEGER); INSERT INTO t VALUES (1);")
	require.NoError(t, err)
	db.Close()

	// Perform backup.
	backupPath, err := Backup(dbPath)
	require.NoError(t, err)
	assert.NotEmpty(t, backupPath)

	// Backup file must exist.
	assert.FileExists(t, backupPath)

	// Backup path should follow the pattern {dbPath}.bak.{timestamp}.
	assert.True(t, strings.HasPrefix(backupPath, dbPath+".bak."),
		"backup path %q should start with %q", backupPath, dbPath+".bak.")

	// Content of backup should match the original.
	original, err := os.ReadFile(dbPath)
	require.NoError(t, err)
	backup, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	assert.Equal(t, original, backup)
}

// TestMigrate registers 2 migrations with CREATE TABLE statements, migrates from v0,
// and verifies that the tables exist and result is correct.
func TestMigrate(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Force WAL mode off so file is self-contained (optional, but helps with backup).
	_, err = db.Exec("PRAGMA journal_mode=DELETE;")
	require.NoError(t, err)

	r := NewRegistry()
	require.NoError(t, r.Add(1, "create users", "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);"))
	require.NoError(t, r.Add(2, "create posts", "CREATE TABLE posts (id INTEGER PRIMARY KEY, user_id INTEGER);"))

	result, err := r.Migrate(db, dbPath)
	require.NoError(t, err)

	// Verify result fields.
	assert.Equal(t, 0, result.FromVersion)
	assert.Equal(t, 2, result.ToVersion)
	assert.Equal(t, 2, result.Applied)
	assert.NotEmpty(t, result.BackupPath)
	assert.FileExists(t, result.BackupPath)

	// Verify tables were created by querying sqlite_master.
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='users'").Scan(&tableName)
	require.NoError(t, err)
	assert.Equal(t, "users", tableName)

	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='posts'").Scan(&tableName)
	require.NoError(t, err)
	assert.Equal(t, "posts", tableName)

	// Verify user_version was updated.
	v, err := GetVersion(db)
	require.NoError(t, err)
	assert.Equal(t, 2, v)
}

// TestMigrateAlreadyCurrent sets version to latest and verifies migrate returns Applied=0
// with no backup created.
func TestMigrateAlreadyCurrent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer db.Close()

	r := NewRegistry()
	require.NoError(t, r.Add(1, "create users", "CREATE TABLE users (id INTEGER PRIMARY KEY);"))
	require.NoError(t, r.Add(2, "create posts", "CREATE TABLE posts (id INTEGER PRIMARY KEY);"))

	// Pre-set the version to latest.
	err = SetVersion(db, r.Latest())
	require.NoError(t, err)

	result, err := r.Migrate(db, dbPath)
	require.NoError(t, err)

	assert.Equal(t, 2, result.FromVersion)
	assert.Equal(t, 2, result.ToVersion)
	assert.Equal(t, 0, result.Applied)
	assert.Empty(t, result.BackupPath, "no backup should be created when already current")

	// Count backup files in the directory: there should be none.
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	backupCount := 0
	for _, e := range entries {
		if strings.Contains(e.Name(), ".bak.") {
			backupCount++
		}
	}
	assert.Equal(t, 0, backupCount, "no backup files should exist")
}

// TestPlan verifies that Plan returns only pending migrations.
func TestPlan(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	r := NewRegistry()
	require.NoError(t, r.Add(1, "first", "SELECT 1;"))
	require.NoError(t, r.Add(2, "second", "SELECT 1;"))
	require.NoError(t, r.Add(3, "third", "SELECT 1;"))

	// At version 0, all 3 are pending.
	pending, err := r.Plan(db)
	require.NoError(t, err)
	assert.Len(t, pending, 3)

	// At version 1, only 2 and 3 are pending.
	require.NoError(t, SetVersion(db, 1))
	pending, err = r.Plan(db)
	require.NoError(t, err)
	assert.Len(t, pending, 2)
	assert.Equal(t, 2, pending[0].Version)
	assert.Equal(t, 3, pending[1].Version)

	// At version 3, nothing is pending.
	require.NoError(t, SetVersion(db, 3))
	pending, err = r.Plan(db)
	require.NoError(t, err)
	assert.Len(t, pending, 0)
}

# Phase 33: Schema Migration — Design Document

**Date:** 2026-02-20
**Status:** Approved (recommendation credit 2/6)

## Overview

Implement schema migration management (`internal/migration`) with PRAGMA user_version tracking, forward-only migration scripts, and automatic pre-migration backup. Based on Architecture v11.0 §2.11.

## Architecture

### From Architecture §2.11

- `PRAGMA user_version` controls schema version
- Migration scripts are version-numbered (forward-only + pre-migration auto-backup)
- Daemon and CLI check schema version on startup → mismatch rejects writes

### Core Types

```go
type Migration struct {
    Version     int    `json:"version"`
    Description string `json:"description"`
    SQL         string `json:"sql"`
}

type MigrationResult struct {
    FromVersion int    `json:"from_version"`
    ToVersion   int    `json:"to_version"`
    Applied     int    `json:"applied"`
    BackupPath  string `json:"backup_path"`
}

type Registry struct {
    migrations []Migration
}
```

### Operations

```go
func NewRegistry() *Registry
func (r *Registry) Add(version int, description, sql string) error
func (r *Registry) Latest() int
func GetVersion(db *sql.DB) (int, error)
func SetVersion(db *sql.DB, version int) error
func Backup(dbPath string) (string, error)
func (r *Registry) Migrate(db *sql.DB, dbPath string) (*MigrationResult, error)
func (r *Registry) Plan(db *sql.DB) ([]Migration, error)
```

- **NewRegistry**: Returns empty registry.
- **Add**: Appends migration. Version must equal len(migrations)+1 (sequential). Error if not.
- **Latest**: Returns highest version (len of migrations), 0 if empty.
- **GetVersion**: Executes `PRAGMA user_version` and returns the result.
- **SetVersion**: Executes `PRAGMA user_version = N`.
- **Backup**: Copies dbPath to `{dbPath}.bak.{unix_timestamp}`. Returns backup path.
- **Migrate**: Gets current version, if current == Latest() returns immediately. Otherwise backs up, runs each pending migration SQL in order, sets version after each. Returns MigrationResult.
- **Plan**: Returns list of pending migrations (version > current).

### CLI Command

```
apex migration status    # Show current schema version and latest available
apex migration plan      # Preview pending migrations without applying
```

## Testing

| Test | Description |
|------|-------------|
| TestNewRegistry | Empty registry, Latest() == 0 |
| TestRegistryAdd | Add migrations, verify count and Latest |
| TestRegistryAddInvalid | Non-sequential version returns error |
| TestGetSetVersion | PRAGMA user_version round-trip |
| TestBackup | Backup file created with correct content |
| TestMigrate | From v0 to v2, tables created, result correct |
| TestMigrateAlreadyCurrent | Already at latest, no migrations applied |
| E2E: TestMigrationStatus | Shows version info |
| E2E: TestMigrationPlanEmpty | No pending shows clean message |
| E2E: TestMigrationStatusRuns | Command exits cleanly |

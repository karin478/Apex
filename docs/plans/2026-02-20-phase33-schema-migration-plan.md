# Phase 33: Schema Migration — Implementation Plan

**Design:** `2026-02-20-phase33-schema-migration-design.md`

## Task 1: Migration Core — Registry + Migrate + Backup

**Files:** `internal/migration/migration.go`, `internal/migration/migration_test.go`

**Tests (7):**
1. `TestNewRegistry` — Empty registry, Latest() == 0
2. `TestRegistryAdd` — Add 2 migrations, Latest() == 2, verify order
3. `TestRegistryAddInvalid` — Non-sequential version returns error
4. `TestGetSetVersion` — Open in-memory SQLite, set/get user_version
5. `TestBackup` — Create temp DB, backup, verify backup exists and content matches
6. `TestMigrate` — Register 2 migrations (CREATE TABLE), migrate from v0, verify tables exist
7. `TestMigrateAlreadyCurrent` — Set version to latest, migrate returns Applied=0

**TDD workflow:** Write all 7 tests first, then implement.

## Task 2: Format + CLI — `apex migration status/plan`

**Files:** `internal/migration/format.go`, `internal/migration/format_test.go`, `cmd/apex/migration.go`

**Format functions:**
- `FormatStatus(current, latest int) string` — "Schema version: current/latest" + status message
- `FormatPlan(migrations []Migration) string` — table of pending migrations
- `FormatStatusJSON(current, latest int) string`

**CLI:**
- `apex migration status [--format json]` — show current vs latest version
- `apex migration plan` — list pending migrations
- Register migrationCmd in main.go

## Task 3: E2E Tests

**File:** `e2e/migration_test.go`

**Tests (3):**
1. `TestMigrationStatus` — shows version info
2. `TestMigrationPlanEmpty` — no pending shows clean message
3. `TestMigrationStatusRuns` — exits code 0

## Task 4: PROGRESS.md Update

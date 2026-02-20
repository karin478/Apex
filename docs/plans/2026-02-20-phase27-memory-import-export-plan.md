# Phase 27: Memory Import/Export — Implementation Plan

**Date:** 2026-02-20
**Design:** `2026-02-20-phase27-memory-import-export-design.md`
**Method:** Subagent-Driven Development

## Tasks

### Task 1: Memport Core (Export + Import + File IO)

**Files:** `internal/memport/memport.go`, `internal/memport/memport_test.go`
**Tests (7):**
1. `TestExportAll` — Exports all files from decisions + facts + sessions
2. `TestExportByCategory` — Filters to single category
3. `TestExportEmpty` — Empty dir returns 0 entries
4. `TestImportNew` — Imports entries into fresh directory
5. `TestImportSkip` — Existing key is skipped (count reported)
6. `TestImportOverwrite` — Existing key is overwritten (count reported)
7. `TestWriteAndReadFile` — Round-trip JSON file IO

**Spec:**
- `ExportEntry` with Key, Value, Category, CreatedAt (json tags)
- `ExportData` with Version ("1"), ExportedAt, Count, Entries (json tags)
- `MergeStrategy` typed const: skip, overwrite
- `ImportResult` with Added, Skipped, Overwritten (json tags)
- `Export(memDir, category)` walks dir, collects .md/.jsonl, optional category filter
- `Import(memDir, data, strategy)` writes entries, handles conflicts
- `WriteFile(path, data)` JSON marshal indent to file
- `ReadFile(path)` JSON unmarshal from file

### Task 2: CLI Commands

**Files:** `cmd/apex/memoryexport.go`, update `cmd/apex/main.go`
**Spec:**
- Add `memoryExportCmd` and `memoryImportCmd` as subcommands of existing `memoryCmd`
- `apex memory export [--category CAT] [--output FILE]`
- `apex memory import <file> [--strategy skip|overwrite]`
- Memory dir: `~/.claude/memory/`
- Default strategy: skip

### Task 3: E2E Tests

**Files:** `e2e/memport_test.go`
**Tests (3):**
1. `TestMemoryExportEmpty` — Empty memory dir returns valid JSON with 0 entries
2. `TestMemoryImportFile` — Import from file, verify entries created
3. `TestMemoryImportSkip` — Import twice, second time all skipped

### Task 4: PROGRESS.md Update

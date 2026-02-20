# Phase 27: Memory Import/Export — Design Document

**Date:** 2026-02-20
**Status:** Approved (recommendation credit 5/10)

## Overview

Implement memory import/export functionality (`internal/memport`) enabling backup, restore, and migration of memory entries across projects. Exports to structured JSON; imports with conflict detection and merge strategies.

## Architecture

### Memory Store Format (existing)

The memory store at `~/.claude/memory/` contains:
- `decisions/*.md` — Markdown with YAML frontmatter (type, created, slug)
- `facts/*.md` — Same format
- `sessions/*.jsonl` — JSON lines with timestamp/task/result

### Export Format

```json
{
  "version": "1",
  "exported_at": "2026-02-20T10:00:00Z",
  "count": 3,
  "entries": [
    {
      "key": "decisions/20260220-100000-my-decision.md",
      "value": "---\ntype: decision\n...",
      "category": "decisions",
      "created_at": "2026-02-20T10:00:00Z"
    }
  ]
}
```

Key = relative path within memory dir. Value = full file content. Category = subdirectory name.

### Core Types

```go
type ExportEntry struct {
    Key       string `json:"key"`
    Value     string `json:"value"`
    Category  string `json:"category"`
    CreatedAt string `json:"created_at"`
}

type ExportData struct {
    Version    string        `json:"version"`
    ExportedAt string        `json:"exported_at"`
    Count      int           `json:"count"`
    Entries    []ExportEntry `json:"entries"`
}

type MergeStrategy string

const (
    MergeSkip      MergeStrategy = "skip"
    MergeOverwrite MergeStrategy = "overwrite"
)

type ImportResult struct {
    Added     int `json:"added"`
    Skipped   int `json:"skipped"`
    Overwritten int `json:"overwritten"`
}
```

### Operations

```go
func Export(memDir string, category string) (*ExportData, error)
func Import(memDir string, data *ExportData, strategy MergeStrategy) (*ImportResult, error)
func WriteFile(path string, data *ExportData) error
func ReadFile(path string) (*ExportData, error)
```

- **Export**: Walks memDir, collects all .md and .jsonl files. If category non-empty, filters to that subdirectory only. Reads file content and mod time.
- **Import**: For each entry, checks if key (relative path) already exists. If exists: skip or overwrite based on strategy. If not exists: create file (with parent dirs).
- **WriteFile/ReadFile**: JSON marshal/unmarshal with indentation.

### CLI Commands

Extend existing `memoryCmd` (already in cmd/apex/) with two subcommands:

```
apex memory export [--category decisions|facts|sessions] [--output FILE]
apex memory import <file> [--strategy skip|overwrite]
```

Default output: stdout (for piping). With --output: writes to file.
Default strategy: skip (safe default).

## Testing

| Test | Description |
|------|-------------|
| TestExportAll | Exports all categories |
| TestExportByCategory | Filters to single category |
| TestExportEmpty | Empty dir returns empty entries |
| TestImportNew | Imports entries to fresh dir |
| TestImportSkip | Existing key skipped |
| TestImportOverwrite | Existing key overwritten |
| TestWriteAndReadFile | Round-trip file IO |
| E2E: TestMemoryExportEmpty | Empty store exports cleanly |
| E2E: TestMemoryImportFile | Import from file |
| E2E: TestMemoryImportSkip | Skip strategy in action |

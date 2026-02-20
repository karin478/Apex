# Phase 30: Memory Auto-Cleanup — Design Document

**Date:** 2026-02-20
**Status:** Approved (recommendation credit 8/10)

## Overview

Implement rule-based memory auto-cleanup (`internal/memclean`) that removes stale, low-confidence memory entries while preserving important decisions and preferences.

## Architecture

### Cleanup Rules (from Architecture v11.0)

- Trigger: memory entry count exceeds `MaxEntries * CapacityThreshold` (default 80%)
- Remove entries where: `confidence < 0.3 AND last modified > 30 days ago`
- Exempt categories: `decisions` and `preferences` (never auto-removed)

### Core Types

```go
type MemoryEntry struct {
    Path       string    `json:"path"`       // relative path in memDir
    Category   string    `json:"category"`   // subdirectory name
    Size       int64     `json:"size"`
    ModTime    time.Time `json:"mod_time"`
    Confidence float64   `json:"confidence"` // default 0.5 for files without explicit confidence
}

type CleanupConfig struct {
    CapacityThreshold float64  `json:"capacity_threshold"`  // 0.8
    MaxEntries        int      `json:"max_entries"`         // 1000
    ConfidenceMin     float64  `json:"confidence_min"`      // 0.3
    StaleAfterDays    int      `json:"stale_after_days"`    // 30
    ExemptCategories  []string `json:"exempt_categories"`   // ["decisions", "preferences"]
}

type CleanupResult struct {
    Scanned   int `json:"scanned"`
    Removed   int `json:"removed"`
    Exempted  int `json:"exempted"`
    Remaining int `json:"remaining"`
}
```

### Operations

```go
func DefaultConfig() CleanupConfig
func Scan(memDir string) ([]MemoryEntry, error)
func Evaluate(entries []MemoryEntry, cfg CleanupConfig, now time.Time) (toRemove, toKeep []MemoryEntry)
func Execute(memDir string, toRemove []MemoryEntry) (*CleanupResult, int, error)
func DryRun(memDir string, cfg CleanupConfig) (*CleanupResult, []MemoryEntry, error)
```

- **Scan**: Walks memDir, collects .md and .jsonl files with path, category, size, modtime. Confidence defaults to 0.5 (real confidence tracking is a future enhancement).
- **Evaluate**: Checks capacity threshold. If under threshold, returns nothing to remove. If over, filters entries by: not exempt category AND confidence < min AND modtime older than staleAfterDays. Takes `now` parameter for testability.
- **Execute**: Deletes files in toRemove list, returns result with counts.
- **DryRun**: Calls Scan + Evaluate, returns what would be removed without deleting.

### CLI Command

```
apex memory cleanup [--dry-run] [--max-entries N]
```

Default: executes cleanup. With --dry-run: preview only.

## Testing

| Test | Description |
|------|-------------|
| TestDefaultConfig | Verifies default values |
| TestScan | Scans temp dir with mixed files |
| TestEvaluateUnderThreshold | Under 80% capacity, nothing removed |
| TestEvaluateOverThreshold | Over 80%, stale low-confidence entries identified |
| TestEvaluateExempt | Decisions/preferences never removed |
| TestExecute | Files actually deleted from disk |
| E2E: TestMemoryCleanupDryRun | Dry run shows preview |
| E2E: TestMemoryCleanupEmpty | Empty dir shows clean message |
| E2E: TestMemoryCleanupRuns | Command exits cleanly |

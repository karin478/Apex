# Phase 30: Memory Auto-Cleanup — Implementation Plan

**Date:** 2026-02-20
**Design:** `2026-02-20-phase30-memory-auto-cleanup-design.md`
**Method:** Subagent-Driven Development

## Tasks

### Task 1: Cleanup Core (Scan + Evaluate + Execute)

**Files:** `internal/memclean/memclean.go`, `internal/memclean/memclean_test.go`
**Tests (6):**
1. `TestDefaultConfig` — Verifies default values (0.8 threshold, 1000 max, 0.3 confidence, 30 days, exempt decisions/preferences)
2. `TestScan` — Scans temp dir with .md and .jsonl files, correct category/size/modtime
3. `TestEvaluateUnderThreshold` — Under 80% capacity returns nothing to remove
4. `TestEvaluateOverThreshold` — Over 80%, stale low-confidence entries marked for removal
5. `TestEvaluateExempt` — Entries in "decisions" category never removed even if stale
6. `TestExecute` — Files deleted from disk, correct count returned

**Spec:**
- `MemoryEntry`: Path, Category, Size, ModTime, Confidence (json tags)
- `CleanupConfig`: CapacityThreshold (0.8), MaxEntries (1000), ConfidenceMin (0.3), StaleAfterDays (30), ExemptCategories (["decisions", "preferences"])
- `CleanupResult`: Scanned, Removed, Exempted, Remaining
- `DefaultConfig()` returns config with default values
- `Scan(memDir)` walks dir, collects .md/.jsonl, Category = first path component, Confidence = 0.5 default
- `Evaluate(entries, cfg, now)` — if len(entries) < cfg.MaxEntries * cfg.CapacityThreshold → nothing to remove. Otherwise: remove entries where category not in ExemptCategories AND confidence < ConfidenceMin AND modtime before now.AddDate(0,0,-StaleAfterDays)
- `Execute(memDir, toRemove)` — os.Remove each file, return CleanupResult
- `DryRun(memDir, cfg)` — Scan + Evaluate with time.Now(), return result + toRemove list

### Task 2: CLI Command

**Files:** `cmd/apex/memorycleanup.go`
**Spec:**
- Add `memoryCleanupCmd` as subcommand of existing `memoryCmd`
- `apex memory cleanup [--dry-run] [--max-entries N]`
- Memory dir: `~/.claude/memory/`
- Default: run Execute. With --dry-run: run DryRun, show what would be removed
- Register in memoryCmd via init() in the new file

### Task 3: E2E Tests

**Files:** `e2e/memclean_test.go`
**Tests (3):**
1. `TestMemoryCleanupDryRun` — Dry run exits 0, shows preview info
2. `TestMemoryCleanupEmpty` — Empty dir shows "nothing to clean"
3. `TestMemoryCleanupRuns` — Command exits cleanly

### Task 4: PROGRESS.md Update

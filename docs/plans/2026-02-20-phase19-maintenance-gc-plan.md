# Phase 19: Maintenance Subsystem (GC) — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Garbage collection for old runs, audit logs, and stale snapshots via `apex gc` command.

**Architecture:** New `internal/gc` package with `Policy`/`Result` types and `Run()` function. Scans manifest timestamps, audit file dates, and git stash list to identify deletable items.

**Tech Stack:** Go, Cobra CLI, Testify, os/filepath, time

---

## Task 1: GC Core — Policy, Run Cleanup, Audit Cleanup

**Files:**
- Create: `internal/gc/gc.go`
- Create: `internal/gc/gc_test.go`

**Implementation:** `gc.go` with `Policy`, `Result`, `DefaultPolicy()`, `Run(baseDir, policy)`.

`Run()` does:
1. Load all manifests from `{baseDir}/runs/`, sort by timestamp
2. Identify runs to delete: older than MaxAgeDays AND beyond MaxRuns limit
3. `os.RemoveAll` each run directory, track bytes freed
4. Scan `{baseDir}/audit/*.jsonl`, parse date from filename, delete files older than MaxAuditDays
5. If `policy.DryRun`, skip actual deletion but still count

**Tests (6):**
- `TestDefaultPolicy` — verify defaults (30 days, 100 runs, 90 audit days)
- `TestRunCleanupByAge` — create 3 runs, 2 old, verify 2 removed
- `TestRunCleanupByCount` — create 5 runs with MaxRuns=2, verify 3 removed
- `TestAuditCleanup` — create old + new audit files, verify old removed
- `TestDryRun` — verify nothing deleted in dry-run mode
- `TestEmptyDir` — verify graceful handling of empty/missing dirs

**Commit:** `feat(gc): add garbage collection for runs and audit logs`

---

## Task 2: CLI Command — `apex gc`

**Files:**
- Create: `cmd/apex/gc.go`
- Modify: `cmd/apex/main.go` (register gcCmd)

**Implementation:**
```go
var gcCmd = &cobra.Command{
    Use:   "gc",
    Short: "Clean up old runs, audit logs, and snapshots",
    RunE:  runGC,
}

// Flags: --dry-run, --max-age, --max-runs, --max-audit
```

**Output format:**
```
[GC] Removed 12 old runs
[GC] Removed 5 audit log files
[GC] Freed 45.2 MB
```

**Commit:** `feat(cli): add apex gc command`

---

## Task 3: E2E Tests

**Files:**
- Create: `e2e/gc_test.go`

**Tests (3):**
- `TestGCEmpty` — run gc on fresh env, exit 0, no errors
- `TestGCDryRun` — run gc --dry-run, verify no files deleted
- `TestGCAfterRun` — run a task, then gc --max-runs 0 --max-age 0, verify run cleaned

**Commit:** `test(e2e): add gc command E2E tests`

---

## Task 4: Update PROGRESS.md

Update table, test counts, key packages.

**Commit:** `docs: mark Phase 19 Maintenance GC as complete`

# Phase 19: Maintenance Subsystem (GC) — Design Document

> Date: 2026-02-20
> Status: Approved
> Architecture Ref: v11.0 §2.13 Maintenance Subsystem

## 1. Goal

Provide garbage collection for accumulated audit logs, run manifests, and stale snapshots. Prevents disk bloat and keeps the workspace manageable via `apex gc` CLI command.

## 2. Package: `internal/gc`

### 2.1 Core Types

```go
// Policy defines retention rules for garbage collection.
type Policy struct {
    MaxAgeDays   int  // delete runs older than N days (default: 30)
    MaxRuns      int  // keep at most N recent runs (default: 100)
    MaxAuditDays int  // keep audit logs for N days (default: 90)
    DryRun       bool // report without deleting
}

// Result tracks what was cleaned up.
type Result struct {
    RunsRemoved       int
    AuditFilesRemoved int
    SnapshotsRemoved  int
    BytesFreed        int64
}

func DefaultPolicy() Policy
func Run(baseDir string, policy Policy) (*Result, error)
```

### 2.2 GC Targets

| Target | Location | Rule |
|--------|----------|------|
| Old runs | `~/.apex/runs/{id}/` | Keep newest MaxRuns OR younger than MaxAgeDays |
| Audit logs | `~/.apex/audit/YYYY-MM-DD.jsonl` | Keep files younger than MaxAuditDays |
| Stale snapshots | Git stash refs `apex-snapshot-*` | Remove if associated run deleted |

### 2.3 Deletion Order

1. Identify old runs (by manifest timestamp + MaxAgeDays, capped at MaxRuns)
2. Delete run directories
3. Identify old audit files (by filename date + MaxAuditDays)
4. Delete audit files
5. Identify stale snapshots (apex-snapshot-* with no matching run)
6. Drop stale snapshots
7. Sum freed bytes

## 3. CLI: `apex gc`

```
apex gc                  # default policy
apex gc --dry-run        # preview mode
apex gc --max-age 14     # override retention age
apex gc --max-runs 50    # override max runs kept
apex gc --max-audit 60   # override audit retention
```

## 4. Non-Goals

- No daemon mode / automatic trigger
- No per-artifact-type GC
- No compression (delete only)
- No config file integration (CLI flags only for now)

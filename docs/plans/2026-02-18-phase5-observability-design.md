# Phase 5 Observability Design

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scope | MVP + TUI: Hash Chain Audit + Run Manifest + apex status + apex doctor | Practical observability without metrics export or daily anchors |
| Approach | Incremental upgrade of existing audit package | Backward compatible, minimal disruption |
| Hash algorithm | SHA-256 | Standard, fast, tamper-evident |
| Manifest storage | `~/.apex/runs/{run_id}/manifest.json` | One file per run, easy to browse |

## Architecture

### 1. Hash Chain Audit

Upgrade existing `audit.Record` with two new fields:

```
Existing: {timestamp, action_id, task, risk_level, outcome, duration_ms, model, error}
New:      {prev_hash, hash}
```

- `prev_hash`: SHA-256 hash of the previous record (empty string for chain head)
- `hash`: SHA-256 of the current record's JSON (all fields except `hash` itself)
- `Logger.Log()` maintains in-memory `lastHash`, initialized from last record in file on startup
- Backward compatible: old records without hash fields are treated as chain heads

New `Verify()` method:
- Reads all audit files chronologically
- Recomputes each record's hash and checks `prev_hash` linkage
- Returns `(valid bool, brokenAt int, err error)`

### 2. Run Manifest

New package `internal/manifest` stores execution metadata:

```json
{
  "run_id": "uuid",
  "task": "original task description",
  "timestamp": "2026-02-18T22:00:00Z",
  "model": "claude-opus-4-6",
  "effort": "high",
  "risk_level": "LOW",
  "node_count": 3,
  "duration_ms": 12500,
  "outcome": "success",
  "nodes": [
    {"id": "step-1", "task": "...", "status": "completed"},
    {"id": "step-2", "task": "...", "status": "failed", "error": "..."}
  ]
}
```

Storage: `~/.apex/runs/{run_id}/manifest.json`

Functions:
- `Save(manifest)` — write manifest to disk
- `Load(runID)` — read a single manifest
- `Recent(n)` — list most recent N runs (sorted by timestamp)

### 3. CLI Commands

**`apex status [--last N]`** (default N=5):
- Lists recent runs in table format
- Columns: RUN_ID (short 8-char), TASK (truncated), OUTCOME, DURATION, NODES, TIMESTAMP

**`apex doctor`**:
- Verifies audit hash chain integrity
- Reports: `Audit chain OK (N records verified)` or `Audit chain BROKEN at record #M`

## Deliverables

| File | Description |
|------|-------------|
| `internal/audit/logger.go` | Upgrade: add PrevHash/Hash, Verify() |
| `internal/audit/logger_test.go` | Update tests for hash chain |
| `internal/manifest/manifest.go` | Run Manifest CRUD |
| `internal/manifest/manifest_test.go` | Manifest tests |
| `cmd/apex/status.go` | `apex status` command |
| `cmd/apex/doctor.go` | `apex doctor` command |
| `cmd/apex/run.go` | Integrate manifest write after execution |
| `cmd/apex/main.go` | Register new commands |

## Error Handling

- Hash chain verification failure: report location, don't crash
- Missing manifest directory: auto-create on first run
- Corrupt manifest JSON: skip, report warning

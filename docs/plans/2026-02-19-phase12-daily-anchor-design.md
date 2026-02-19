# Phase 12: Daily Anchor + Audit Enhancement

**Date**: 2026-02-19 | **Author**: Hank & Claude | **Status**: Approved

## Background

The audit system already has a SHA-256 hash chain (`prev_hash` / `hash` fields) with `Verify()` tamper detection and `apex doctor` integration. What's missing is the **Daily Anchor** — an independent checkpoint that pins each day's chain state to a separate, harder-to-tamper store.

Architecture reference: §2.9 Observability & Audit, §2.11 Runtime State DB.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Anchor storage | anchors.jsonl + Git tag | Dual protection: file for fast local verify, git tag for independent verification |
| Trigger | Auto on `apex run` completion | "Mandatory" requires automation; manual command deferred |
| Anchor update | Replace same-day anchor if new audit entries exist | Last anchor of the day always reflects the final chain state |

## Data Model

### Anchor Record (`~/.apex/audit/anchors.jsonl`)

```json
{
  "date": "2026-02-19",
  "chain_hash": "<SHA-256 of last audit record's hash for that date>",
  "record_count": 5,
  "created_at": "2026-02-19T16:30:00Z",
  "git_tag": "apex-audit-anchor-2026-02-19"
}
```

File permissions: `0600` (owner read/write only; needs write for updates).

### Git Tag

- Name: `apex-audit-anchor-{YYYY-MM-DD}`
- Message: `Daily audit anchor: {chain_hash} ({record_count} records)`
- Lightweight annotated tag created in the current working directory
- Best-effort: creation failure does not block execution
- Updated via `git tag -f` if same-day anchor is refreshed

## Trigger Flow

```
apex run "task"
  → execute tasks + write audit entries (existing)
  → call anchor.MaybeCreate(auditDir, workDir)
    → read today's audit records from {date}.jsonl
    → if no records: skip
    → compute: chain_hash = last record's .hash field
    → count: record_count = number of records today
    → load existing anchors from anchors.jsonl
    → if today's anchor exists AND chain_hash matches: skip (no change)
    → if today's anchor exists AND chain_hash differs: update in place
    → if no anchor for today: append new anchor
    → write anchors.jsonl (atomic: write tmp + rename)
    → attempt git tag creation (best-effort)
```

## Verification (apex doctor)

Enhanced doctor output:

```
Apex Doctor
===========

Audit hash chain... OK (42 records)
Daily anchors...... OK (last: 2026-02-19, 3 anchors verified)
Git tag anchors.... OK (3/3 tags match)
```

Verification logic:

1. Load all anchors from `anchors.jsonl`
2. For each anchor:
   a. Find all audit records for that date
   b. Get the last record's `.hash`
   c. Compare with `anchor.chain_hash`
   d. Mismatch → report "MISMATCH on {date}" (potential tampering)
3. Optionally check git tags:
   a. Run `git tag -l "apex-audit-anchor-*"`
   b. For each tag, extract message and compare chain_hash
   c. Missing or mismatched tags → report "TAG MISSING/MISMATCH"

## File Changes

| File | Change | Description |
|------|--------|-------------|
| `internal/audit/anchor.go` | **New** | Anchor struct, MaybeCreate(), LoadAnchors(), VerifyAnchors(), createGitTag() |
| `internal/audit/anchor_test.go` | **New** | Unit tests for all anchor operations |
| `internal/audit/logger.go` | **Minor** | Add RecordsForDate(date) and LastHashForDate(date) helpers |
| `internal/audit/logger_test.go` | **Minor** | Tests for new helper methods |
| `cmd/apex/run.go` | **Minor** | Call anchor.MaybeCreate() after audit logging |
| `cmd/apex/doctor.go` | **Enhance** | Add anchor verification section, record count display |
| `e2e/doctor_test.go` | **Enhance** | Add anchor-related E2E tests |
| `e2e/anchor_test.go` | **New** | E2E tests: anchor creation on run, doctor anchor verify |

## Error Handling

- `anchors.jsonl` write failure → log warning to stderr, do not fail the run
- Git tag creation failure → log warning, do not fail
- Corrupted `anchors.jsonl` → `apex doctor` reports parse error, suggests re-anchor
- Missing anchors for past dates → `apex doctor` reports "NO ANCHOR for {date}"

## Testing Strategy

- **Unit tests**: Anchor CRUD, verify logic, git tag creation (mock git)
- **E2E tests**: Run creates anchor, doctor verifies anchor, corrupted anchor detected
- **Edge cases**: No audit entries today, multiple runs same day, cross-day boundary

## Out of Scope (deferred)

- OS keychain storage (Phase N+1)
- `apex anchor` manual command (Phase N+1)
- Anchor archival/rotation (part of future `apex gc`)

# Phase 22: Policy Change Audit — Design Document

> Date: 2026-02-20
> Status: Approved
> Architecture Ref: v11.0 §2.1 Governance / Policy Change Audit

## 1. Goal

Detect changes to key configuration files, automatically log audit entries, and provide `apex audit policy` to query policy change history.

## 2. Extend: `internal/audit`

### 2.1 Types

```go
type PolicyFile struct {
    Path     string `json:"path"`
    Checksum string `json:"checksum"` // SHA-256
}

type PolicyChange struct {
    File        string `json:"file"`
    OldChecksum string `json:"old_checksum"`
    NewChecksum string `json:"new_checksum"`
    Timestamp   string `json:"timestamp"`
}

type PolicyTracker struct {
    stateDir string // ~/.apex/policy-state/
}
```

### 2.2 Functions

```go
func NewPolicyTracker(stateDir string) *PolicyTracker
func (t *PolicyTracker) Check(files []string) ([]PolicyChange, error)
func (t *PolicyTracker) State() ([]PolicyFile, error)
func FormatPolicyChanges(changes []PolicyChange) string
```

- `Check()` computes SHA-256 of each file, compares with stored state, returns changes, updates state.
- `State()` loads current tracked file states from `policy-state.json`.
- `FormatPolicyChanges()` renders human-readable table.

### 2.3 State Persistence

State stored as `{stateDir}/policy-state.json`:
```json
[
  {"path": "config.yaml", "checksum": "abc123..."}
]
```

## 3. Integration with `apex run`

After config load, before execution:
1. Call `tracker.Check([]string{configPath})`
2. If changes detected, log audit entry with type "policy_change"
3. Print `[POLICY] {file} changed` to terminal

## 4. CLI: `apex audit policy`

```
apex audit policy              # list all policy changes from audit log
```

Filters audit entries by type "policy_change" and displays in table format.

## 5. Non-Goals

- No content diffing (checksum only)
- No change blocking (audit-only)
- No tracking of .claude/ directory files

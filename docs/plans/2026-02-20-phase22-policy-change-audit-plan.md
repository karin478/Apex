# Phase 22: Policy Change Audit — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Detect configuration file changes via SHA-256 checksums, log audit entries, and provide `apex audit policy` CLI to view policy change history.

**Architecture:** New `PolicyTracker` type in `internal/audit` with JSON state file for checksum persistence. Integrates with existing `Logger` for audit entries. New `apex audit policy` subcommand.

**Tech Stack:** Go, Cobra CLI, Testify, crypto/sha256, encoding/json

---

## Task 1: PolicyTracker Core — Types + Check + State

**Files:**
- Create: `internal/audit/policy.go`
- Create: `internal/audit/policy_test.go`

**Implementation:** `policy.go`

```go
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// PolicyFile represents a tracked configuration file with its checksum.
type PolicyFile struct {
	Path     string `json:"path"`
	Checksum string `json:"checksum"`
}

// PolicyChange records a detected configuration file change.
type PolicyChange struct {
	File        string `json:"file"`
	OldChecksum string `json:"old_checksum"`
	NewChecksum string `json:"new_checksum"`
	Timestamp   string `json:"timestamp"`
}

// PolicyTracker monitors configuration files for changes.
type PolicyTracker struct {
	stateDir string
}

// NewPolicyTracker creates a tracker that persists state under stateDir.
func NewPolicyTracker(stateDir string) *PolicyTracker {
	return &PolicyTracker{stateDir: stateDir}
}

func (t *PolicyTracker) statePath() string {
	return filepath.Join(t.stateDir, "policy-state.json")
}

// State loads the current tracked file states.
func (t *PolicyTracker) State() ([]PolicyFile, error) {
	data, err := os.ReadFile(t.statePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var files []PolicyFile
	if err := json.Unmarshal(data, &files); err != nil {
		return nil, err
	}
	return files, nil
}

func (t *PolicyTracker) saveState(files []PolicyFile) error {
	if err := os.MkdirAll(t.stateDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(t.statePath(), data, 0o644)
}

func fileChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

// Check compares current file checksums with stored state.
// Returns detected changes and updates the state file.
func (t *PolicyTracker) Check(files []string) ([]PolicyChange, error) {
	state, err := t.State()
	if err != nil {
		return nil, err
	}

	oldMap := make(map[string]string)
	for _, f := range state {
		oldMap[f.Path] = f.Checksum
	}

	var changes []PolicyChange
	var newState []PolicyFile

	now := time.Now().UTC().Format(time.RFC3339)

	for _, path := range files {
		checksum, err := fileChecksum(path)
		if err != nil {
			continue // skip files that can't be read
		}
		newState = append(newState, PolicyFile{Path: path, Checksum: checksum})

		old, exists := oldMap[path]
		if !exists {
			// First time tracking — record as change with empty old checksum
			changes = append(changes, PolicyChange{
				File:        path,
				OldChecksum: "",
				NewChecksum: checksum,
				Timestamp:   now,
			})
		} else if old != checksum {
			changes = append(changes, PolicyChange{
				File:        path,
				OldChecksum: old,
				NewChecksum: checksum,
				Timestamp:   now,
			})
		}
	}

	if err := t.saveState(newState); err != nil {
		return changes, err
	}

	return changes, nil
}
```

**Tests (5):**

```go
package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyTrackerNewFile(t *testing.T) {
	dir := t.TempDir()
	tracker := NewPolicyTracker(dir)

	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("model: sonnet"), 0644))

	changes, err := tracker.Check([]string{configPath})
	require.NoError(t, err)
	assert.Len(t, changes, 1)
	assert.Equal(t, configPath, changes[0].File)
	assert.Empty(t, changes[0].OldChecksum)
	assert.NotEmpty(t, changes[0].NewChecksum)
}

func TestPolicyTrackerNoChange(t *testing.T) {
	dir := t.TempDir()
	tracker := NewPolicyTracker(dir)

	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("model: sonnet"), 0644))

	// First check records initial state
	_, err := tracker.Check([]string{configPath})
	require.NoError(t, err)

	// Second check with same content — no changes
	changes, err := tracker.Check([]string{configPath})
	require.NoError(t, err)
	assert.Empty(t, changes)
}

func TestPolicyTrackerDetectsChange(t *testing.T) {
	dir := t.TempDir()
	tracker := NewPolicyTracker(dir)

	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("model: sonnet"), 0644))

	// First check
	_, err := tracker.Check([]string{configPath})
	require.NoError(t, err)

	// Modify file
	require.NoError(t, os.WriteFile(configPath, []byte("model: opus"), 0644))

	// Second check detects change
	changes, err := tracker.Check([]string{configPath})
	require.NoError(t, err)
	assert.Len(t, changes, 1)
	assert.NotEmpty(t, changes[0].OldChecksum)
	assert.NotEmpty(t, changes[0].NewChecksum)
	assert.NotEqual(t, changes[0].OldChecksum, changes[0].NewChecksum)
}

func TestPolicyTrackerState(t *testing.T) {
	dir := t.TempDir()
	tracker := NewPolicyTracker(dir)

	// Empty state initially
	state, err := tracker.State()
	require.NoError(t, err)
	assert.Nil(t, state)

	// After check, state is populated
	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("content"), 0644))
	_, err = tracker.Check([]string{configPath})
	require.NoError(t, err)

	state, err = tracker.State()
	require.NoError(t, err)
	assert.Len(t, state, 1)
	assert.Equal(t, configPath, state[0].Path)
}

func TestPolicyTrackerSkipsMissingFile(t *testing.T) {
	dir := t.TempDir()
	tracker := NewPolicyTracker(dir)

	changes, err := tracker.Check([]string{"/nonexistent/file.yaml"})
	require.NoError(t, err)
	assert.Empty(t, changes)
}
```

**Commit:** `feat(audit): add PolicyTracker for configuration file change detection`

---

## Task 2: Format Function + Audit Integration

**Files:**
- Modify: `internal/audit/policy.go` (add FormatPolicyChanges)
- Add test to: `internal/audit/policy_test.go`

**Implementation:** Add to `policy.go`:

```go
// FormatPolicyChanges renders changes as a human-readable table.
func FormatPolicyChanges(changes []PolicyChange) string {
	if len(changes) == 0 {
		return "No policy changes detected.\n"
	}

	var sb strings.Builder
	sb.WriteString("=== Policy Change History ===\n\n")
	sb.WriteString(fmt.Sprintf("%-22s | %-30s | %-12s | %-12s\n", "TIME", "FILE", "OLD", "NEW"))
	sb.WriteString(strings.Repeat("-", 82) + "\n")

	for _, c := range changes {
		old := c.OldChecksum
		if len(old) > 10 {
			old = old[:10] + "..."
		}
		if old == "" {
			old = "(new)"
		}
		newC := c.NewChecksum
		if len(newC) > 10 {
			newC = newC[:10] + "..."
		}
		file := filepath.Base(c.File)
		sb.WriteString(fmt.Sprintf("%-22s | %-30s | %-12s | %-12s\n", c.Timestamp, file, old, newC))
	}

	return sb.String()
}
```

Also needs `"fmt"`, `"strings"` imports added to policy.go.

**Test (1 additional):**

```go
func TestFormatPolicyChangesEmpty(t *testing.T) {
	out := FormatPolicyChanges(nil)
	assert.Contains(t, out, "No policy changes detected")
}

func TestFormatPolicyChangesWithData(t *testing.T) {
	changes := []PolicyChange{
		{File: "/path/to/config.yaml", OldChecksum: "aabbccddee1122334455", NewChecksum: "5544332211eeddccbbaa", Timestamp: "2026-02-20T10:00:00Z"},
	}
	out := FormatPolicyChanges(changes)
	assert.Contains(t, out, "Policy Change History")
	assert.Contains(t, out, "config.yaml")
	assert.Contains(t, out, "aabbccddee...")
}
```

**Commit:** `feat(audit): add FormatPolicyChanges function`

---

## Task 3: CLI Command — `apex audit policy`

**Files:**
- Create: `cmd/apex/auditpolicy.go`
- Modify: `cmd/apex/main.go` (add `rootCmd.AddCommand(auditPolicyCmd)`)

**Implementation:** `auditpolicy.go`

```go
package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/audit"
	"github.com/spf13/cobra"
)

var auditPolicyCmd = &cobra.Command{
	Use:   "audit-policy",
	Short: "Show policy change history",
	Long:  "List detected configuration file changes from audit logs.",
	RunE:  showAuditPolicy,
}

func showAuditPolicy(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	auditDir := filepath.Join(home, ".apex", "audit")

	logger, err := audit.NewLogger(auditDir)
	if err != nil {
		return fmt.Errorf("audit init failed: %w", err)
	}

	// Find all policy_change entries in audit log
	records, err := logger.Recent(math.MaxInt)
	if err != nil {
		return fmt.Errorf("reading audit log: %w", err)
	}

	var changes []audit.PolicyChange
	for _, r := range records {
		if r.Task == "[policy_change]" {
			changes = append(changes, audit.PolicyChange{
				File:        r.Error, // file path stored in Error field
				OldChecksum: r.Model, // old checksum stored in Model field
				NewChecksum: r.SandboxLevel, // new checksum stored in SandboxLevel field
				Timestamp:   r.Timestamp,
			})
		}
	}

	fmt.Print(audit.FormatPolicyChanges(changes))
	return nil
}
```

**IMPORTANT DESIGN NOTE:** Rather than the hacky field-packing above, we should store policy changes as proper audit entries. The better approach: use `Task: "[policy_change] config.yaml"` and store checksum info in a structured way. Let me revise:

Actually, the simplest clean approach: just log policy changes as regular audit entries with a distinctive Task prefix `[policy_change]`, and have the CLI filter by that prefix. The checksum details go in the Error field as a formatted string `old_sha→new_sha`.

Revised `auditpolicy.go`:

```go
package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/lyndonlyu/apex/internal/audit"
	"github.com/spf13/cobra"
)

var auditPolicyCmd = &cobra.Command{
	Use:   "audit-policy",
	Short: "Show policy change history",
	Long:  "List detected configuration file changes from audit logs.",
	RunE:  showAuditPolicy,
}

func showAuditPolicy(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	auditDir := filepath.Join(home, ".apex", "audit")

	logger, err := audit.NewLogger(auditDir)
	if err != nil {
		return fmt.Errorf("audit init failed: %w", err)
	}

	records, err := logger.Recent(math.MaxInt)
	if err != nil {
		return fmt.Errorf("reading audit log: %w", err)
	}

	var changes []audit.PolicyChange
	for _, r := range records {
		if strings.HasPrefix(r.Task, "[policy_change]") {
			file := strings.TrimPrefix(r.Task, "[policy_change] ")
			parts := strings.SplitN(r.Error, "→", 2)
			old, new_ := "", ""
			if len(parts) == 2 {
				old = parts[0]
				new_ = parts[1]
			}
			changes = append(changes, audit.PolicyChange{
				File:        file,
				OldChecksum: old,
				NewChecksum: new_,
				Timestamp:   r.Timestamp,
			})
		}
	}

	fmt.Print(audit.FormatPolicyChanges(changes))
	return nil
}
```

**Register in main.go:** Add `rootCmd.AddCommand(auditPolicyCmd)` after `diffCmd`.

**Commit:** `feat(cli): add apex audit-policy command`

---

## Task 4: E2E Tests

**Files:**
- Create: `e2e/auditpolicy_test.go`

**Tests (3):**

```go
package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditPolicyEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, code := env.runApex("audit-policy")
	assert.Equal(t, 0, code, "apex audit-policy should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "No policy changes detected")
}

func TestAuditPolicyAfterRun(t *testing.T) {
	env := newTestEnv(t)

	// Run a task — this triggers policy check on config.yaml
	env.runApex("run", "say hello")

	stdout, stderr, code := env.runApex("audit-policy")
	assert.Equal(t, 0, code, "apex audit-policy should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "Policy Change History")
	assert.Contains(t, stdout, "config.yaml")
}

func TestAuditPolicyDetectsConfigChange(t *testing.T) {
	env := newTestEnv(t)

	// First run — establishes baseline
	env.runApex("run", "say hello")

	// Modify config.yaml
	configPath := filepath.Join(env.Home, ".apex", "config.yaml")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, append(data, []byte("\n# modified")...), 0644))

	// Second run — detects change
	env.runApex("run", "say hello again")

	stdout, stderr, code := env.runApex("audit-policy")
	assert.Equal(t, 0, code, "apex audit-policy should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "config.yaml")
}
```

**Commit:** `test(e2e): add audit-policy E2E tests`

---

## Task 5: Update PROGRESS.md

Update Phase table, test counts, package descriptions.

**Commit:** `docs: mark Phase 22 Policy Change Audit as complete`

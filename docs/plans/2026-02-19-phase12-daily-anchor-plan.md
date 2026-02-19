# Phase 12: Daily Anchor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Daily Anchor to the audit system — an independent daily checkpoint that pins each day's hash chain to a separate file + git tag, with verification in `apex doctor`.

**Architecture:** Extend `internal/audit` with an `anchor.go` module. On each `apex run` completion, auto-generate/update anchor for today. `apex doctor` verifies anchors match the audit chain. Git tags created best-effort.

**Tech Stack:** Go, SHA-256, JSONL append-only file, `os/exec` for git tag

---

### Task 1: Logger Helper — RecordsForDate

**Files:**
- Modify: `internal/audit/logger.go`
- Test: `internal/audit/logger_test.go`

**Step 1: Write the failing test**

Add to `internal/audit/logger_test.go`:

```go
func TestRecordsForDate(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)

	// Log 3 entries (they'll go into today's file)
	for i := 0; i < 3; i++ {
		logger.Log(Entry{Task: fmt.Sprintf("task-%d", i), RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	}

	today := time.Now().Format("2006-01-02")
	records, err := logger.RecordsForDate(today)
	require.NoError(t, err)
	assert.Len(t, records, 3)
	assert.Equal(t, "task-0", records[0].Task)
	assert.Equal(t, "task-2", records[2].Task)

	// Non-existent date returns empty
	records, err = logger.RecordsForDate("1999-01-01")
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestLastHashForDate(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)

	logger.Log(Entry{Task: "task-0", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	logger.Log(Entry{Task: "task-1", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})

	today := time.Now().Format("2006-01-02")
	hash, count, err := logger.LastHashForDate(today)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Equal(t, 2, count)

	// Verify the hash matches the last record
	records, _ := logger.RecordsForDate(today)
	assert.Equal(t, records[1].Hash, hash)

	// Non-existent date
	hash, count, err = logger.LastHashForDate("1999-01-01")
	require.NoError(t, err)
	assert.Empty(t, hash)
	assert.Equal(t, 0, count)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/audit/... -run "TestRecordsForDate|TestLastHashForDate" -v`
Expected: FAIL — methods not defined

**Step 3: Write minimal implementation**

Add to `internal/audit/logger.go`:

```go
// RecordsForDate returns all audit records from a given date (YYYY-MM-DD).
func (l *Logger) RecordsForDate(date string) ([]Record, error) {
	path := filepath.Join(l.dir, date+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil, nil
	}
	lines := strings.Split(content, "\n")
	var records []Record
	for _, line := range lines {
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return nil, fmt.Errorf("parse audit record: %w", err)
		}
		records = append(records, r)
	}
	return records, nil
}

// LastHashForDate returns the hash of the last audit record for a given date,
// plus the total record count. Returns ("", 0, nil) if no records exist.
func (l *Logger) LastHashForDate(date string) (string, int, error) {
	records, err := l.RecordsForDate(date)
	if err != nil {
		return "", 0, err
	}
	if len(records) == 0 {
		return "", 0, nil
	}
	return records[len(records)-1].Hash, len(records), nil
}

// Dir returns the audit log directory path.
func (l *Logger) Dir() string {
	return l.dir
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/audit/... -run "TestRecordsForDate|TestLastHashForDate" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/audit/logger.go internal/audit/logger_test.go
git commit -m "feat(audit): add RecordsForDate, LastHashForDate, Dir helpers"
```

---

### Task 2: Anchor Data Model + Write/Load

**Files:**
- Create: `internal/audit/anchor.go`
- Create: `internal/audit/anchor_test.go`

**Step 1: Write the failing test**

Create `internal/audit/anchor_test.go`:

```go
package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnchorWriteAndLoad(t *testing.T) {
	dir := t.TempDir()

	a := Anchor{
		Date:        "2026-02-19",
		ChainHash:   "abc123",
		RecordCount: 5,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	err := WriteAnchor(dir, a)
	require.NoError(t, err)

	// File should exist
	path := filepath.Join(dir, "anchors.jsonl")
	assert.FileExists(t, path)

	// Load and verify
	anchors, err := LoadAnchors(dir)
	require.NoError(t, err)
	require.Len(t, anchors, 1)
	assert.Equal(t, "2026-02-19", anchors[0].Date)
	assert.Equal(t, "abc123", anchors[0].ChainHash)
	assert.Equal(t, 5, anchors[0].RecordCount)
}

func TestAnchorUpdateSameDay(t *testing.T) {
	dir := t.TempDir()

	// Write initial anchor
	WriteAnchor(dir, Anchor{Date: "2026-02-19", ChainHash: "hash1", RecordCount: 3, CreatedAt: "t1"})

	// Update same day with new hash
	WriteAnchor(dir, Anchor{Date: "2026-02-19", ChainHash: "hash2", RecordCount: 5, CreatedAt: "t2"})

	anchors, err := LoadAnchors(dir)
	require.NoError(t, err)
	require.Len(t, anchors, 1, "should replace, not append")
	assert.Equal(t, "hash2", anchors[0].ChainHash)
	assert.Equal(t, 5, anchors[0].RecordCount)
}

func TestAnchorMultipleDays(t *testing.T) {
	dir := t.TempDir()

	WriteAnchor(dir, Anchor{Date: "2026-02-18", ChainHash: "hash18", RecordCount: 2, CreatedAt: "t1"})
	WriteAnchor(dir, Anchor{Date: "2026-02-19", ChainHash: "hash19", RecordCount: 3, CreatedAt: "t2"})

	anchors, err := LoadAnchors(dir)
	require.NoError(t, err)
	require.Len(t, anchors, 2)
	assert.Equal(t, "2026-02-18", anchors[0].Date)
	assert.Equal(t, "2026-02-19", anchors[1].Date)
}

func TestLoadAnchorsEmpty(t *testing.T) {
	dir := t.TempDir()

	anchors, err := LoadAnchors(dir)
	require.NoError(t, err)
	assert.Empty(t, anchors)
}

func TestAnchorFilePermissions(t *testing.T) {
	dir := t.TempDir()

	WriteAnchor(dir, Anchor{Date: "2026-02-19", ChainHash: "hash", RecordCount: 1, CreatedAt: "t1"})

	path := filepath.Join(dir, "anchors.jsonl")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/audit/... -run "TestAnchor" -v`
Expected: FAIL — types and functions not defined

**Step 3: Write minimal implementation**

Create `internal/audit/anchor.go`:

```go
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Anchor represents a daily checkpoint of the audit hash chain.
type Anchor struct {
	Date        string `json:"date"`         // YYYY-MM-DD
	ChainHash   string `json:"chain_hash"`   // SHA-256 of last audit record's hash
	RecordCount int    `json:"record_count"` // number of audit records that day
	CreatedAt   string `json:"created_at"`   // RFC3339 timestamp
	GitTag      string `json:"git_tag,omitempty"`
}

const anchorsFile = "anchors.jsonl"

// LoadAnchors reads all anchors from the anchors.jsonl file.
func LoadAnchors(auditDir string) ([]Anchor, error) {
	path := filepath.Join(auditDir, anchorsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil, nil
	}
	lines := strings.Split(content, "\n")
	var anchors []Anchor
	for _, line := range lines {
		var a Anchor
		if err := json.Unmarshal([]byte(line), &a); err != nil {
			return nil, fmt.Errorf("parse anchor: %w", err)
		}
		anchors = append(anchors, a)
	}
	return anchors, nil
}

// WriteAnchor writes or updates an anchor for the given date.
// If an anchor for the same date exists, it is replaced (atomic rewrite).
// The file is created with 0600 permissions.
func WriteAnchor(auditDir string, anchor Anchor) error {
	existing, err := LoadAnchors(auditDir)
	if err != nil {
		return err
	}

	// Replace or append
	found := false
	for i, a := range existing {
		if a.Date == anchor.Date {
			existing[i] = anchor
			found = true
			break
		}
	}
	if !found {
		existing = append(existing, anchor)
	}

	// Write atomically: tmp file + rename
	path := filepath.Join(auditDir, anchorsFile)
	tmp := path + ".tmp"

	var buf strings.Builder
	for _, a := range existing {
		data, err := json.Marshal(a)
		if err != nil {
			return err
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}

	if err := os.WriteFile(tmp, []byte(buf.String()), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/audit/... -run "TestAnchor" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/audit/anchor.go internal/audit/anchor_test.go
git commit -m "feat(audit): add Anchor data model with Write/Load and atomic rewrite"
```

---

### Task 3: MaybeCreate — Auto-Anchor Logic

**Files:**
- Modify: `internal/audit/anchor.go`
- Modify: `internal/audit/anchor_test.go`

**Step 1: Write the failing test**

Add to `internal/audit/anchor_test.go`:

```go
func TestMaybeCreateAnchor(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)

	// Log some entries
	logger.Log(Entry{Task: "task-0", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	logger.Log(Entry{Task: "task-1", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})

	// Create anchor
	created, err := MaybeCreateAnchor(logger, "")
	require.NoError(t, err)
	assert.True(t, created)

	// Verify anchor was written
	anchors, _ := LoadAnchors(dir)
	require.Len(t, anchors, 1)

	today := time.Now().Format("2006-01-02")
	assert.Equal(t, today, anchors[0].Date)
	assert.Equal(t, 2, anchors[0].RecordCount)

	// Verify chain_hash matches last record
	hash, _, _ := logger.LastHashForDate(today)
	assert.Equal(t, hash, anchors[0].ChainHash)
}

func TestMaybeCreateAnchorSkipsIfUnchanged(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)

	logger.Log(Entry{Task: "task-0", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})

	// First call creates
	created, _ := MaybeCreateAnchor(logger, "")
	assert.True(t, created)

	// Second call without new entries: skip
	created, _ = MaybeCreateAnchor(logger, "")
	assert.False(t, created, "should skip when chain_hash unchanged")
}

func TestMaybeCreateAnchorUpdatesOnNewEntries(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)

	logger.Log(Entry{Task: "task-0", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	MaybeCreateAnchor(logger, "")

	// Log more entries
	logger.Log(Entry{Task: "task-1", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})

	created, _ := MaybeCreateAnchor(logger, "")
	assert.True(t, created, "should update when new entries exist")

	anchors, _ := LoadAnchors(dir)
	require.Len(t, anchors, 1, "should replace, not duplicate")
	assert.Equal(t, 2, anchors[0].RecordCount)
}

func TestMaybeCreateAnchorNoEntries(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)

	created, err := MaybeCreateAnchor(logger, "")
	require.NoError(t, err)
	assert.False(t, created, "should not create anchor when no audit entries exist")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/audit/... -run "TestMaybeCreate" -v`
Expected: FAIL — MaybeCreateAnchor not defined

**Step 3: Write minimal implementation**

Add to `internal/audit/anchor.go`:

```go
import "time"
```

(add to existing imports)

```go
// MaybeCreateAnchor checks today's audit records and creates/updates the daily anchor.
// workDir is used for git tag creation (empty string = skip git tag).
// Returns (true, nil) if an anchor was created or updated.
func MaybeCreateAnchor(logger *Logger, workDir string) (bool, error) {
	today := time.Now().Format("2006-01-02")

	hash, count, err := logger.LastHashForDate(today)
	if err != nil {
		return false, fmt.Errorf("read audit records: %w", err)
	}
	if count == 0 {
		return false, nil
	}

	// Check if anchor already exists with same hash
	existing, err := LoadAnchors(logger.Dir())
	if err != nil {
		return false, fmt.Errorf("load anchors: %w", err)
	}
	for _, a := range existing {
		if a.Date == today && a.ChainHash == hash {
			return false, nil // unchanged
		}
	}

	tagName := "apex-audit-anchor-" + today
	anchor := Anchor{
		Date:        today,
		ChainHash:   hash,
		RecordCount: count,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		GitTag:      tagName,
	}

	if err := WriteAnchor(logger.Dir(), anchor); err != nil {
		return false, fmt.Errorf("write anchor: %w", err)
	}

	// Best-effort git tag
	if workDir != "" {
		createGitTag(workDir, tagName, hash, count)
	}

	return true, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/audit/... -run "TestMaybeCreate" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/audit/anchor.go internal/audit/anchor_test.go
git commit -m "feat(audit): add MaybeCreateAnchor auto-anchor logic"
```

---

### Task 4: Git Tag Creation

**Files:**
- Modify: `internal/audit/anchor.go`
- Modify: `internal/audit/anchor_test.go`

**Step 1: Write the failing test**

Add to `internal/audit/anchor_test.go`:

```go
func TestCreateGitTag(t *testing.T) {
	// Create a temp git repo
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = dir
		out, err := c.CombinedOutput()
		require.NoError(t, err, "git cmd %v failed: %s", args, out)
	}

	// Create tag
	ok := createGitTag(dir, "apex-audit-anchor-2026-02-19", "abc123", 5)
	assert.True(t, ok)

	// Verify tag exists
	c := exec.Command("git", "tag", "-l", "apex-audit-anchor-2026-02-19")
	c.Dir = dir
	out, err := c.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "apex-audit-anchor-2026-02-19")

	// Verify tag message
	c = exec.Command("git", "tag", "-l", "-n1", "apex-audit-anchor-2026-02-19")
	c.Dir = dir
	out, _ = c.Output()
	assert.Contains(t, string(out), "abc123")
}

func TestCreateGitTagNotGitRepo(t *testing.T) {
	dir := t.TempDir() // not a git repo
	ok := createGitTag(dir, "test-tag", "hash", 1)
	assert.False(t, ok, "should return false for non-git directory")
}

func TestCreateGitTagForceUpdate(t *testing.T) {
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = dir
		c.CombinedOutput()
	}

	// Create tag twice — second should force-update
	createGitTag(dir, "apex-audit-anchor-2026-02-19", "hash1", 3)
	ok := createGitTag(dir, "apex-audit-anchor-2026-02-19", "hash2", 5)
	assert.True(t, ok)

	// Verify updated message
	c := exec.Command("git", "tag", "-l", "-n1", "apex-audit-anchor-2026-02-19")
	c.Dir = dir
	out, _ := c.Output()
	assert.Contains(t, string(out), "hash2")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/audit/... -run "TestCreateGitTag" -v`
Expected: FAIL — createGitTag not defined (or empty stub)

**Step 3: Write minimal implementation**

Add to `internal/audit/anchor.go`:

```go
import "os/exec"
```

(add to existing imports)

```go
// createGitTag creates an annotated git tag in the working directory.
// Returns true if successful, false otherwise (best-effort, never errors).
func createGitTag(workDir, tagName, chainHash string, recordCount int) bool {
	msg := fmt.Sprintf("Daily audit anchor: %s (%d records)", chainHash, recordCount)

	// Use -f to force-update if tag exists
	cmd := exec.Command("git", "tag", "-f", "-a", tagName, "-m", msg)
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/audit/... -run "TestCreateGitTag" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/audit/anchor.go internal/audit/anchor_test.go
git commit -m "feat(audit): add git tag creation for daily anchors"
```

---

### Task 5: VerifyAnchors — Anchor Integrity Check

**Files:**
- Modify: `internal/audit/anchor.go`
- Modify: `internal/audit/anchor_test.go`

**Step 1: Write the failing test**

Add to `internal/audit/anchor_test.go`:

```go
func TestVerifyAnchorsAllValid(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)

	logger.Log(Entry{Task: "task-0", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	logger.Log(Entry{Task: "task-1", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	MaybeCreateAnchor(logger, "")

	results, err := VerifyAnchors(logger)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Valid)
	assert.Equal(t, time.Now().Format("2006-01-02"), results[0].Date)
}

func TestVerifyAnchorsMismatch(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)

	logger.Log(Entry{Task: "task-0", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	MaybeCreateAnchor(logger, "")

	// Tamper with anchor
	anchors, _ := LoadAnchors(dir)
	anchors[0].ChainHash = "tampered-hash"
	WriteAnchor(dir, anchors[0])

	results, err := VerifyAnchors(logger)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.False(t, results[0].Valid)
	assert.Contains(t, results[0].Error, "mismatch")
}

func TestVerifyAnchorsNoAnchors(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)

	results, err := VerifyAnchors(logger)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestVerifyAnchorsMissingAuditDate(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)

	// Write anchor for a date with no audit records
	WriteAnchor(dir, Anchor{Date: "2020-01-01", ChainHash: "orphan", RecordCount: 5, CreatedAt: "t1"})

	results, err := VerifyAnchors(logger)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.False(t, results[0].Valid)
	assert.Contains(t, results[0].Error, "no audit records")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/audit/... -run "TestVerifyAnchors" -v`
Expected: FAIL — VerifyAnchors and AnchorResult not defined

**Step 3: Write minimal implementation**

Add to `internal/audit/anchor.go`:

```go
// AnchorResult holds the verification result for a single anchor.
type AnchorResult struct {
	Date  string
	Valid bool
	Error string // empty if valid
}

// VerifyAnchors checks all anchors against the audit chain.
func VerifyAnchors(logger *Logger) ([]AnchorResult, error) {
	anchors, err := LoadAnchors(logger.Dir())
	if err != nil {
		return nil, err
	}

	var results []AnchorResult
	for _, a := range anchors {
		hash, count, err := logger.LastHashForDate(a.Date)
		if err != nil {
			results = append(results, AnchorResult{Date: a.Date, Valid: false, Error: fmt.Sprintf("read error: %v", err)})
			continue
		}
		if count == 0 {
			results = append(results, AnchorResult{Date: a.Date, Valid: false, Error: "no audit records for this date"})
			continue
		}
		if hash != a.ChainHash {
			results = append(results, AnchorResult{Date: a.Date, Valid: false,
				Error: fmt.Sprintf("chain_hash mismatch: anchor=%s, actual=%s", a.ChainHash, hash)})
			continue
		}
		results = append(results, AnchorResult{Date: a.Date, Valid: true})
	}
	return results, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/audit/... -run "TestVerifyAnchors" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/audit/anchor.go internal/audit/anchor_test.go
git commit -m "feat(audit): add VerifyAnchors integrity check"
```

---

### Task 6: Wire Anchor into run.go

**Files:**
- Modify: `cmd/apex/run.go:322` (after audit logging block)

**Step 1: Write the code**

Add after the audit logging block (after line 322 in `run.go`), before the manifest section:

```go
	// Daily anchor — create/update after audit entries are written
	if logger != nil {
		cwd, _ := os.Getwd()
		if created, anchorErr := audit.MaybeCreateAnchor(logger, cwd); anchorErr != nil {
			fmt.Fprintf(os.Stderr, "warning: anchor creation failed: %v\n", anchorErr)
		} else if created {
			fmt.Println("Daily audit anchor updated.")
		}
	}
```

**Step 2: Verify compilation**

Run: `go build ./cmd/apex/`
Expected: success

**Step 3: Run existing E2E tests to ensure no regression**

Run: `go test ./e2e/... -v -count=1 -timeout=120s`
Expected: all 29+ tests PASS

**Step 4: Commit**

```bash
git add cmd/apex/run.go
git commit -m "feat(run): wire daily anchor creation after audit logging"
```

---

### Task 7: Enhance apex doctor with Anchor Verification

**Files:**
- Modify: `cmd/apex/doctor.go`

**Step 1: Write the implementation**

Replace `cmd/apex/doctor.go` content:

```go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lyndonlyu/apex/internal/audit"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Verify system integrity",
	RunE:  runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	auditDir := filepath.Join(home, ".apex", "audit")

	fmt.Println("Apex Doctor")
	fmt.Println("===========")
	fmt.Println()

	// 1. Hash chain verification
	fmt.Print("Audit hash chain... ")
	logger, err := audit.NewLogger(auditDir)
	if err != nil {
		fmt.Println("SKIP (no audit directory)")
		return nil
	}

	valid, brokenAt, err := logger.Verify()
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return nil
	}

	if valid {
		fmt.Println("OK")
	} else {
		fmt.Printf("BROKEN at record #%d\n", brokenAt)
		fmt.Println("  The audit log may have been tampered with.")
	}

	// 2. Daily anchor verification
	fmt.Print("Daily anchors...... ")
	results, err := audit.VerifyAnchors(logger)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else if len(results) == 0 {
		fmt.Println("SKIP (no anchors yet)")
	} else {
		allValid := true
		for _, r := range results {
			if !r.Valid {
				allValid = false
				break
			}
		}
		lastDate := results[len(results)-1].Date
		if allValid {
			fmt.Printf("OK (last: %s, %d anchors verified)\n", lastDate, len(results))
		} else {
			fmt.Printf("ISSUES FOUND (%d anchors)\n", len(results))
		}
		for _, r := range results {
			if r.Valid {
				fmt.Printf("  %s: OK\n", r.Date)
			} else {
				fmt.Printf("  %s: MISMATCH — %s\n", r.Date, r.Error)
			}
		}
	}

	// 3. Git tag anchor verification (best-effort)
	fmt.Print("Git tag anchors.... ")
	anchors, _ := audit.LoadAnchors(auditDir)
	if len(anchors) == 0 {
		fmt.Println("SKIP (no anchors)")
	} else {
		cwd, _ := os.Getwd()
		tagOut, tagErr := exec.Command("git", "-C", cwd, "tag", "-l", "apex-audit-anchor-*").Output()
		if tagErr != nil {
			fmt.Println("SKIP (not a git repo)")
		} else {
			tags := strings.Split(strings.TrimSpace(string(tagOut)), "\n")
			tagSet := make(map[string]bool)
			for _, t := range tags {
				tagSet[strings.TrimSpace(t)] = true
			}
			found := 0
			for _, a := range anchors {
				if tagSet[a.GitTag] {
					found++
				}
			}
			if found == len(anchors) {
				fmt.Printf("OK (%d/%d tags found)\n", found, len(anchors))
			} else {
				fmt.Printf("PARTIAL (%d/%d tags found)\n", found, len(anchors))
			}
		}
	}

	return nil
}
```

**Step 2: Verify compilation**

Run: `go build ./cmd/apex/`
Expected: success

**Step 3: Commit**

```bash
git add cmd/apex/doctor.go
git commit -m "feat(doctor): add daily anchor and git tag verification"
```

---

### Task 8: E2E Tests for Anchor

**Files:**
- Create: `e2e/anchor_test.go`
- Modify: `e2e/doctor_test.go`

**Step 1: Write E2E tests**

Create `e2e/anchor_test.go`:

```go
package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnchorCreatedOnRun verifies that "apex run" creates a daily anchor.
func TestAnchorCreatedOnRun(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, code := env.runApex("run", "say hello")
	require.Equal(t, 0, code, "apex run should succeed; stdout=%s stderr=%s", stdout, stderr)

	// Check anchors.jsonl exists
	anchorPath := filepath.Join(env.auditDir(), "anchors.jsonl")
	require.True(t, env.fileExists(anchorPath), "anchors.jsonl should exist after run")

	// Parse and validate
	data, err := os.ReadFile(anchorPath)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 1, "should have exactly 1 anchor")

	var anchor struct {
		Date        string `json:"date"`
		ChainHash   string `json:"chain_hash"`
		RecordCount int    `json:"record_count"`
		GitTag      string `json:"git_tag"`
	}
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &anchor))

	assert.NotEmpty(t, anchor.Date)
	assert.NotEmpty(t, anchor.ChainHash)
	assert.Greater(t, anchor.RecordCount, 0)
	assert.Contains(t, anchor.GitTag, "apex-audit-anchor-")
}

// TestAnchorUpdatedOnSecondRun verifies that a second run updates the anchor.
func TestAnchorUpdatedOnSecondRun(t *testing.T) {
	env := newTestEnv(t)

	// First run
	env.runApex("run", "say hello")

	// Second run
	env.runApex("run", "say goodbye")

	// Should still have 1 anchor (updated, not duplicated)
	anchorPath := filepath.Join(env.auditDir(), "anchors.jsonl")
	data, err := os.ReadFile(anchorPath)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 1, "should have 1 anchor, not 2")

	var anchor struct {
		RecordCount int `json:"record_count"`
	}
	json.Unmarshal([]byte(lines[0]), &anchor)
	assert.Equal(t, 2, anchor.RecordCount, "anchor should reflect both runs")
}

// TestAnchorGitTag verifies that a git tag is created in the working directory.
func TestAnchorGitTag(t *testing.T) {
	env := newTestEnv(t)

	env.runApex("run", "say hello")

	// Check git tag exists in WorkDir
	stdout, _, code := env.runApexWithEnv(nil, "doctor")
	require.Equal(t, 0, code)
	assert.Contains(t, stdout, "Git tag anchors")
}
```

Add to `e2e/doctor_test.go`:

```go
// TestDoctorAnchorVerification verifies that doctor checks anchor integrity.
func TestDoctorAnchorVerification(t *testing.T) {
	env := newTestEnv(t)

	// Run a task to create audit + anchor
	env.runApex("run", "say hello")

	// Doctor should report anchor OK
	stdout, _, code := env.runApex("doctor")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "Daily anchors")
	assert.Contains(t, stdout, "OK")
}
```

**Step 2: Run tests to verify they pass**

Run: `go test ./e2e/... -v -count=1 -timeout=120s`
Expected: all tests PASS (including new ones)

**Step 3: Commit**

```bash
git add e2e/anchor_test.go e2e/doctor_test.go
git commit -m "test(e2e): add anchor creation and doctor anchor verification tests"
```

---

### Task 9: Update PROGRESS.md

**Files:**
- Modify: `PROGRESS.md`

**Step 1: Update progress**

Add Phase 12 to the completed phases table and update Current section:

- Add row: `| 12 | Daily Anchor | \`2026-02-19-phase12-daily-anchor-design.md\` | Done |`
- Update "Current: Phase 13 — TBD"
- Update E2E test count
- Add note about anchor package in Key Packages

**Step 2: Commit**

```bash
git add PROGRESS.md
git commit -m "docs: mark Phase 12 Daily Anchor as complete"
```

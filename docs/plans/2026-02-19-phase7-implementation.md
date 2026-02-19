# Phase 7: Snapshot & Rollback Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add git-stash-based snapshots before DAG execution with manual restore on failure.

**Architecture:** New `internal/snapshot` package wrapping git stash commands. Integration into `run.go` creates a snapshot before execution and drops it on success or prompts restore on failure. Two new CLI subcommands under `apex snapshot`.

**Tech Stack:** Go stdlib (`os/exec`, `strings`, `time`), git CLI, Cobra, Testify

---

### Task 1: Snapshot Manager Package

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/snapshot/snapshot.go`
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/snapshot/snapshot_test.go`

**Step 1: Write the failing tests**

```go
package snapshot

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initGitRepo creates a git repo with one committed file for testing.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v failed: %s", args, out)
	}
	run("init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("original"), 0644))
	run("add", ".")
	run("commit", "-m", "initial")
	return dir
}

func TestCreateSnapshot(t *testing.T) {
	dir := initGitRepo(t)
	m := New(dir)

	// Modify a file so there's something to stash
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified"), 0644)

	snap, err := m.Create("test-run-001")
	require.NoError(t, err)
	assert.Contains(t, snap.Message, "apex-test-run-001")
	assert.Equal(t, "test-run-001", snap.RunID)

	// Verify file was restored to original (stash saves and reverts)
	data, _ := os.ReadFile(filepath.Join(dir, "file.txt"))
	assert.Equal(t, "original", string(data))
}

func TestCreateWithNoChanges(t *testing.T) {
	dir := initGitRepo(t)
	m := New(dir)

	// No changes — Create should return nil snapshot, nil error
	snap, err := m.Create("test-run-002")
	assert.NoError(t, err)
	assert.Nil(t, snap)
}

func TestRestoreSnapshot(t *testing.T) {
	dir := initGitRepo(t)
	m := New(dir)

	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("before-run"), 0644)
	_, err := m.Create("test-run-003")
	require.NoError(t, err)

	// Simulate post-execution changes
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("after-run-failed"), 0644)

	// Restore should bring back pre-execution state
	err = m.Restore("test-run-003")
	require.NoError(t, err)

	data, _ := os.ReadFile(filepath.Join(dir, "file.txt"))
	assert.Equal(t, "before-run", string(data))
}

func TestListSnapshots(t *testing.T) {
	dir := initGitRepo(t)
	m := New(dir)

	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("v1"), 0644)
	m.Create("run-a")

	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("v2"), 0644)
	m.Create("run-b")

	snaps, err := m.List()
	require.NoError(t, err)
	assert.Len(t, snaps, 2)
}

func TestDropSnapshot(t *testing.T) {
	dir := initGitRepo(t)
	m := New(dir)

	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("v1"), 0644)
	m.Create("run-drop")

	err := m.Drop("run-drop")
	require.NoError(t, err)

	snaps, _ := m.List()
	assert.Empty(t, snaps)
}

func TestCreateInNonGitDir(t *testing.T) {
	dir := t.TempDir() // no git init
	m := New(dir)

	_, err := m.Create("test-run")
	assert.Error(t, err)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/snapshot/ -v`
Expected: FAIL — package does not exist

**Step 3: Implement snapshot.go**

```go
package snapshot

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const messagePrefix = "apex-"

// Snapshot represents a saved working tree state.
type Snapshot struct {
	Index   int
	Message string
	RunID   string
}

// Manager handles git stash operations for snapshots.
type Manager struct {
	workDir string
}

// New creates a Manager for the given working directory.
func New(workDir string) *Manager {
	return &Manager{workDir: workDir}
}

func (m *Manager) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = m.workDir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Create stashes current working tree changes with an apex-prefixed message.
// Returns nil snapshot and nil error if there are no changes to stash.
func (m *Manager) Create(runID string) (*Snapshot, error) {
	msg := fmt.Sprintf("%s%s-%s", messagePrefix, runID, time.Now().UTC().Format("20060102T150405Z"))

	out, err := m.git("stash", "push", "--include-untracked", "-m", msg)
	if err != nil {
		return nil, fmt.Errorf("git stash push failed: %w: %s", err, out)
	}

	// git stash push prints "No local changes to save" when clean
	if strings.Contains(out, "No local changes") {
		return nil, nil
	}

	return &Snapshot{
		Index:   0,
		Message: msg,
		RunID:   runID,
	}, nil
}

// Restore pops the stash matching the given runID.
func (m *Manager) Restore(runID string) error {
	idx, err := m.findStash(runID)
	if err != nil {
		return err
	}
	out, err := m.git("stash", "pop", fmt.Sprintf("stash@{%d}", idx))
	if err != nil {
		return fmt.Errorf("git stash pop failed: %w: %s", err, out)
	}
	return nil
}

// List returns all apex-prefixed stash entries.
func (m *Manager) List() ([]Snapshot, error) {
	out, err := m.git("stash", "list")
	if err != nil {
		return nil, fmt.Errorf("git stash list failed: %w", err)
	}
	if out == "" {
		return nil, nil
	}

	var snaps []Snapshot
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, messagePrefix) {
			continue
		}
		// Format: stash@{N}: On branch: message
		idx := parseStashIndex(line)
		msg := parseStashMessage(line)
		runID := parseRunID(msg)
		snaps = append(snaps, Snapshot{
			Index:   idx,
			Message: msg,
			RunID:   runID,
		})
	}
	return snaps, nil
}

// Drop removes the stash matching the given runID.
func (m *Manager) Drop(runID string) error {
	idx, err := m.findStash(runID)
	if err != nil {
		return err
	}
	out, err := m.git("stash", "drop", fmt.Sprintf("stash@{%d}", idx))
	if err != nil {
		return fmt.Errorf("git stash drop failed: %w: %s", err, out)
	}
	return nil
}

func (m *Manager) findStash(runID string) (int, error) {
	snaps, err := m.List()
	if err != nil {
		return -1, err
	}
	prefix := messagePrefix + runID
	for _, s := range snaps {
		if strings.HasPrefix(s.Message, prefix) {
			return s.Index, nil
		}
	}
	return -1, fmt.Errorf("no snapshot found for run %s", runID)
}

func parseStashIndex(line string) int {
	// stash@{0}: ...
	start := strings.Index(line, "{")
	end := strings.Index(line, "}")
	if start == -1 || end == -1 {
		return 0
	}
	var idx int
	fmt.Sscanf(line[start+1:end], "%d", &idx)
	return idx
}

func parseStashMessage(line string) string {
	// stash@{N}: On branch: message  OR  stash@{N}: WIP on branch: message
	idx := strings.Index(line, messagePrefix)
	if idx == -1 {
		return ""
	}
	return strings.TrimSpace(line[idx:])
}

func parseRunID(msg string) string {
	// apex-{runID}-{timestamp}
	if !strings.HasPrefix(msg, messagePrefix) {
		return ""
	}
	rest := msg[len(messagePrefix):] // runID-timestamp
	// Find the last dash before the timestamp (format: 20060102T150405Z)
	lastDash := strings.LastIndex(rest, "-")
	if lastDash == -1 {
		return rest
	}
	return rest[:lastDash]
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/snapshot/ -v`
Expected: All 6 tests PASS

**Step 5: Commit**

```bash
git add internal/snapshot/snapshot.go internal/snapshot/snapshot_test.go
git commit -m "feat(snapshot): add git-stash-based snapshot manager for rollback support"
```

---

### Task 2: CLI Commands (snapshot list + restore)

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/snapshot.go`
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/main.go` — add `rootCmd.AddCommand(snapshotCmd)`

**Step 1: Implement snapshot.go CLI**

```go
package main

import (
	"fmt"
	"os"

	"github.com/lyndonlyu/apex/internal/snapshot"
	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Manage execution snapshots",
}

var snapshotListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all apex snapshots",
	RunE:  listSnapshots,
}

var snapshotRestoreCmd = &cobra.Command{
	Use:   "restore [run-id]",
	Short: "Restore working tree from a snapshot",
	Args:  cobra.ExactArgs(1),
	RunE:  restoreSnapshot,
}

func init() {
	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotCmd.AddCommand(snapshotRestoreCmd)
}

func listSnapshots(cmd *cobra.Command, args []string) error {
	cwd, _ := os.Getwd()
	m := snapshot.New(cwd)

	snaps, err := m.List()
	if err != nil {
		return fmt.Errorf("failed to list snapshots: %w", err)
	}

	if len(snaps) == 0 {
		fmt.Println("No snapshots found.")
		return nil
	}

	fmt.Printf("%-12s %-50s\n", "RUN_ID", "MESSAGE")
	fmt.Println("------------ --------------------------------------------------")
	for _, s := range snaps {
		runID := s.RunID
		if len(runID) > 12 {
			runID = runID[:12]
		}
		fmt.Printf("%-12s %-50s\n", runID, s.Message)
	}
	return nil
}

func restoreSnapshot(cmd *cobra.Command, args []string) error {
	runID := args[0]
	cwd, _ := os.Getwd()
	m := snapshot.New(cwd)

	fmt.Printf("Restoring snapshot for run %s...\n", runID)
	if err := m.Restore(runID); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}
	fmt.Println("Snapshot restored successfully.")
	return nil
}
```

**Step 2: Register command in main.go**

Add to `init()` in `cmd/apex/main.go`:

```go
rootCmd.AddCommand(snapshotCmd)
```

**Step 3: Build and verify**

```bash
go build -o bin/apex ./cmd/apex/
./bin/apex snapshot --help
./bin/apex snapshot list --help
./bin/apex snapshot restore --help
```

**Step 4: Commit**

```bash
git add cmd/apex/snapshot.go cmd/apex/main.go
git commit -m "feat: add apex snapshot list and restore commands"
```

---

### Task 3: Integrate Snapshots into DAG Execution

**Files:**
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/run.go`

**Step 1: Add snapshot integration to run.go**

Add import:
```go
"github.com/lyndonlyu/apex/internal/snapshot"
```

Move the `runManifest.RunID` generation to early in the function (before execution). Currently at line 209, move `runID := uuid.New().String()` to right after risk gating (after line 73), so it's available for both snapshot and manifest.

After planning completes and before the "Execute DAG" section (before line 115), add snapshot creation:

```go
	// Create snapshot before execution
	runID := uuid.New().String()
	cwd, _ := os.Getwd()
	snapMgr := snapshot.New(cwd)
	snap, snapErr := snapMgr.Create(runID)
	if snapErr != nil {
		fmt.Fprintf(os.Stderr, "warning: snapshot creation failed: %v\n", snapErr)
	} else if snap != nil {
		fmt.Printf("Snapshot saved (%s)\n", snap.Message)
	}
```

After execution completes and outcome is determined (after the outcome block around line 193), add snapshot cleanup:

```go
	// Handle snapshot based on outcome
	if snap != nil {
		if outcome == "success" {
			if dropErr := snapMgr.Drop(runID); dropErr != nil {
				fmt.Fprintf(os.Stderr, "warning: snapshot cleanup failed: %v\n", dropErr)
			}
		} else {
			fmt.Printf("\nSnapshot available. Restore with: apex snapshot restore %s\n", runID)
		}
	}
```

Update the manifest `RunID` field (line 209) to use the pre-generated `runID` variable instead of `uuid.New().String()`.

**Step 2: Build and verify**

```bash
go build -o bin/apex ./cmd/apex/
```

**Step 3: Commit**

```bash
git add cmd/apex/run.go
git commit -m "feat: integrate snapshot creation into DAG execution pipeline"
```

---

### Task 4: E2E Verification

**Step 1: Run all tests**

```bash
go test ./... -v
```

Expected: All packages pass (15+ packages)

**Step 2: Build binary**

```bash
go build -o bin/apex ./cmd/apex/
```

**Step 3: Verify snapshot commands**

```bash
./bin/apex snapshot list        # Should show "No snapshots found."
./bin/apex snapshot --help      # Should show subcommands
```

**Step 4: Commit (if any fixes needed)**

Only commit if fixes were made during verification.

# Phase 6: Kill Switch Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an emergency kill switch that halts all DAG execution via file-based signaling and context cancellation.

**Architecture:** New `internal/killswitch` package with a file-polling watcher that returns a derived context. When `~/.claude/KILL_SWITCH` appears, the context cancels, propagating through Pool workers. Two new CLI commands (`kill-switch`, `resume`) for manual control.

**Tech Stack:** Go stdlib (`context`, `os`, `time`), Cobra CLI, Testify

---

### Task 1: Kill Switch Watcher Package

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/killswitch/killswitch.go`
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/killswitch/killswitch_test.go`

**Step 1: Write the failing tests**

```go
package killswitch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsActive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "KILL_SWITCH")
	w := New(path)

	assert.False(t, w.IsActive())

	os.WriteFile(path, []byte("test"), 0644)
	assert.True(t, w.IsActive())
}

func TestActivateAndClear(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "KILL_SWITCH")
	w := New(path)

	require.NoError(t, w.Activate("emergency"))
	assert.True(t, w.IsActive())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "emergency", string(data))

	require.NoError(t, w.Clear())
	assert.False(t, w.IsActive())
}

func TestClearWhenNotActive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "KILL_SWITCH")
	w := New(path)

	// Clear on non-existent file should not error
	require.NoError(t, w.Clear())
}

func TestWatchDetectsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "KILL_SWITCH")
	w := New(path)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	watchCtx, watchCancel := w.Watch(ctx)
	defer watchCancel()

	// Create the kill switch file after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.WriteFile(path, []byte("stop"), 0644)
	}()

	// Wait for context to be cancelled
	<-watchCtx.Done()
	assert.ErrorIs(t, watchCtx.Err(), context.Canceled)
}

func TestWatchParentCancel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "KILL_SWITCH")
	w := New(path)

	ctx, cancel := context.WithCancel(context.Background())
	watchCtx, watchCancel := w.Watch(ctx)
	defer watchCancel()

	// Cancel parent — watch context should also cancel
	cancel()
	<-watchCtx.Done()
	assert.Error(t, watchCtx.Err())
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/killswitch/ -v`
Expected: FAIL — package does not exist

**Step 3: Implement killswitch.go**

```go
package killswitch

import (
	"context"
	"os"
	"time"
)

const DefaultInterval = 200 * time.Millisecond

// Watcher monitors a kill switch file and cancels a context when it appears.
type Watcher struct {
	path     string
	interval time.Duration
}

// New creates a Watcher that monitors the given file path.
func New(path string) *Watcher {
	return &Watcher{
		path:     path,
		interval: DefaultInterval,
	}
}

// Path returns the kill switch file path being watched.
func (w *Watcher) Path() string {
	return w.path
}

// IsActive returns true if the kill switch file exists.
func (w *Watcher) IsActive() bool {
	_, err := os.Stat(w.path)
	return err == nil
}

// Activate creates the kill switch file with the given reason.
func (w *Watcher) Activate(reason string) error {
	return os.WriteFile(w.path, []byte(reason), 0644)
}

// Clear removes the kill switch file. Returns nil if it doesn't exist.
func (w *Watcher) Clear() error {
	err := os.Remove(w.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Watch starts a goroutine that polls for the kill switch file.
// It returns a derived context that is cancelled when the file appears
// or when the parent context is cancelled. The caller must call the
// returned CancelFunc when done to release resources.
func (w *Watcher) Watch(ctx context.Context) (context.Context, context.CancelFunc) {
	watchCtx, cancel := context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-watchCtx.Done():
				return
			case <-ticker.C:
				if w.IsActive() {
					cancel()
					return
				}
			}
		}
	}()

	return watchCtx, cancel
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/killswitch/ -v`
Expected: All 5 tests PASS

**Step 5: Commit**

```bash
git add internal/killswitch/killswitch.go internal/killswitch/killswitch_test.go
git commit -m "feat(killswitch): add file-based kill switch watcher with context cancellation"
```

---

### Task 2: CLI Commands (kill-switch + resume)

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/killswitch.go`
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/main.go` — add `rootCmd.AddCommand(killSwitchCmd, resumeCmd)`

**Step 1: Implement killswitch.go CLI**

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lyndonlyu/apex/internal/killswitch"
	"github.com/spf13/cobra"
)

var killSwitchCmd = &cobra.Command{
	Use:   "kill-switch [reason]",
	Short: "Activate emergency kill switch to halt all execution",
	RunE:  activateKillSwitch,
}

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Deactivate kill switch and allow execution to continue",
	RunE:  deactivateKillSwitch,
}

func killSwitchPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "KILL_SWITCH")
}

func activateKillSwitch(cmd *cobra.Command, args []string) error {
	w := killswitch.New(killSwitchPath())

	if w.IsActive() {
		fmt.Printf("Kill switch already active at %s\n", w.Path())
		return nil
	}

	reason := "manual activation"
	if len(args) > 0 {
		reason = strings.Join(args, " ")
	}

	if err := w.Activate(reason); err != nil {
		return fmt.Errorf("failed to activate kill switch: %w", err)
	}

	fmt.Printf("Kill switch ACTIVATED at %s\n", w.Path())
	fmt.Printf("Reason: %s\n", reason)
	fmt.Println("All running executions will be stopped.")
	fmt.Println("Use 'apex resume' to deactivate.")
	return nil
}

func deactivateKillSwitch(cmd *cobra.Command, args []string) error {
	w := killswitch.New(killSwitchPath())

	if !w.IsActive() {
		fmt.Println("No kill switch active.")
		return nil
	}

	if err := w.Clear(); err != nil {
		return fmt.Errorf("failed to deactivate kill switch: %w", err)
	}

	fmt.Println("Kill switch DEACTIVATED. Execution may resume.")
	return nil
}
```

**Step 2: Register commands in main.go**

Add to the `init()` function in `cmd/apex/main.go`:

```go
rootCmd.AddCommand(killSwitchCmd)
rootCmd.AddCommand(resumeCmd)
```

**Step 3: Build and verify**

```bash
go build -o bin/apex ./cmd/apex/
./bin/apex kill-switch --help
./bin/apex resume --help
```

Expected: Both commands show help text

**Step 4: Commit**

```bash
git add cmd/apex/killswitch.go cmd/apex/main.go
git commit -m "feat: add apex kill-switch and apex resume commands"
```

---

### Task 3: Integrate Kill Switch into DAG Execution

**Files:**
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/run.go`

**Step 1: Add kill switch check and watcher to run.go**

Add import:
```go
"github.com/lyndonlyu/apex/internal/killswitch"
```

Replace this line in `runTask`:
```go
execErr := p.Execute(context.Background(), d)
```

With the kill switch integration block. The full changes to `runTask` are:

After `cfg.EnsureDirs()`, add pre-flight check:
```go
	// Kill switch pre-flight check
	ks := killswitch.New(killSwitchPath())
	if ks.IsActive() {
		return fmt.Errorf("kill switch is active at %s — use 'apex resume' to deactivate", ks.Path())
	}
```

Replace the `context.Background()` in `p.Execute` with a watched context:
```go
	killCtx, killCancel := ks.Watch(context.Background())
	defer killCancel()

	fmt.Println("Executing...")
	start := time.Now()
	execErr := p.Execute(killCtx, d)
	duration := time.Since(start)

	// Detect kill switch interruption
	killedBySwitch := ks.IsActive() && killCtx.Err() != nil
```

After restoring original task names and before audit, add killed status handling:
```go
	if killedBySwitch {
		fmt.Println("\n[KILL SWITCH] Execution halted by kill switch.")
	}
```

Update the outcome determination to handle kill switch:
```go
	outcome := "success"
	if killedBySwitch {
		outcome = "killed"
	} else if d.HasFailure() {
		if execErr != nil {
			outcome = "failure"
		} else {
			outcome = "partial_failure"
		}
	}
```

**Step 2: Build and verify**

```bash
go build -o bin/apex ./cmd/apex/
```

Expected: Build succeeds

**Step 3: Commit**

```bash
git add cmd/apex/run.go
git commit -m "feat: integrate kill switch into DAG execution pipeline"
```

---

### Task 4: E2E Verification

**Step 1: Run all tests**

```bash
go test ./... -v
```

Expected: All packages pass (14+ packages)

**Step 2: Build binary**

```bash
go build -o bin/apex ./cmd/apex/
```

**Step 3: Verify kill-switch command**

```bash
./bin/apex kill-switch "testing"
./bin/apex kill-switch       # Should say "already active"
./bin/apex resume
./bin/apex resume            # Should say "no kill switch active"
```

**Step 4: Verify run refuses when active**

```bash
./bin/apex kill-switch "block test"
./bin/apex run "test task" 2>&1 | head -5   # Should refuse
./bin/apex resume
```

**Step 5: Commit (if any fixes needed)**

Only commit if fixes were made during verification.

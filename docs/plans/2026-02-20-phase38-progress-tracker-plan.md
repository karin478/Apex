# Progress Tracker Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an `internal/progress` package that tracks task progress through a Start → Update → Complete/Fail lifecycle with thread-safe access.

**Architecture:** Status enum, ProgressReport struct, Tracker with mutex-protected map, lifecycle methods (Start/Update/Complete/Fail/Get/List). Format functions + Cobra CLI.

**Tech Stack:** Go, `time`, `sync`, `encoding/json`, `sort`, Testify, Cobra CLI

---

### Task 1: Progress Tracker Core — Status + ProgressReport + Tracker (7 tests)

**Files:**
- Create: `internal/progress/progress.go`
- Create: `internal/progress/progress_test.go`

**Step 1: Write 7 failing tests**

```go
package progress

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTracker(t *testing.T) {
	tr := NewTracker()
	list := tr.List()
	assert.Empty(t, list)
}

func TestTrackerStart(t *testing.T) {
	tr := NewTracker()
	report := tr.Start("task-1", "initialization")

	assert.Equal(t, "task-1", report.TaskID)
	assert.Equal(t, "initialization", report.Phase)
	assert.Equal(t, 0, report.Percent)
	assert.Equal(t, StatusRunning, report.Status)
	assert.False(t, report.StartedAt.IsZero())
	assert.False(t, report.UpdatedAt.IsZero())
}

func TestTrackerUpdate(t *testing.T) {
	tr := NewTracker()
	tr.Start("task-1", "build")

	t.Run("normal update", func(t *testing.T) {
		err := tr.Update("task-1", 50, "halfway done")
		require.NoError(t, err)

		report, err := tr.Get("task-1")
		require.NoError(t, err)
		assert.Equal(t, 50, report.Percent)
		assert.Equal(t, "halfway done", report.Message)
	})

	t.Run("clamp above 100", func(t *testing.T) {
		err := tr.Update("task-1", 150, "over")
		require.NoError(t, err)

		report, err := tr.Get("task-1")
		require.NoError(t, err)
		assert.Equal(t, 100, report.Percent)
	})

	t.Run("clamp below 0", func(t *testing.T) {
		err := tr.Update("task-1", -10, "under")
		require.NoError(t, err)

		report, err := tr.Get("task-1")
		require.NoError(t, err)
		assert.Equal(t, 0, report.Percent)
	})
}

func TestTrackerUpdateNotFound(t *testing.T) {
	tr := NewTracker()
	err := tr.Update("nonexistent", 50, "test")
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestTrackerComplete(t *testing.T) {
	tr := NewTracker()
	tr.Start("task-1", "deploy")
	err := tr.Update("task-1", 75, "almost")
	require.NoError(t, err)

	err = tr.Complete("task-1", "done successfully")
	require.NoError(t, err)

	report, err := tr.Get("task-1")
	require.NoError(t, err)
	assert.Equal(t, 100, report.Percent)
	assert.Equal(t, StatusCompleted, report.Status)
	assert.Equal(t, "done successfully", report.Message)
}

func TestTrackerFail(t *testing.T) {
	tr := NewTracker()
	tr.Start("task-1", "deploy")
	err := tr.Update("task-1", 40, "in progress")
	require.NoError(t, err)

	err = tr.Fail("task-1", "connection lost")
	require.NoError(t, err)

	report, err := tr.Get("task-1")
	require.NoError(t, err)
	assert.Equal(t, 40, report.Percent)
	assert.Equal(t, StatusFailed, report.Status)
	assert.Equal(t, "connection lost", report.Message)
}

func TestTrackerList(t *testing.T) {
	tr := NewTracker()
	tr.Start("task-a", "phase-a")
	time.Sleep(10 * time.Millisecond)
	tr.Start("task-b", "phase-b")
	time.Sleep(10 * time.Millisecond)
	err := tr.Update("task-a", 50, "updated last")
	require.NoError(t, err)

	list := tr.List()
	assert.Len(t, list, 2)
	// task-a was updated most recently, should be first.
	assert.Equal(t, "task-a", list[0].TaskID)
	assert.Equal(t, "task-b", list[1].TaskID)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/progress/ -v -count=1`
Expected: FAIL — functions not defined

**Step 3: Write minimal implementation**

```go
// Package progress provides structured task progress tracking and reporting.
package progress

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ErrTaskNotFound is returned when a task is not being tracked.
var ErrTaskNotFound = errors.New("progress: task not found")

// Status represents the state of a tracked task.
type Status string

const (
	StatusRunning   Status = "RUNNING"
	StatusCompleted Status = "COMPLETED"
	StatusFailed    Status = "FAILED"
)

// ProgressReport holds the progress state for a single task.
type ProgressReport struct {
	TaskID    string    `json:"task_id"`
	Phase     string    `json:"phase"`
	Percent   int       `json:"percent"`
	Message   string    `json:"message"`
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Status    Status    `json:"status"`
}

// Tracker manages progress reports for multiple tasks.
type Tracker struct {
	mu      sync.RWMutex
	reports map[string]*ProgressReport
}

// NewTracker creates an empty Tracker.
func NewTracker() *Tracker {
	return &Tracker{reports: make(map[string]*ProgressReport)}
}

// Start begins tracking a task. Overwrites if the task already exists.
func (t *Tracker) Start(taskID, phase string) *ProgressReport {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	report := &ProgressReport{
		TaskID:    taskID,
		Phase:     phase,
		Percent:   0,
		Status:    StatusRunning,
		StartedAt: now,
		UpdatedAt: now,
	}
	t.reports[taskID] = report
	return report
}

// Update updates the progress percentage and message for a task.
// Percent is clamped to 0-100.
func (t *Tracker) Update(taskID string, percent int, message string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	report, ok := t.reports[taskID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}

	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	report.Percent = percent
	report.Message = message
	report.UpdatedAt = time.Now()
	return nil
}

// Complete marks a task as completed with Percent=100.
func (t *Tracker) Complete(taskID, message string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	report, ok := t.reports[taskID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}

	report.Percent = 100
	report.Status = StatusCompleted
	report.Message = message
	report.UpdatedAt = time.Now()
	return nil
}

// Fail marks a task as failed, keeping the current percent.
func (t *Tracker) Fail(taskID, message string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	report, ok := t.reports[taskID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}

	report.Status = StatusFailed
	report.Message = message
	report.UpdatedAt = time.Now()
	return nil
}

// Get returns a copy of the progress report for a task.
func (t *Tracker) Get(taskID string) (ProgressReport, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	report, ok := t.reports[taskID]
	if !ok {
		return ProgressReport{}, fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}
	return *report, nil
}

// List returns all reports sorted by UpdatedAt descending.
func (t *Tracker) List() []ProgressReport {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]ProgressReport, 0, len(t.reports))
	for _, r := range t.reports {
		result = append(result, *r)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/progress/ -v -count=1 -race`
Expected: PASS (7 tests)

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/progress/progress.go internal/progress/progress_test.go
git commit -m "feat(progress): add task progress tracker with Start/Update/Complete/Fail lifecycle

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: Format Functions + CLI Commands

**Files:**
- Create: `internal/progress/format.go`
- Create: `cmd/apex/progress.go`
- Modify: `cmd/apex/main.go` (add `rootCmd.AddCommand(progressCmd)` after the `modeCmd` line)

**Step 1: Write format.go**

```go
package progress

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatProgressList formats a list of reports as a human-readable table.
func FormatProgressList(reports []ProgressReport) string {
	if len(reports) == 0 {
		return "No tasks tracked.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-20s %-15s %-9s %-12s %s\n",
		"TASK_ID", "PHASE", "PERCENT", "STATUS", "UPDATED")
	for _, r := range reports {
		fmt.Fprintf(&b, "%-20s %-15s %-9s %-12s %s\n",
			r.TaskID, r.Phase, fmt.Sprintf("%d%%", r.Percent), r.Status,
			r.UpdatedAt.Format("15:04:05"))
	}
	return b.String()
}

// FormatProgressReport formats a single progress report for display.
func FormatProgressReport(report ProgressReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Task ID:   %s\n", report.TaskID)
	fmt.Fprintf(&b, "Phase:     %s\n", report.Phase)
	fmt.Fprintf(&b, "Percent:   %d%%\n", report.Percent)
	fmt.Fprintf(&b, "Status:    %s\n", report.Status)
	fmt.Fprintf(&b, "Message:   %s\n", report.Message)
	fmt.Fprintf(&b, "Started:   %s\n", report.StartedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "Updated:   %s\n", report.UpdatedAt.Format(time.RFC3339))
	return b.String()
}

// FormatProgressListJSON formats progress reports as indented JSON.
func FormatProgressListJSON(reports []ProgressReport) (string, error) {
	data, err := json.MarshalIndent(reports, "", "  ")
	if err != nil {
		return "", fmt.Errorf("progress: json marshal: %w", err)
	}
	return string(data), nil
}
```

Note: `FormatProgressReport` needs `"time"` import for `time.RFC3339`.

**Step 2: Write cmd/apex/progress.go CLI**

```go
package main

import (
	"fmt"

	"github.com/lyndonlyu/apex/internal/progress"
	"github.com/spf13/cobra"
)

var progressFormat string

// progressTracker is a package-level tracker for CLI demo purposes.
var progressTracker = progress.NewTracker()

var progressCmd = &cobra.Command{
	Use:   "progress",
	Short: "Task progress tracking",
}

var progressListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tracked tasks",
	RunE:  runProgressList,
}

var progressShowCmd = &cobra.Command{
	Use:   "show <task-id>",
	Short: "Show progress for a specific task",
	Args:  cobra.ExactArgs(1),
	RunE:  runProgressShow,
}

var progressStartCmd = &cobra.Command{
	Use:   "start <task-id>",
	Short: "Start tracking a task",
	Args:  cobra.ExactArgs(1),
	RunE:  runProgressStart,
}

var progressPhase string

func init() {
	progressListCmd.Flags().StringVar(&progressFormat, "format", "", "Output format (json)")
	progressStartCmd.Flags().StringVar(&progressPhase, "phase", "default", "Phase name")
	progressCmd.AddCommand(progressListCmd, progressShowCmd, progressStartCmd)
}

func runProgressList(cmd *cobra.Command, args []string) error {
	list := progressTracker.List()

	if progressFormat == "json" {
		out, err := progress.FormatProgressListJSON(list)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(progress.FormatProgressList(list))
	}
	return nil
}

func runProgressShow(cmd *cobra.Command, args []string) error {
	report, err := progressTracker.Get(args[0])
	if err != nil {
		return err
	}
	fmt.Print(progress.FormatProgressReport(report))
	return nil
}

func runProgressStart(cmd *cobra.Command, args []string) error {
	report := progressTracker.Start(args[0], progressPhase)
	fmt.Print(progress.FormatProgressReport(*report))
	return nil
}
```

**Step 3: Add command to main.go**

Add `rootCmd.AddCommand(progressCmd)` after the `modeCmd` line in `cmd/apex/main.go`.

**Step 4: Run build + tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build ./cmd/apex/ && go test ./internal/progress/ -v -count=1`
Expected: BUILD OK, PASS

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/progress/format.go cmd/apex/progress.go cmd/apex/main.go
git commit -m "feat(progress): add format functions and CLI for progress list/show/start

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: E2E Tests (3 tests)

**Files:**
- Create: `e2e/progress_test.go`

**Step 1: Write E2E tests**

```go
package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProgressList(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("progress", "list")

	assert.Equal(t, 0, exitCode,
		"apex progress list should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "No tasks tracked"),
		"stdout should indicate no tasks, got: %s", stdout)
}

func TestProgressStart(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("progress", "start", "test-task", "--phase", "build")

	assert.Equal(t, 0, exitCode,
		"apex progress start should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "test-task"),
		"stdout should contain task ID, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "build"),
		"stdout should contain phase name, got: %s", stdout)
}

func TestProgressShowNotFound(t *testing.T) {
	env := newTestEnv(t)

	_, _, exitCode := env.runApex("progress", "show", "nonexistent-task")

	assert.NotEqual(t, 0, exitCode,
		"apex progress show with nonexistent task should exit non-zero")
}
```

**Step 2: Build and run E2E tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build -o bin/apex ./cmd/apex/ && go test ./e2e/ -run TestProgress -v -count=1`
Expected: PASS (3 tests)

**Step 3: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add e2e/progress_test.go
git commit -m "test(e2e): add E2E tests for progress list, start, and show

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Update PROGRESS.md

**Files:**
- Modify: `PROGRESS.md`

**Step 1: Update completed phases table**

Add row: `| 38 | Progress Tracker | \`2026-02-20-phase38-progress-tracker-design.md\` | Done |`

**Step 2: Update Current section**

Change "Phase 38 — TBD" → "Phase 39 — TBD"

**Step 3: Update test counts**

- Unit tests: 43 → 44 packages
- E2E tests: 111 → 114 tests

**Step 4: Add Key Package**

Add: `| \`internal/progress\` | Structured task progress tracking with Start/Update/Complete/Fail lifecycle and percent clamping |`

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add PROGRESS.md
git commit -m "docs: mark Phase 38 Progress Tracker as complete

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

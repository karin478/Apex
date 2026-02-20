# Phase 21: Run Manifest Diffing — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Compare two run manifests side-by-side via `apex run diff <id1> <id2>` to debug reproducibility issues.

**Architecture:** Extend existing `internal/manifest` package with `Diff()`, `FieldDiff`, `NodeDiff`, `DiffResult` types and format functions. New `diff` subcommand under `apex run`. No new packages.

**Tech Stack:** Go, Cobra CLI, Testify, encoding/json, fmt, strings

---

## Task 1: Diff Core — Types + Diff Function

**Files:**
- Create: `internal/manifest/diff.go`
- Create: `internal/manifest/diff_test.go`

**Implementation:** `diff.go` with types and `Diff()` function.

```go
package manifest

import "fmt"

// FieldDiff represents a difference in a single top-level field.
type FieldDiff struct {
	Field string `json:"field"`
	Left  string `json:"left"`
	Right string `json:"right"`
}

// NodeDiff represents a difference at the node level.
type NodeDiff struct {
	NodeID string      `json:"node_id"`
	Type   string      `json:"type"` // "changed", "left_only", "right_only"
	Fields []FieldDiff `json:"fields,omitempty"`
}

// DiffResult holds the complete diff between two manifests.
type DiffResult struct {
	LeftRunID  string      `json:"left_run_id"`
	RightRunID string      `json:"right_run_id"`
	Fields     []FieldDiff `json:"fields"`
	NodeDiffs  []NodeDiff  `json:"node_diffs"`
}

// Diff compares two manifests and returns their differences.
func Diff(left, right *Manifest) *DiffResult {
	result := &DiffResult{
		LeftRunID:  left.RunID,
		RightRunID: right.RunID,
	}

	// Compare top-level fields
	pairs := []struct {
		name  string
		left  string
		right string
	}{
		{"task", left.Task, right.Task},
		{"model", left.Model, right.Model},
		{"effort", left.Effort, right.Effort},
		{"risk_level", left.RiskLevel, right.RiskLevel},
		{"node_count", fmt.Sprintf("%d", left.NodeCount), fmt.Sprintf("%d", right.NodeCount)},
		{"duration_ms", fmt.Sprintf("%d", left.DurationMs), fmt.Sprintf("%d", right.DurationMs)},
		{"outcome", left.Outcome, right.Outcome},
		{"trace_id", left.TraceID, right.TraceID},
	}
	for _, p := range pairs {
		if p.left != p.right {
			result.Fields = append(result.Fields, FieldDiff{Field: p.name, Left: p.left, Right: p.right})
		}
	}

	// Build node maps by ID
	leftNodes := make(map[string]NodeResult)
	for _, n := range left.Nodes {
		leftNodes[n.ID] = n
	}
	rightNodes := make(map[string]NodeResult)
	for _, n := range right.Nodes {
		rightNodes[n.ID] = n
	}

	// Nodes in both: check for changes
	for id, ln := range leftNodes {
		rn, ok := rightNodes[id]
		if !ok {
			result.NodeDiffs = append(result.NodeDiffs, NodeDiff{NodeID: id, Type: "left_only"})
			continue
		}
		var fields []FieldDiff
		if ln.Status != rn.Status {
			fields = append(fields, FieldDiff{Field: "status", Left: ln.Status, Right: rn.Status})
		}
		if ln.Error != rn.Error {
			fields = append(fields, FieldDiff{Field: "error", Left: ln.Error, Right: rn.Error})
		}
		if len(fields) > 0 {
			result.NodeDiffs = append(result.NodeDiffs, NodeDiff{NodeID: id, Type: "changed", Fields: fields})
		}
	}

	// Nodes only in right
	for id := range rightNodes {
		if _, ok := leftNodes[id]; !ok {
			result.NodeDiffs = append(result.NodeDiffs, NodeDiff{NodeID: id, Type: "right_only"})
		}
	}

	return result
}
```

**Tests (6):**

```go
package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiffIdentical(t *testing.T) {
	m := &Manifest{RunID: "a", Task: "t", Model: "m", Effort: "low", Outcome: "success"}
	d := Diff(m, m)
	assert.Equal(t, "a", d.LeftRunID)
	assert.Empty(t, d.Fields)
	assert.Empty(t, d.NodeDiffs)
}

func TestDiffTopLevelFields(t *testing.T) {
	left := &Manifest{RunID: "a", Model: "sonnet", Effort: "low", Outcome: "success"}
	right := &Manifest{RunID: "b", Model: "opus", Effort: "high", Outcome: "failed"}
	d := Diff(left, right)
	assert.Len(t, d.Fields, 3) // model, effort, outcome
	assert.Equal(t, "model", d.Fields[0].Field)
	assert.Equal(t, "sonnet", d.Fields[0].Left)
	assert.Equal(t, "opus", d.Fields[0].Right)
}

func TestDiffNodeChanged(t *testing.T) {
	left := &Manifest{RunID: "a", Nodes: []NodeResult{{ID: "n1", Status: "SUCCESS"}}}
	right := &Manifest{RunID: "b", Nodes: []NodeResult{{ID: "n1", Status: "FAILED", Error: "timeout"}}}
	d := Diff(left, right)
	assert.Len(t, d.NodeDiffs, 1)
	assert.Equal(t, "changed", d.NodeDiffs[0].Type)
	assert.Equal(t, "n1", d.NodeDiffs[0].NodeID)
	assert.Len(t, d.NodeDiffs[0].Fields, 2) // status + error
}

func TestDiffNodeLeftOnly(t *testing.T) {
	left := &Manifest{RunID: "a", Nodes: []NodeResult{{ID: "n1", Status: "SUCCESS"}, {ID: "n2", Status: "SUCCESS"}}}
	right := &Manifest{RunID: "b", Nodes: []NodeResult{{ID: "n1", Status: "SUCCESS"}}}
	d := Diff(left, right)
	assert.Len(t, d.NodeDiffs, 1)
	assert.Equal(t, "left_only", d.NodeDiffs[0].Type)
	assert.Equal(t, "n2", d.NodeDiffs[0].NodeID)
}

func TestDiffNodeRightOnly(t *testing.T) {
	left := &Manifest{RunID: "a", Nodes: []NodeResult{{ID: "n1", Status: "SUCCESS"}}}
	right := &Manifest{RunID: "b", Nodes: []NodeResult{{ID: "n1", Status: "SUCCESS"}, {ID: "n3", Status: "FAILED"}}}
	d := Diff(left, right)
	assert.Len(t, d.NodeDiffs, 1)
	assert.Equal(t, "right_only", d.NodeDiffs[0].Type)
	assert.Equal(t, "n3", d.NodeDiffs[0].NodeID)
}

func TestDiffNoDifferences(t *testing.T) {
	left := &Manifest{RunID: "a", Model: "m", Nodes: []NodeResult{{ID: "n1", Status: "SUCCESS"}}}
	right := &Manifest{RunID: "b", Model: "m", Nodes: []NodeResult{{ID: "n1", Status: "SUCCESS"}}}
	d := Diff(left, right)
	assert.Empty(t, d.Fields)
	assert.Empty(t, d.NodeDiffs)
}
```

**Commit:** `feat(manifest): add Diff function with FieldDiff, NodeDiff, DiffResult types`

---

## Task 2: Format Functions — Human + JSON

**Files:**
- Create: `internal/manifest/diff_format.go`
- Create: `internal/manifest/diff_format_test.go`

**Implementation:** `diff_format.go` with human table and JSON output.

```go
package manifest

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatDiffHuman returns a human-readable table of differences.
func FormatDiffHuman(d *DiffResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("=== Run Diff: %s vs %s ===\n\n", d.LeftRunID, d.RightRunID))

	if len(d.Fields) == 0 && len(d.NodeDiffs) == 0 {
		sb.WriteString("No differences found.\n")
		return sb.String()
	}

	if len(d.Fields) > 0 {
		sb.WriteString(fmt.Sprintf("%-15s | %-25s | %-25s\n", "Field", d.LeftRunID, d.RightRunID))
		sb.WriteString(strings.Repeat("-", 70) + "\n")
		for _, f := range d.Fields {
			sb.WriteString(fmt.Sprintf("%-15s | %-25s | %-25s\n", f.Field, f.Left, f.Right))
		}
	}

	if len(d.NodeDiffs) > 0 {
		sb.WriteString("\nNode Differences:\n")
		for _, nd := range d.NodeDiffs {
			switch nd.Type {
			case "left_only":
				sb.WriteString(fmt.Sprintf("  [left_only]  %s: (not in %s)\n", nd.NodeID, d.RightRunID))
			case "right_only":
				sb.WriteString(fmt.Sprintf("  [right_only] %s: (not in %s)\n", nd.NodeID, d.LeftRunID))
			case "changed":
				for _, f := range nd.Fields {
					sb.WriteString(fmt.Sprintf("  [changed]    %s: %s %s → %s\n", nd.NodeID, f.Field, f.Left, f.Right))
				}
			}
		}
	}

	return sb.String()
}

// FormatDiffJSON returns the diff as a JSON string.
func FormatDiffJSON(d *DiffResult) (string, error) {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}
```

**Tests (3):**

```go
package manifest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatDiffHumanNoDiff(t *testing.T) {
	d := &DiffResult{LeftRunID: "a", RightRunID: "b"}
	out := FormatDiffHuman(d)
	assert.Contains(t, out, "No differences found")
}

func TestFormatDiffHumanWithChanges(t *testing.T) {
	d := &DiffResult{
		LeftRunID:  "run-1",
		RightRunID: "run-2",
		Fields:     []FieldDiff{{Field: "model", Left: "sonnet", Right: "opus"}},
		NodeDiffs:  []NodeDiff{{NodeID: "n1", Type: "changed", Fields: []FieldDiff{{Field: "status", Left: "SUCCESS", Right: "FAILED"}}}},
	}
	out := FormatDiffHuman(d)
	assert.Contains(t, out, "model")
	assert.Contains(t, out, "sonnet")
	assert.Contains(t, out, "[changed]")
	assert.Contains(t, out, "SUCCESS → FAILED")
}

func TestFormatDiffJSON(t *testing.T) {
	d := &DiffResult{
		LeftRunID:  "a",
		RightRunID: "b",
		Fields:     []FieldDiff{{Field: "model", Left: "x", Right: "y"}},
	}
	out, err := FormatDiffJSON(d)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(out, "{"))
	assert.Contains(t, out, `"left_run_id"`)
	assert.Contains(t, out, `"model"`)
}
```

**Commit:** `feat(manifest): add FormatDiffHuman and FormatDiffJSON functions`

---

## Task 3: CLI Command — `apex run diff`

**Files:**
- Create: `cmd/apex/rundiff.go`
- Modify: `cmd/apex/run.go` (register diffCmd as subcommand of runCmd — actually, `run` uses `Args: cobra.MinimumNArgs(1)` so we need a separate top-level `diff` command OR adjust run command)

**Note:** Since `runCmd` uses `RunE` and `Args: cobra.MinimumNArgs(1)`, adding a subcommand would conflict. Instead, create `apex diff <id1> <id2>` as a top-level command.

**Implementation:** `cmd/apex/diff.go`

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/manifest"
	"github.com/spf13/cobra"
)

var diffFormat string

var diffCmd = &cobra.Command{
	Use:   "diff <run-id-1> <run-id-2>",
	Short: "Compare two run manifests",
	Long:  "Load two run manifests by ID and display their differences side-by-side.",
	Args:  cobra.ExactArgs(2),
	RunE:  runDiff,
}

func init() {
	diffCmd.Flags().StringVar(&diffFormat, "format", "human", "Output format: human or json")
}

func runDiff(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	runsDir := filepath.Join(home, ".apex", "runs")
	store := manifest.NewStore(runsDir)

	left, err := store.Load(args[0])
	if err != nil {
		return fmt.Errorf("failed to load left manifest %s: %w", args[0], err)
	}
	right, err := store.Load(args[1])
	if err != nil {
		return fmt.Errorf("failed to load right manifest %s: %w", args[1], err)
	}

	d := manifest.Diff(left, right)

	switch diffFormat {
	case "json":
		out, fmtErr := manifest.FormatDiffJSON(d)
		if fmtErr != nil {
			return fmt.Errorf("format error: %w", fmtErr)
		}
		fmt.Print(out)
	default:
		fmt.Print(manifest.FormatDiffHuman(d))
	}
	return nil
}
```

**Modify `cmd/apex/main.go`:** Add `rootCmd.AddCommand(diffCmd)`.

**Commit:** `feat(cli): add apex diff command for manifest comparison`

---

## Task 4: E2E Tests

**Files:**
- Create: `e2e/diff_test.go`

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

func TestDiffMissingRun(t *testing.T) {
	env := newTestEnv(t)

	_, stderr, code := env.runApex("diff", "nonexistent-1", "nonexistent-2")
	assert.NotEqual(t, 0, code, "apex diff with missing runs should fail")
	assert.Contains(t, stderr, "failed to load", "should mention load failure")
}

func TestDiffTwoRuns(t *testing.T) {
	env := newTestEnv(t)

	// Create two fake manifests directly
	for _, run := range []struct {
		id      string
		model   string
		outcome string
	}{
		{"run-aaa", "sonnet", "success"},
		{"run-bbb", "opus", "failed"},
	} {
		dir := filepath.Join(env.runsDir(), run.id)
		require.NoError(t, os.MkdirAll(dir, 0755))
		data := []byte(fmt.Sprintf(`{"run_id":"%s","model":"%s","outcome":"%s","nodes":[]}`, run.id, run.model, run.outcome))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0644))
	}

	stdout, stderr, code := env.runApex("diff", "run-aaa", "run-bbb")
	assert.Equal(t, 0, code, "apex diff should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "model")
	assert.Contains(t, stdout, "sonnet")
	assert.Contains(t, stdout, "opus")
	assert.Contains(t, stdout, "outcome")
}

func TestDiffJSONFormat(t *testing.T) {
	env := newTestEnv(t)

	// Create two fake manifests
	for _, id := range []string{"run-x", "run-y"} {
		dir := filepath.Join(env.runsDir(), id)
		require.NoError(t, os.MkdirAll(dir, 0755))
		data := []byte(fmt.Sprintf(`{"run_id":"%s","model":"m","outcome":"success","nodes":[]}`, id))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0644))
	}

	stdout, stderr, code := env.runApex("diff", "run-x", "run-y", "--format", "json")
	assert.Equal(t, 0, code, "apex diff --format json should exit 0; stderr=%s", stderr)

	var result map[string]interface{}
	err := json.Unmarshal([]byte(stdout), &result)
	assert.NoError(t, err, "output should be valid JSON")
	assert.Equal(t, "run-x", result["left_run_id"])
}
```

**Note:** E2E tests need `import "fmt"` for `fmt.Sprintf`.

**Commit:** `test(e2e): add diff command E2E tests`

---

## Task 5: Update PROGRESS.md

Update the Phase table to add Phase 21 as Done, update test counts.

**Commit:** `docs: mark Phase 21 Run Manifest Diffing as complete`

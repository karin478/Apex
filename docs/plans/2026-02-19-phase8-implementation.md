# Phase 8: Human-in-the-Loop Approval Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace hard rejection of HIGH-risk tasks with interactive per-node approval, letting users approve, skip, or reject individual DAG nodes.

**Architecture:** Update governance to gate HIGH at approval (not rejection). New `internal/approval` package with `Reviewer` that uses injected `io.Reader/Writer` for testability. Add `NodeSlice()` and `RemoveNode()` to DAG. Integration in `run.go` calls reviewer after planning, removes skipped nodes, logs approval to audit.

**Tech Stack:** Go stdlib (`bufio`, `fmt`, `io`, `sort`, `strings`), Testify, Cobra

---

### Task 1: Update Governance Risk Levels

**Files:**
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/governance/risk.go`
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/governance/risk_test.go`

**Step 1: Update failing tests first**

In `risk_test.go`, update `TestShouldReject` so HIGH returns false, and add `TestShouldRequireApproval`:

```go
func TestShouldReject(t *testing.T) {
	assert.False(t, LOW.ShouldReject())
	assert.False(t, MEDIUM.ShouldReject())
	assert.False(t, HIGH.ShouldReject())
	assert.True(t, CRITICAL.ShouldReject())
}

func TestShouldRequireApproval(t *testing.T) {
	assert.False(t, LOW.ShouldRequireApproval())
	assert.False(t, MEDIUM.ShouldRequireApproval())
	assert.True(t, HIGH.ShouldRequireApproval())
	assert.False(t, CRITICAL.ShouldRequireApproval())
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/governance/ -v -run "TestShouldReject|TestShouldRequireApproval"`

Expected: `TestShouldReject` FAIL (HIGH currently returns true), `TestShouldRequireApproval` FAIL (method doesn't exist)

**Step 3: Update risk.go**

Change `ShouldReject` and add `ShouldRequireApproval`:

```go
func (r RiskLevel) ShouldReject() bool {
	return r >= CRITICAL
}

func (r RiskLevel) ShouldRequireApproval() bool {
	return r == HIGH
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/governance/ -v`

Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/governance/risk.go internal/governance/risk_test.go
git commit -m "feat(governance): change HIGH to require approval instead of rejection"
```

---

### Task 2: Add DAG Helper Methods

**Files:**
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/dag/dag.go`
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/dag/dag_test.go`

**Step 1: Write failing tests**

Add to `dag_test.go`:

```go
func TestNodeSlice(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{"a"}},
		{ID: "c", Task: "task c", Depends: []string{"a"}},
	}
	d, _ := New(nodes)
	slice := d.NodeSlice()
	assert.Len(t, slice, 3)
	// Verify topological order: a before b, a before c
	indexMap := make(map[string]int)
	for i, n := range slice {
		indexMap[n.ID] = i
	}
	assert.Less(t, indexMap["a"], indexMap["b"])
	assert.Less(t, indexMap["a"], indexMap["c"])
}

func TestRemoveNode(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{"a"}},
		{ID: "c", Task: "task c", Depends: []string{"b"}},
		{ID: "d", Task: "task d", Depends: []string{"a"}},
	}
	d, _ := New(nodes)
	d.RemoveNode("b")

	assert.Len(t, d.Nodes, 3)
	assert.Nil(t, d.Nodes["b"])
	// c's dependency on b should be removed
	assert.NotContains(t, d.Nodes["c"].Depends, "b")
	// a and d are unchanged
	assert.NotNil(t, d.Nodes["a"])
	assert.NotNil(t, d.Nodes["d"])
}

func TestRemoveNodeNonexistent(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
	}
	d, _ := New(nodes)
	d.RemoveNode("nonexistent") // should not panic
	assert.Len(t, d.Nodes, 1)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/dag/ -v -run "TestNodeSlice|TestRemoveNode"`

Expected: FAIL (methods don't exist)

**Step 3: Implement NodeSlice and RemoveNode in dag.go**

Add these methods to `dag.go`:

```go
// NodeSlice returns nodes in topological order (dependencies before dependents).
// Thread-safe.
func (d *DAG) NodeSlice() []*Node {
	d.mu.Lock()
	defer d.mu.Unlock()

	visited := make(map[string]bool, len(d.Nodes))
	var order []*Node

	var visit func(id string)
	visit = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true
		n := d.Nodes[id]
		for _, dep := range n.Depends {
			visit(dep)
		}
		order = append(order, n)
	}

	// Sort keys for deterministic output
	ids := make([]string, 0, len(d.Nodes))
	for id := range d.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		visit(id)
	}

	return order
}

// RemoveNode removes a node from the DAG and strips it from all dependency lists.
// Thread-safe. No-op if the node does not exist.
func (d *DAG) RemoveNode(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.Nodes[id]; !ok {
		return
	}
	delete(d.Nodes, id)
	for _, n := range d.Nodes {
		filtered := n.Depends[:0]
		for _, dep := range n.Depends {
			if dep != id {
				filtered = append(filtered, dep)
			}
		}
		n.Depends = filtered
	}
}
```

Add `"sort"` to the import list in `dag.go`.

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/dag/ -v`

Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/dag/dag.go internal/dag/dag_test.go
git commit -m "feat(dag): add NodeSlice and RemoveNode helper methods"
```

---

### Task 3: Approval Package

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/approval/reviewer.go`
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/approval/reviewer_test.go`

**Step 1: Write the failing tests**

Create `reviewer_test.go` with all test scenarios:

```go
package approval

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lyndonlyu/apex/internal/dag"
	"github.com/lyndonlyu/apex/internal/governance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeNodes() []*dag.Node {
	return []*dag.Node{
		{ID: "1", Task: "migrate database schema"},
		{ID: "2", Task: "update API endpoints"},
		{ID: "3", Task: "run integration tests"},
	}
}

func classifyFunc(task string) governance.RiskLevel {
	return governance.Classify(task)
}

func TestApproveAll(t *testing.T) {
	in := strings.NewReader("a\n")
	out := &bytes.Buffer{}
	r := NewReviewer(in, out)
	result, err := r.Review(makeNodes(), classifyFunc)
	require.NoError(t, err)
	assert.True(t, result.Approved)
	assert.Len(t, result.Nodes, 3)
	for _, nd := range result.Nodes {
		assert.Equal(t, Approved, nd.Decision)
	}
	assert.Contains(t, out.String(), "Approval Required")
}

func TestQuitRejectsAll(t *testing.T) {
	in := strings.NewReader("q\n")
	out := &bytes.Buffer{}
	r := NewReviewer(in, out)
	result, err := r.Review(makeNodes(), classifyFunc)
	require.NoError(t, err)
	assert.False(t, result.Approved)
	for _, nd := range result.Nodes {
		assert.Equal(t, Rejected, nd.Decision)
	}
}

func TestReviewOneByOne(t *testing.T) {
	// r to review, then: approve node 1, skip node 2, approve node 3
	in := strings.NewReader("r\na\ns\na\n")
	out := &bytes.Buffer{}
	r := NewReviewer(in, out)
	result, err := r.Review(makeNodes(), classifyFunc)
	require.NoError(t, err)
	assert.True(t, result.Approved)
	assert.Equal(t, Approved, result.Nodes[0].Decision)
	assert.Equal(t, Skipped, result.Nodes[1].Decision)
	assert.Equal(t, Approved, result.Nodes[2].Decision)
}

func TestReviewRejectMidway(t *testing.T) {
	// r to review, approve node 1, then reject all
	in := strings.NewReader("r\na\nr\n")
	out := &bytes.Buffer{}
	r := NewReviewer(in, out)
	result, err := r.Review(makeNodes(), classifyFunc)
	require.NoError(t, err)
	assert.False(t, result.Approved)
	assert.Equal(t, Approved, result.Nodes[0].Decision)
	assert.Equal(t, Rejected, result.Nodes[1].Decision)
	assert.Equal(t, Rejected, result.Nodes[2].Decision)
}

func TestEmptyNodeList(t *testing.T) {
	in := strings.NewReader("")
	out := &bytes.Buffer{}
	r := NewReviewer(in, out)
	result, err := r.Review([]*dag.Node{}, classifyFunc)
	require.NoError(t, err)
	assert.True(t, result.Approved)
	assert.Empty(t, result.Nodes)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/approval/ -v`

Expected: FAIL (package doesn't exist)

**Step 3: Implement reviewer.go**

Create `reviewer.go`:

```go
package approval

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/lyndonlyu/apex/internal/dag"
	"github.com/lyndonlyu/apex/internal/governance"
)

type Decision int

const (
	Approved Decision = iota
	Skipped
	Rejected
)

func (d Decision) String() string {
	switch d {
	case Approved:
		return "approved"
	case Skipped:
		return "skipped"
	case Rejected:
		return "rejected"
	default:
		return "unknown"
	}
}

type NodeDecision struct {
	NodeID   string
	Decision Decision
}

type Result struct {
	Approved bool
	Nodes    []NodeDecision
}

type Reviewer struct {
	in  *bufio.Scanner
	out io.Writer
}

func NewReviewer(in io.Reader, out io.Writer) *Reviewer {
	return &Reviewer{
		in:  bufio.NewScanner(in),
		out: out,
	}
}

func (r *Reviewer) Review(nodes []*dag.Node, classify func(string) governance.RiskLevel) (*Result, error) {
	if len(nodes) == 0 {
		return &Result{Approved: true}, nil
	}

	// Display plan
	fmt.Fprintf(r.out, "\nApproval Required — %d steps planned\n", len(nodes))
	fmt.Fprintln(r.out, strings.Repeat("-", 44))
	for i, n := range nodes {
		risk := classify(n.Task)
		fmt.Fprintf(r.out, "  [%d] %-30s %s\n", i+1, n.Task, risk)
	}
	fmt.Fprintln(r.out, strings.Repeat("-", 44))
	fmt.Fprintf(r.out, "(a)pprove all / (r)eview one-by-one / (q)uit: ")

	choice := r.readLine()

	switch strings.ToLower(strings.TrimSpace(choice)) {
	case "a":
		return r.approveAll(nodes), nil
	case "r":
		return r.reviewOneByOne(nodes, classify), nil
	default:
		return r.rejectAll(nodes), nil
	}
}

func (r *Reviewer) readLine() string {
	if r.in.Scan() {
		return r.in.Text()
	}
	return ""
}

func (r *Reviewer) approveAll(nodes []*dag.Node) *Result {
	decisions := make([]NodeDecision, len(nodes))
	for i, n := range nodes {
		decisions[i] = NodeDecision{NodeID: n.ID, Decision: Approved}
	}
	return &Result{Approved: true, Nodes: decisions}
}

func (r *Reviewer) rejectAll(nodes []*dag.Node) *Result {
	decisions := make([]NodeDecision, len(nodes))
	for i, n := range nodes {
		decisions[i] = NodeDecision{NodeID: n.ID, Decision: Rejected}
	}
	return &Result{Approved: false, Nodes: decisions}
}

func (r *Reviewer) reviewOneByOne(nodes []*dag.Node, classify func(string) governance.RiskLevel) *Result {
	decisions := make([]NodeDecision, len(nodes))
	anyApproved := false

	for i, n := range nodes {
		risk := classify(n.Task)
		fmt.Fprintf(r.out, "\n[%d/%d] %s (%s)\n", i+1, len(nodes), n.Task, risk)
		fmt.Fprintf(r.out, "  (a)pprove / (s)kip / (r)eject all: ")

		choice := strings.ToLower(strings.TrimSpace(r.readLine()))

		switch choice {
		case "a":
			decisions[i] = NodeDecision{NodeID: n.ID, Decision: Approved}
			anyApproved = true
		case "s":
			decisions[i] = NodeDecision{NodeID: n.ID, Decision: Skipped}
		default: // "r" or anything else = reject all remaining
			decisions[i] = NodeDecision{NodeID: n.ID, Decision: Rejected}
			for j := i + 1; j < len(nodes); j++ {
				decisions[j] = NodeDecision{NodeID: nodes[j].ID, Decision: Rejected}
			}
			return &Result{Approved: false, Nodes: decisions}
		}
	}

	return &Result{Approved: anyApproved, Nodes: decisions}
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/approval/ -v`

Expected: ALL PASS (5 tests)

**Step 5: Commit**

```bash
git add internal/approval/reviewer.go internal/approval/reviewer_test.go
git commit -m "feat(approval): add interactive per-node approval reviewer"
```

---

### Task 4: Integrate Approval into run.go

**Files:**
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/run.go`

**Step 1: Add import and approval gate**

Add import:
```go
"github.com/lyndonlyu/apex/internal/approval"
```

Replace the current HIGH rejection block (lines 60-64) and MEDIUM confirm block (lines 66-74) with:

```go
	// Gate by risk level
	if risk.ShouldReject() {
		fmt.Printf("Task rejected (%s risk). Break it into smaller, safer steps.\n", risk)
		return nil
	}

	if risk.ShouldConfirm() {
		fmt.Printf("Warning: %s risk detected. Proceed? (y/n): ", risk)
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}
```

This block stays the same — `ShouldReject` now only triggers for CRITICAL and `ShouldConfirm` still only triggers for MEDIUM, so HIGH falls through.

After the DAG is created (after `fmt.Printf("Plan: %d steps\n", len(d.Nodes))` at line 94), add the approval gate:

```go
	// Approval gate for HIGH risk tasks
	if risk.ShouldRequireApproval() {
		reviewer := approval.NewReviewer(os.Stdin, os.Stdout)
		result, reviewErr := reviewer.Review(d.NodeSlice(), governance.Classify)
		if reviewErr != nil {
			return fmt.Errorf("approval review failed: %w", reviewErr)
		}
		if !result.Approved {
			fmt.Println("Approval rejected. Aborting.")
			return nil
		}
		// Remove skipped nodes
		for _, nd := range result.Nodes {
			if nd.Decision == approval.Skipped {
				d.RemoveNode(nd.NodeID)
			}
		}
		if len(d.Nodes) == 0 {
			fmt.Println("All nodes skipped. Nothing to execute.")
			return nil
		}
		fmt.Printf("Approved: %d steps to execute\n", len(d.Nodes))
	}
```

**Step 2: Verify build**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build -o bin/apex ./cmd/apex/`

Expected: Build succeeds

**Step 3: Commit**

```bash
git add cmd/apex/run.go
git commit -m "feat: integrate interactive approval gate into DAG execution pipeline"
```

---

### Task 5: E2E Verification

**Step 1: Run all tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./... -v 2>&1 | tail -30`

Expected: ALL packages PASS (17 packages now: 16 existing + 1 new approval)

**Step 2: Build**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build -o bin/apex ./cmd/apex/`

Expected: Build succeeds

**Step 3: Verify CLI commands**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
./bin/apex --help
./bin/apex run --help
```

Expected: No errors, run command shows in help

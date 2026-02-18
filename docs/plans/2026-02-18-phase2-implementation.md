# Phase 2 DAG + Agent Pool Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add DAG-based task orchestration so `apex run` can decompose complex tasks into subtasks and execute them concurrently.

**Architecture:** Planner calls Claude to decompose a task into a DAG of nodes. DAG package manages the graph and scheduling. Pool executes ready nodes concurrently via Executor. Simple tasks bypass the planner entirely.

**Tech Stack:** Go 1.25, existing packages (config, governance, executor, memory, audit), sync.Mutex for DAG thread safety, goroutines + channels for Pool.

---

### Task 1: Update Config for Phase 2

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1: Write failing tests for new config fields**

Add to `internal/config/config_test.go`:
```go
func TestDefaultConfigPhase2(t *testing.T) {
	cfg := Default()

	// Updated timeouts
	assert.Equal(t, 1800, cfg.Claude.Timeout)
	assert.Equal(t, 7200, cfg.Claude.LongTaskTimeout)

	// New planner config
	assert.Equal(t, "claude-opus-4-6", cfg.Planner.Model)
	assert.Equal(t, 120, cfg.Planner.Timeout)

	// New pool config
	assert.Equal(t, 4, cfg.Pool.MaxConcurrent)
}

func TestLoadConfigPhase2Override(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	content := []byte(`planner:
  model: claude-sonnet-4-6
  timeout: 60
pool:
  max_concurrent: 2
`)
	require.NoError(t, os.WriteFile(configPath, content, 0644))

	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Equal(t, "claude-sonnet-4-6", cfg.Planner.Model)
	assert.Equal(t, 60, cfg.Planner.Timeout)
	assert.Equal(t, 2, cfg.Pool.MaxConcurrent)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/config/ -v -run "Phase2"`
Expected: FAIL (PlannerConfig and PoolConfig don't exist)

**Step 3: Update implementation**

Update `internal/config/config.go` — add PlannerConfig and PoolConfig structs, add them to Config, update Default() and Load() defaults:

```go
type PlannerConfig struct {
	Model   string `yaml:"model"`
	Timeout int    `yaml:"timeout"`
}

type PoolConfig struct {
	MaxConcurrent int `yaml:"max_concurrent"`
}

type Config struct {
	Claude     ClaudeConfig     `yaml:"claude"`
	Governance GovernanceConfig `yaml:"governance"`
	Planner    PlannerConfig    `yaml:"planner"`
	Pool       PoolConfig       `yaml:"pool"`
	BaseDir    string           `yaml:"-"`
}
```

Default() changes:
```go
Claude: ClaudeConfig{
    Model:           "claude-opus-4-6",
    Effort:          "high",
    Timeout:         1800,
    LongTaskTimeout: 7200,
},
Planner: PlannerConfig{
    Model:   "claude-opus-4-6",
    Timeout: 120,
},
Pool: PoolConfig{
    MaxConcurrent: 4,
},
```

Load() zero-value defaults:
```go
if cfg.Planner.Model == "" {
    cfg.Planner.Model = "claude-opus-4-6"
}
if cfg.Planner.Timeout == 0 {
    cfg.Planner.Timeout = 120
}
if cfg.Pool.MaxConcurrent == 0 {
    cfg.Pool.MaxConcurrent = 4
}
```

**Step 4: Fix existing TestDefaultConfig to match new timeout values**

Update `TestDefaultConfig`:
```go
assert.Equal(t, 1800, cfg.Claude.Timeout)
assert.Equal(t, 7200, cfg.Claude.LongTaskTimeout)
```

**Step 5: Run all config tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/config/ -v`
Expected: ALL PASS

**Step 6: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/config/
git commit -m "feat(config): add planner and pool config, increase timeouts

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: DAG Package

**Files:**
- Create: `internal/dag/dag.go`
- Create: `internal/dag/dag_test.go`

**Step 1: Write failing tests**

Create `internal/dag/dag_test.go`:
```go
package dag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDAG(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{"a"}},
		{ID: "c", Task: "task c", Depends: []string{"a"}},
		{ID: "d", Task: "task d", Depends: []string{"b", "c"}},
	}

	d, err := New(nodes)
	require.NoError(t, err)
	assert.Len(t, d.Nodes, 4)
}

func TestNewDAGCycleDetection(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{"b"}},
		{ID: "b", Task: "task b", Depends: []string{"a"}},
	}

	_, err := New(nodes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

func TestNewDAGMissingDependency(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{"nonexistent"}},
	}

	_, err := New(nodes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestReadyNodes(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{"a"}},
		{ID: "c", Task: "task c", Depends: []string{}},
	}

	d, _ := New(nodes)
	ready := d.ReadyNodes()

	// a and c have no deps, should be ready
	ids := readyIDs(ready)
	assert.Contains(t, ids, "a")
	assert.Contains(t, ids, "c")
	assert.NotContains(t, ids, "b")
}

func TestMarkCompleted(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{"a"}},
	}

	d, _ := New(nodes)
	d.MarkRunning("a")
	assert.Equal(t, Running, d.Nodes["a"].Status)

	d.MarkCompleted("a", "done")
	assert.Equal(t, Completed, d.Nodes["a"].Status)
	assert.Equal(t, "done", d.Nodes["a"].Result)

	// b should now be ready
	ready := d.ReadyNodes()
	ids := readyIDs(ready)
	assert.Contains(t, ids, "b")
}

func TestMarkFailedCascade(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{"a"}},
		{ID: "c", Task: "task c", Depends: []string{"b"}},
		{ID: "d", Task: "task d", Depends: []string{}},
	}

	d, _ := New(nodes)
	d.MarkRunning("a")
	d.MarkFailed("a", "error")

	// a failed, b and c should cascade fail
	assert.Equal(t, Failed, d.Nodes["a"].Status)
	assert.Equal(t, Failed, d.Nodes["b"].Status)
	assert.Equal(t, Failed, d.Nodes["c"].Status)

	// d is independent, should still be pending
	assert.Equal(t, Pending, d.Nodes["d"].Status)
}

func TestIsComplete(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{}},
	}

	d, _ := New(nodes)
	assert.False(t, d.IsComplete())

	d.MarkRunning("a")
	d.MarkCompleted("a", "done")
	assert.False(t, d.IsComplete())

	d.MarkRunning("b")
	d.MarkCompleted("b", "done")
	assert.True(t, d.IsComplete())
}

func TestHasFailure(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
	}

	d, _ := New(nodes)
	assert.False(t, d.HasFailure())

	d.MarkRunning("a")
	d.MarkFailed("a", "error")
	assert.True(t, d.HasFailure())
}

func TestSummary(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{"a"}},
	}

	d, _ := New(nodes)
	d.MarkRunning("a")
	d.MarkCompleted("a", "done")

	summary := d.Summary()
	assert.Contains(t, summary, "a")
	assert.Contains(t, summary, "COMPLETED")
	assert.Contains(t, summary, "b")
	assert.Contains(t, summary, "PENDING")
}

func TestSingleNode(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "single", Task: "do one thing", Depends: []string{}},
	}

	d, err := New(nodes)
	require.NoError(t, err)

	ready := d.ReadyNodes()
	assert.Len(t, ready, 1)
	assert.Equal(t, "single", ready[0].ID)
}

func TestEmptyDAG(t *testing.T) {
	_, err := New([]NodeSpec{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

// helper
func readyIDs(nodes []*Node) []string {
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	return ids
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/dag/ -v`
Expected: FAIL (package doesn't exist)

**Step 3: Write implementation**

Create `internal/dag/dag.go`:
```go
package dag

import (
	"fmt"
	"strings"
	"sync"
)

type Status int

const (
	Pending Status = iota
	Running
	Completed
	Failed
)

func (s Status) String() string {
	switch s {
	case Pending:
		return "PENDING"
	case Running:
		return "RUNNING"
	case Completed:
		return "COMPLETED"
	case Failed:
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}

type NodeSpec struct {
	ID      string   `json:"id"`
	Task    string   `json:"task"`
	Depends []string `json:"depends"`
}

type Node struct {
	ID      string
	Task    string
	Depends []string
	Status  Status
	Result  string
	Error   string
}

type DAG struct {
	Nodes map[string]*Node
	mu    sync.Mutex
}

func New(specs []NodeSpec) (*DAG, error) {
	if len(specs) == 0 {
		return nil, fmt.Errorf("cannot create DAG from empty node list")
	}

	nodes := make(map[string]*Node, len(specs))
	for _, s := range specs {
		nodes[s.ID] = &Node{
			ID:      s.ID,
			Task:    s.Task,
			Depends: s.Depends,
			Status:  Pending,
		}
	}

	// Validate dependencies exist
	for _, n := range nodes {
		for _, dep := range n.Depends {
			if _, ok := nodes[dep]; !ok {
				return nil, fmt.Errorf("node %q depends on %q which does not exist", n.ID, dep)
			}
		}
	}

	d := &DAG{Nodes: nodes}

	// Validate no cycles
	if err := d.detectCycles(); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *DAG) detectCycles() error {
	visited := make(map[string]int) // 0=unvisited, 1=visiting, 2=done

	var dfs func(id string) error
	dfs = func(id string) error {
		visited[id] = 1
		for _, dep := range d.Nodes[id].Depends {
			switch visited[dep] {
			case 1:
				return fmt.Errorf("cycle detected involving %q and %q", id, dep)
			case 0:
				if err := dfs(dep); err != nil {
					return err
				}
			}
		}
		visited[id] = 2
		return nil
	}

	for id := range d.Nodes {
		if visited[id] == 0 {
			if err := dfs(id); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *DAG) ReadyNodes() []*Node {
	d.mu.Lock()
	defer d.mu.Unlock()

	var ready []*Node
	for _, n := range d.Nodes {
		if n.Status != Pending {
			continue
		}
		allDepsComplete := true
		for _, dep := range n.Depends {
			if d.Nodes[dep].Status != Completed {
				allDepsComplete = false
				break
			}
		}
		if allDepsComplete {
			ready = append(ready, n)
		}
	}
	return ready
}

func (d *DAG) MarkRunning(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if n, ok := d.Nodes[id]; ok {
		n.Status = Running
	}
}

func (d *DAG) MarkCompleted(id string, result string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if n, ok := d.Nodes[id]; ok {
		n.Status = Completed
		n.Result = result
	}
}

func (d *DAG) MarkFailed(id string, errMsg string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if n, ok := d.Nodes[id]; ok {
		n.Status = Failed
		n.Error = errMsg
	}
	// Cascade fail dependents
	d.cascadeFail(id)
}

func (d *DAG) cascadeFail(failedID string) {
	for _, n := range d.Nodes {
		if n.Status != Pending {
			continue
		}
		for _, dep := range n.Depends {
			if dep == failedID || (d.Nodes[dep] != nil && d.Nodes[dep].Status == Failed) {
				n.Status = Failed
				n.Error = fmt.Sprintf("dependency %q failed", dep)
				d.cascadeFail(n.ID)
				break
			}
		}
	}
}

func (d *DAG) IsComplete() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, n := range d.Nodes {
		if n.Status != Completed && n.Status != Failed {
			return false
		}
	}
	return true
}

func (d *DAG) HasFailure() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, n := range d.Nodes {
		if n.Status == Failed {
			return true
		}
	}
	return false
}

func (d *DAG) Summary() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var lines []string
	for _, n := range d.Nodes {
		line := fmt.Sprintf("  [%s] %s: %s", n.Status, n.ID, n.Task)
		if n.Error != "" {
			line += fmt.Sprintf(" (error: %s)", n.Error)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/dag/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/dag/
git commit -m "feat(dag): add DAG data structure with scheduling and cascade failure

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: Planner Package

**Files:**
- Create: `internal/planner/planner.go`
- Create: `internal/planner/planner_test.go`

**Step 1: Write failing tests**

Create `internal/planner/planner_test.go`:
```go
package planner

import (
	"testing"

	"github.com/lyndonlyu/apex/internal/dag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNodes(t *testing.T) {
	raw := `[{"id":"step1","task":"analyze code","depends":[]},{"id":"step2","task":"refactor","depends":["step1"]}]`

	nodes, err := ParseNodes(raw)
	require.NoError(t, err)
	assert.Len(t, nodes, 2)
	assert.Equal(t, "step1", nodes[0].ID)
	assert.Equal(t, "analyze code", nodes[0].Task)
	assert.Empty(t, nodes[0].Depends)
	assert.Equal(t, "step2", nodes[1].ID)
	assert.Equal(t, []string{"step1"}, nodes[1].Depends)
}

func TestParseNodesInvalid(t *testing.T) {
	_, err := ParseNodes("not json")
	assert.Error(t, err)
}

func TestParseNodesEmpty(t *testing.T) {
	_, err := ParseNodes("[]")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestParseNodesFromWrappedJSON(t *testing.T) {
	// Claude sometimes wraps JSON in markdown code blocks
	raw := "```json\n[{\"id\":\"a\",\"task\":\"do thing\",\"depends\":[]}]\n```"

	nodes, err := ParseNodes(raw)
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
}

func TestIsSimpleTask(t *testing.T) {
	assert.True(t, IsSimpleTask("explain this function"))
	assert.True(t, IsSimpleTask("read the README"))
	assert.True(t, IsSimpleTask("run the tests"))
	assert.False(t, IsSimpleTask("refactor the auth module and then update all tests and deploy"))
	assert.False(t, IsSimpleTask("first analyze the code, then refactor, after that write tests"))
}

func TestBuildPlannerPrompt(t *testing.T) {
	prompt := BuildPlannerPrompt("refactor auth and update tests")
	assert.Contains(t, prompt, "refactor auth and update tests")
	assert.Contains(t, prompt, "JSON")
	assert.Contains(t, prompt, "id")
	assert.Contains(t, prompt, "task")
	assert.Contains(t, prompt, "depends")
}

func TestSingleNodeFallback(t *testing.T) {
	nodes := SingleNodeFallback("simple task")
	assert.Len(t, nodes, 1)
	assert.Equal(t, "task", nodes[0].ID)
	assert.Equal(t, "simple task", nodes[0].Task)
	assert.Empty(t, nodes[0].Depends)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/planner/ -v`
Expected: FAIL

**Step 3: Write implementation**

Create `internal/planner/planner.go`:
```go
package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/lyndonlyu/apex/internal/dag"
	"github.com/lyndonlyu/apex/internal/executor"
)

var complexPattern = regexp.MustCompile(`(?i)\b(and then|then|after that|first .+ then|followed by|next|finally|step \d|phase \d)\b`)

func IsSimpleTask(task string) bool {
	words := strings.Fields(task)
	if len(words) > 50 {
		return false
	}
	return !complexPattern.MatchString(task)
}

func BuildPlannerPrompt(task string) string {
	return fmt.Sprintf(`You are a task planner. Decompose the following task into subtasks.

Return ONLY a JSON array. Each element has:
- "id": short unique identifier (e.g. "step1", "analyze", "test")
- "task": clear description of what to do
- "depends": array of IDs this step depends on (empty if none)

Rules:
- Maximize parallelism: independent steps should not depend on each other
- Keep steps focused: each step should be one clear action
- Minimum steps needed, don't over-decompose
- Return valid JSON only, no markdown, no explanation

Task: %s`, task)
}

func ParseNodes(raw string) ([]dag.NodeSpec, error) {
	// Strip markdown code blocks if present
	cleaned := raw
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(.*?)\\n?```")
	if matches := re.FindStringSubmatch(raw); len(matches) > 1 {
		cleaned = matches[1]
	}
	cleaned = strings.TrimSpace(cleaned)

	var nodes []dag.NodeSpec
	if err := json.Unmarshal([]byte(cleaned), &nodes); err != nil {
		return nil, fmt.Errorf("failed to parse planner output: %w", err)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("planner returned empty node list")
	}

	return nodes, nil
}

func SingleNodeFallback(task string) []dag.NodeSpec {
	return []dag.NodeSpec{
		{ID: "task", Task: task, Depends: []string{}},
	}
}

func Plan(ctx context.Context, exec *executor.Executor, task string, model string, timeout int) ([]dag.NodeSpec, error) {
	if IsSimpleTask(task) {
		return SingleNodeFallback(task), nil
	}

	planExec := executor.New(executor.Options{
		Model:   model,
		Effort:  "high",
		Timeout: time.Duration(timeout) * time.Second,
	})

	prompt := BuildPlannerPrompt(task)
	result, err := planExec.Run(ctx, prompt)
	if err != nil {
		// Fallback to single node on planner failure
		return SingleNodeFallback(task), nil
	}

	nodes, err := ParseNodes(result.Output)
	if err != nil {
		// Fallback to single node on parse failure
		return SingleNodeFallback(task), nil
	}

	return nodes, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/planner/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/planner/
git commit -m "feat(planner): add LLM task decomposition with JSON parsing and fallback

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Pool Package

**Files:**
- Create: `internal/pool/pool.go`
- Create: `internal/pool/pool_test.go`

**Step 1: Write failing tests**

Create `internal/pool/pool_test.go`:
```go
package pool

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lyndonlyu/apex/internal/dag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRunner implements Runner interface for testing
type mockRunner struct {
	callCount atomic.Int32
	delay     time.Duration
	failIDs   map[string]bool
}

func (m *mockRunner) RunTask(ctx context.Context, task string) (string, error) {
	m.callCount.Add(1)
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return "result for: " + task, nil
}

type failRunner struct {
	failIDs map[string]bool
}

func (f *failRunner) RunTask(ctx context.Context, task string) (string, error) {
	if f.failIDs != nil {
		// We can't match on task directly since pool passes task string
		// Instead fail on any task containing certain keywords
		for keyword := range f.failIDs {
			if keyword != "" && contains(task, keyword) {
				return "", assert.AnError
			}
		}
	}
	return "ok", nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && containsStr(s, substr)))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestNewPool(t *testing.T) {
	runner := &mockRunner{}
	p := New(4, runner)
	assert.NotNil(t, p)
}

func TestExecuteLinearDAG(t *testing.T) {
	nodes := []dag.NodeSpec{
		{ID: "a", Task: "first", Depends: []string{}},
		{ID: "b", Task: "second", Depends: []string{"a"}},
		{ID: "c", Task: "third", Depends: []string{"b"}},
	}
	d, _ := dag.New(nodes)
	runner := &mockRunner{}
	p := New(4, runner)

	err := p.Execute(context.Background(), d)
	require.NoError(t, err)

	assert.Equal(t, dag.Completed, d.Nodes["a"].Status)
	assert.Equal(t, dag.Completed, d.Nodes["b"].Status)
	assert.Equal(t, dag.Completed, d.Nodes["c"].Status)
	assert.Equal(t, int32(3), runner.callCount.Load())
}

func TestExecuteParallelDAG(t *testing.T) {
	nodes := []dag.NodeSpec{
		{ID: "a", Task: "parallel 1", Depends: []string{}},
		{ID: "b", Task: "parallel 2", Depends: []string{}},
		{ID: "c", Task: "parallel 3", Depends: []string{}},
		{ID: "d", Task: "final", Depends: []string{"a", "b", "c"}},
	}
	d, _ := dag.New(nodes)

	runner := &mockRunner{delay: 50 * time.Millisecond}
	p := New(4, runner)

	start := time.Now()
	err := p.Execute(context.Background(), d)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.True(t, d.IsComplete())
	assert.False(t, d.HasFailure())
	// Parallel execution should be faster than 4 * 50ms = 200ms
	assert.True(t, duration < 300*time.Millisecond, "parallel execution too slow: %v", duration)
}

func TestExecuteWithFailure(t *testing.T) {
	nodes := []dag.NodeSpec{
		{ID: "a", Task: "will fail", Depends: []string{}},
		{ID: "b", Task: "depends on a", Depends: []string{"a"}},
		{ID: "c", Task: "independent", Depends: []string{}},
	}
	d, _ := dag.New(nodes)

	runner := &failRunner{failIDs: map[string]bool{"will fail": true}}
	p := New(4, runner)

	err := p.Execute(context.Background(), d)
	// Execute completes even with failures
	assert.NoError(t, err)
	assert.True(t, d.HasFailure())
	assert.Equal(t, dag.Failed, d.Nodes["a"].Status)
	assert.Equal(t, dag.Failed, d.Nodes["b"].Status) // cascade
	assert.Equal(t, dag.Completed, d.Nodes["c"].Status) // independent
}

func TestExecuteSingleNode(t *testing.T) {
	nodes := []dag.NodeSpec{
		{ID: "only", Task: "single task", Depends: []string{}},
	}
	d, _ := dag.New(nodes)
	runner := &mockRunner{}
	p := New(1, runner)

	err := p.Execute(context.Background(), d)
	require.NoError(t, err)
	assert.Equal(t, dag.Completed, d.Nodes["only"].Status)
}

func TestExecuteContextCancel(t *testing.T) {
	nodes := []dag.NodeSpec{
		{ID: "slow", Task: "slow task", Depends: []string{}},
	}
	d, _ := dag.New(nodes)
	runner := &mockRunner{delay: 5 * time.Second}
	p := New(1, runner)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := p.Execute(ctx, d)
	assert.Error(t, err)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/pool/ -v`
Expected: FAIL

**Step 3: Write implementation**

Create `internal/pool/pool.go`:
```go
package pool

import (
	"context"
	"fmt"
	"sync"

	"github.com/lyndonlyu/apex/internal/dag"
)

// Runner executes a single task and returns the result.
type Runner interface {
	RunTask(ctx context.Context, task string) (string, error)
}

type Pool struct {
	maxWorkers int
	runner     Runner
}

func New(maxWorkers int, runner Runner) *Pool {
	if maxWorkers <= 0 {
		maxWorkers = 4
	}
	return &Pool{
		maxWorkers: maxWorkers,
		runner:     runner,
	}
}

func (p *Pool) Execute(ctx context.Context, d *dag.DAG) error {
	for !d.IsComplete() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		ready := d.ReadyNodes()
		if len(ready) == 0 {
			// No ready nodes but DAG not complete — everything is running or blocked
			// Brief sleep to avoid busy loop, then retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				// Check again
				if d.IsComplete() {
					break
				}
				// Wait for any running node to finish
				continue
			}
		}

		// Limit concurrent workers
		sem := make(chan struct{}, p.maxWorkers)
		var wg sync.WaitGroup

		for _, node := range ready {
			d.MarkRunning(node.ID)
			sem <- struct{}{}
			wg.Add(1)

			go func(n *dag.Node) {
				defer wg.Done()
				defer func() { <-sem }()

				result, err := p.runner.RunTask(ctx, n.Task)
				if err != nil {
					d.MarkFailed(n.ID, err.Error())
					return
				}
				d.MarkCompleted(n.ID, result)
			}(node)
		}

		wg.Wait()
	}

	if d.HasFailure() {
		return nil // Partial failure, caller checks DAG state
	}
	return nil
}

// ExecutorRunner wraps executor.Executor to implement Runner.
type ExecutorRunner struct {
	Exec interface {
		Run(ctx context.Context, task string) (interface{ GetOutput() string }, error)
	}
}

func (e *ExecutorRunner) RunTask(ctx context.Context, task string) (string, error) {
	result, err := e.Exec.Run(ctx, task)
	if err != nil {
		return "", err
	}
	return result.GetOutput(), nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/pool/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/pool/
git commit -m "feat(pool): add concurrent worker pool for DAG execution

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 5: Wire Phase 2 into CLI

**Files:**
- Modify: `cmd/apex/run.go` — integrate planner + DAG + pool
- Create: `cmd/apex/plan.go` — new `apex plan` command
- Modify: `cmd/apex/main.go` — register plan command
- Create: `internal/pool/executor_runner.go` — adapter connecting Executor to Runner interface

**Step 1: Create executor_runner adapter**

Create `internal/pool/executor_runner.go`:
```go
package pool

import (
	"context"

	"github.com/lyndonlyu/apex/internal/executor"
)

type ClaudeRunner struct {
	Executor *executor.Executor
}

func NewClaudeRunner(exec *executor.Executor) *ClaudeRunner {
	return &ClaudeRunner{Executor: exec}
}

func (r *ClaudeRunner) RunTask(ctx context.Context, task string) (string, error) {
	result, err := r.Executor.Run(ctx, task)
	if err != nil {
		return "", err
	}
	return result.Output, nil
}
```

**Step 2: Create `cmd/apex/plan.go`**

```go
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lyndonlyu/apex/internal/config"
	"github.com/lyndonlyu/apex/internal/dag"
	"github.com/lyndonlyu/apex/internal/executor"
	"github.com/lyndonlyu/apex/internal/planner"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan [task]",
	Short: "Preview task decomposition without executing",
	Long:  "Decompose a task into a DAG and display the execution plan without running it.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  planTask,
}

func planTask(cmd *cobra.Command, args []string) error {
	task := args[0]

	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".apex", "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	exec := executor.New(executor.Options{
		Model:   cfg.Planner.Model,
		Effort:  "high",
		Timeout: time.Duration(cfg.Planner.Timeout) * time.Second,
	})

	fmt.Println("🔍 Analyzing task...")
	nodes, err := planner.Plan(context.Background(), exec, task, cfg.Planner.Model, cfg.Planner.Timeout)
	if err != nil {
		return fmt.Errorf("planning failed: %w", err)
	}

	d, err := dag.New(nodes)
	if err != nil {
		return fmt.Errorf("invalid DAG: %w", err)
	}

	fmt.Printf("\n📋 Execution Plan (%d steps):\n\n", len(d.Nodes))
	for _, n := range nodes {
		deps := "none"
		if len(n.Depends) > 0 {
			deps = fmt.Sprintf("%v", n.Depends)
		}
		fmt.Printf("  [%s] %s\n", n.ID, n.Task)
		fmt.Printf("        depends: %s\n\n", deps)
	}

	return nil
}
```

**Step 3: Update `cmd/apex/run.go` for DAG execution**

Replace `runTask` function to integrate planner + DAG + pool. The updated `run.go` should:
1. Load config
2. Governance check
3. Call `planner.Plan()` to decompose
4. Build DAG
5. Create Pool with ClaudeRunner
6. Execute DAG
7. Audit each node + overall
8. Save to memory

**Step 4: Register plan command in `cmd/apex/main.go`**

Add `rootCmd.AddCommand(planCmd)` in init().

**Step 5: Run all tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./... -v`
Expected: ALL PASS

**Step 6: Build and verify**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
make build
./bin/apex plan --help
./bin/apex run --help
```
Expected: Both commands show help text

**Step 7: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add cmd/apex/ internal/pool/executor_runner.go
git commit -m "feat: integrate DAG orchestration into CLI (plan + run commands)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 6: End-to-End Verification

**Step 1: Run full test suite**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./... -v -count=1`
Expected: ALL PASS

**Step 2: Build**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && make build`

**Step 3: Verify all commands**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
./bin/apex version
./bin/apex --help
./bin/apex plan --help
./bin/apex run --help
./bin/apex history --help
./bin/apex memory search --help
```

**Step 4: Final commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add -A
git commit -m "feat: Phase 2 complete - DAG orchestration with concurrent agent pool

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

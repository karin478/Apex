# Phase 13: Adversarial Review — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `apex review <proposal>` command that runs a 4-step Advocate/Critic/Judge debate protocol via Claude.

**Architecture:** New `internal/reasoning` package with `RunReview()` function that sequentially calls the executor 4 times with role-specific prompts. Results saved as JSON to `~/.apex/reviews/`. CLI command wired into Cobra.

**Tech Stack:** Go, Cobra CLI, existing `internal/executor` + `internal/audit`, Testify, `github.com/google/uuid` (already in go.mod)

---

### Task 1: Review Data Model + Save/Load

**Files:**
- Create: `internal/reasoning/review.go`
- Create: `internal/reasoning/review_test.go`

**Step 1: Write the failing test**

Create `internal/reasoning/review_test.go`:

```go
package reasoning

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndLoadReview(t *testing.T) {
	dir := t.TempDir()

	result := &ReviewResult{
		ID:         "test-id-123",
		Proposal:   "Use Redis for caching",
		CreatedAt:  "2026-02-19T12:00:00Z",
		Steps: []Step{
			{Role: "advocate", Prompt: "p1", Output: "o1"},
			{Role: "critic", Prompt: "p2", Output: "o2"},
			{Role: "advocate", Prompt: "p3", Output: "o3"},
			{Role: "judge", Prompt: "p4", Output: "o4"},
		},
		Verdict: Verdict{
			Decision: "approve",
			Summary:  "Good proposal",
			Risks:    []string{"risk1"},
			Actions:  []string{"action1"},
		},
		DurationMs: 5000,
	}

	err := SaveReview(dir, result)
	require.NoError(t, err)

	// File should exist with correct name
	path := filepath.Join(dir, "test-id-123.json")
	assert.FileExists(t, path)

	// Permissions should be 0600
	info, _ := os.Stat(path)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	// Load and verify round-trip
	loaded, err := LoadReview(dir, "test-id-123")
	require.NoError(t, err)
	assert.Equal(t, result.Proposal, loaded.Proposal)
	assert.Equal(t, result.Verdict.Decision, loaded.Verdict.Decision)
	assert.Len(t, loaded.Steps, 4)
	assert.Equal(t, "advocate", loaded.Steps[0].Role)
}

func TestLoadReviewNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadReview(dir, "nonexistent")
	assert.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/reasoning/... -run "TestSaveAndLoadReview|TestLoadReviewNotFound" -v`
Expected: FAIL — package does not exist

**Step 3: Write minimal implementation**

Create `internal/reasoning/review.go`:

```go
package reasoning

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Step struct {
	Role   string `json:"role"`
	Prompt string `json:"prompt"`
	Output string `json:"output"`
}

type Verdict struct {
	Decision string   `json:"decision"`
	Summary  string   `json:"summary"`
	Risks    []string `json:"risks"`
	Actions  []string `json:"suggested_actions"`
}

type ReviewResult struct {
	ID         string  `json:"id"`
	Proposal   string  `json:"proposal"`
	CreatedAt  string  `json:"created_at"`
	Steps      []Step  `json:"steps"`
	Verdict    Verdict `json:"verdict"`
	DurationMs int64   `json:"duration_ms"`
}

func SaveReview(dir string, result *ReviewResult) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, result.ID+".json")
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadReview(dir, id string) (*ReviewResult, error) {
	path := filepath.Join(dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load review %s: %w", id, err)
	}
	var result ReviewResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse review %s: %w", id, err)
	}
	return &result, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/reasoning/... -run "TestSaveAndLoadReview|TestLoadReviewNotFound" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/reasoning/review.go internal/reasoning/review_test.go
git commit -m "feat(reasoning): add ReviewResult data model with Save/Load"
```

---

### Task 2: Verdict Parsing from Judge Output

**Files:**
- Modify: `internal/reasoning/review.go`
- Modify: `internal/reasoning/review_test.go`

**Step 1: Write the failing test**

Add to `internal/reasoning/review_test.go`:

```go
func TestParseVerdictCleanJSON(t *testing.T) {
	input := `{"decision":"approve","summary":"Good idea","risks":["risk1"],"suggested_actions":["action1"]}`
	v, err := parseVerdict(input)
	require.NoError(t, err)
	assert.Equal(t, "approve", v.Decision)
	assert.Equal(t, "Good idea", v.Summary)
	assert.Equal(t, []string{"risk1"}, v.Risks)
	assert.Equal(t, []string{"action1"}, v.Actions)
}

func TestParseVerdictMarkdownWrapped(t *testing.T) {
	input := "Here is my verdict:\n```json\n{\"decision\":\"reject\",\"summary\":\"Too risky\",\"risks\":[\"r1\",\"r2\"],\"suggested_actions\":[\"a1\"]}\n```\nEnd of review."
	v, err := parseVerdict(input)
	require.NoError(t, err)
	assert.Equal(t, "reject", v.Decision)
	assert.Equal(t, []string{"r1", "r2"}, v.Risks)
}

func TestParseVerdictMalformed(t *testing.T) {
	input := "I think this is a good idea but I can't decide."
	v, err := parseVerdict(input)
	require.NoError(t, err)
	// Fallback: decision="revise", summary=full text
	assert.Equal(t, "revise", v.Decision)
	assert.Contains(t, v.Summary, "good idea")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/reasoning/... -run "TestParseVerdict" -v`
Expected: FAIL — parseVerdict not defined

**Step 3: Write minimal implementation**

Add to `internal/reasoning/review.go`:

```go
import (
	"regexp"
	"strings"
)
```

(merge with existing imports)

```go
var jsonBlockRe = regexp.MustCompile("(?s)```(?:json)?\\s*\\n(\\{.*?\\})\\s*\\n```")

func parseVerdict(output string) (Verdict, error) {
	var v Verdict

	// Try direct JSON parse
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &v); err == nil && v.Decision != "" {
		return v, nil
	}

	// Try extracting from markdown code block
	if matches := jsonBlockRe.FindStringSubmatch(output); len(matches) > 1 {
		if err := json.Unmarshal([]byte(matches[1]), &v); err == nil && v.Decision != "" {
			return v, nil
		}
	}

	// Fallback: use full output as summary
	return Verdict{
		Decision: "revise",
		Summary:  strings.TrimSpace(output),
	}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/reasoning/... -run "TestParseVerdict" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/reasoning/review.go internal/reasoning/review_test.go
git commit -m "feat(reasoning): add verdict JSON parsing with markdown fallback"
```

---

### Task 3: RunReview Core Logic

**Files:**
- Modify: `internal/reasoning/review.go`
- Modify: `internal/reasoning/review_test.go`

**Step 1: Write the failing test**

Add to `internal/reasoning/review_test.go`:

```go
import (
	"context"
	"time"
)
```

(merge with existing imports)

```go
// mockRunner implements reasoning.Runner for testing.
type mockRunner struct {
	responses []string
	calls     []string
	callIndex int
}

func (m *mockRunner) RunTask(ctx context.Context, task string) (string, error) {
	m.calls = append(m.calls, task)
	if m.callIndex < len(m.responses) {
		resp := m.responses[m.callIndex]
		m.callIndex++
		return resp, nil
	}
	return "fallback response", nil
}

func TestRunReview(t *testing.T) {
	mock := &mockRunner{
		responses: []string{
			"Redis is great for caching because...",
			"However, Redis has these risks...",
			"Let me address those concerns...",
			`{"decision":"approve","summary":"Redis is viable","risks":["single point of failure"],"suggested_actions":["add sentinel"]}`,
		},
	}

	result, err := RunReview(context.Background(), mock, "Use Redis for caching")
	require.NoError(t, err)

	assert.Equal(t, "Use Redis for caching", result.Proposal)
	assert.NotEmpty(t, result.ID)
	assert.NotEmpty(t, result.CreatedAt)
	require.Len(t, result.Steps, 4)

	// Verify roles
	assert.Equal(t, "advocate", result.Steps[0].Role)
	assert.Equal(t, "critic", result.Steps[1].Role)
	assert.Equal(t, "advocate", result.Steps[2].Role)
	assert.Equal(t, "judge", result.Steps[3].Role)

	// Verify outputs
	assert.Contains(t, result.Steps[0].Output, "Redis is great")
	assert.Contains(t, result.Steps[1].Output, "risks")

	// Verify verdict parsed
	assert.Equal(t, "approve", result.Verdict.Decision)
	assert.Equal(t, "Redis is viable", result.Verdict.Summary)
	assert.Equal(t, []string{"single point of failure"}, result.Verdict.Risks)

	// Verify 4 calls were made to the runner
	assert.Len(t, mock.calls, 4)

	// Verify context was passed through (advocate prompt includes proposal)
	assert.Contains(t, mock.calls[0], "Use Redis for caching")
	// Critic prompt includes advocate output
	assert.Contains(t, mock.calls[1], "Redis is great")
	// Advocate response includes critic output
	assert.Contains(t, mock.calls[2], "risks")
	// Judge prompt includes all previous outputs
	assert.Contains(t, mock.calls[3], "Redis is great")
	assert.Contains(t, mock.calls[3], "risks")
	assert.Contains(t, mock.calls[3], "address those concerns")
}

func TestRunReviewExecutorError(t *testing.T) {
	mock := &mockRunner{
		responses: []string{"advocate output"},
	}
	// Only 1 response — second call will use fallback but let's test with an erroring runner
	errRunner := &errorRunner{failAt: 1}
	_, err := RunReview(context.Background(), errRunner, "test proposal")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "critic")
}

type errorRunner struct {
	callCount int
	failAt    int
}

func (e *errorRunner) RunTask(ctx context.Context, task string) (string, error) {
	e.callCount++
	if e.callCount > e.failAt {
		return "", fmt.Errorf("executor failed")
	}
	return "output", nil
}
```

Add `"fmt"` to imports.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/reasoning/... -run "TestRunReview" -v`
Expected: FAIL — RunReview and Runner not defined

**Step 3: Write minimal implementation**

Add to `internal/reasoning/review.go`:

```go
import (
	"context"
	"time"

	"github.com/google/uuid"
)
```

(merge with existing imports)

```go
// Runner executes a single task prompt and returns the text output.
type Runner interface {
	RunTask(ctx context.Context, task string) (string, error)
}

const (
	advocateSystem = "You are a technical advocate. Present the strongest case FOR this proposal. Focus on benefits, feasibility, and alignment with goals. Be specific and concrete."
	criticSystem   = "You are a technical critic. Challenge the proposal and the advocate's arguments. Identify risks, blind spots, missing considerations, and better alternatives. Be thorough."
	responseSystem = "You are the original advocate. The critic has raised challenges to your proposal. Address each concern point-by-point. Acknowledge valid criticisms and explain mitigations."
	judgeSystem    = `You are an impartial technical judge. Review the full debate transcript and deliver a final verdict. You MUST respond with ONLY a JSON object in this exact format:
{"decision":"approve|reject|revise","summary":"1-2 sentence verdict","risks":["risk1","risk2"],"suggested_actions":["action1","action2"]}
Do not include any text before or after the JSON.`
)

func RunReview(ctx context.Context, runner Runner, proposal string) (*ReviewResult, error) {
	start := time.Now()
	result := &ReviewResult{
		ID:        uuid.New().String(),
		Proposal:  proposal,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Step 1: Advocate
	advocatePrompt := fmt.Sprintf("%s\n\n## Proposal\n%s", advocateSystem, proposal)
	advocateOut, err := runner.RunTask(ctx, advocatePrompt)
	if err != nil {
		return nil, fmt.Errorf("advocate step failed: %w", err)
	}
	result.Steps = append(result.Steps, Step{Role: "advocate", Prompt: advocatePrompt, Output: advocateOut})

	// Step 2: Critic
	criticPrompt := fmt.Sprintf("%s\n\n## Proposal\n%s\n\n## Advocate's Argument\n%s", criticSystem, proposal, advocateOut)
	criticOut, err := runner.RunTask(ctx, criticPrompt)
	if err != nil {
		return nil, fmt.Errorf("critic step failed: %w", err)
	}
	result.Steps = append(result.Steps, Step{Role: "critic", Prompt: criticPrompt, Output: criticOut})

	// Step 3: Advocate Response
	responsePrompt := fmt.Sprintf("%s\n\n## Proposal\n%s\n\n## Critic's Challenges\n%s", responseSystem, proposal, criticOut)
	responseOut, err := runner.RunTask(ctx, responsePrompt)
	if err != nil {
		return nil, fmt.Errorf("advocate response step failed: %w", err)
	}
	result.Steps = append(result.Steps, Step{Role: "advocate", Prompt: responsePrompt, Output: responseOut})

	// Step 4: Judge
	judgePrompt := fmt.Sprintf("%s\n\n## Proposal\n%s\n\n## Advocate's Argument\n%s\n\n## Critic's Challenges\n%s\n\n## Advocate's Response\n%s",
		judgeSystem, proposal, advocateOut, criticOut, responseOut)
	judgeOut, err := runner.RunTask(ctx, judgePrompt)
	if err != nil {
		return nil, fmt.Errorf("judge step failed: %w", err)
	}
	result.Steps = append(result.Steps, Step{Role: "judge", Prompt: judgePrompt, Output: judgeOut})

	// Parse verdict
	verdict, err := parseVerdict(judgeOut)
	if err != nil {
		return nil, fmt.Errorf("parse verdict: %w", err)
	}
	result.Verdict = verdict
	result.DurationMs = time.Since(start).Milliseconds()

	return result, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/reasoning/... -run "TestRunReview" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/reasoning/review.go internal/reasoning/review_test.go
git commit -m "feat(reasoning): add RunReview 4-step debate protocol"
```

---

### Task 4: CLI Command `apex review`

**Files:**
- Create: `cmd/apex/review.go`
- Modify: `cmd/apex/main.go` (add `reviewCmd` registration)

**Step 1: Write the implementation**

Create `cmd/apex/review.go`:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lyndonlyu/apex/internal/audit"
	"github.com/lyndonlyu/apex/internal/config"
	"github.com/lyndonlyu/apex/internal/executor"
	"github.com/lyndonlyu/apex/internal/reasoning"
	"github.com/spf13/cobra"
)

var reviewCmd = &cobra.Command{
	Use:   "review [proposal]",
	Short: "Run adversarial review on a technical proposal",
	Long:  "Subject a technical proposal to a structured Advocate/Critic/Judge debate.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runReview,
}

// executorRunner adapts executor.Executor to reasoning.Runner.
type executorRunner struct {
	exec *executor.Executor
}

func (r *executorRunner) RunTask(ctx context.Context, task string) (string, error) {
	result, err := r.exec.Run(ctx, task)
	if err != nil {
		return "", err
	}
	return result.Output, nil
}

func runReview(cmd *cobra.Command, args []string) error {
	proposal := args[0]

	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".apex", "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	exec := executor.New(executor.Options{
		Model:   cfg.Claude.Model,
		Effort:  cfg.Claude.Effort,
		Timeout: time.Duration(cfg.Claude.Timeout) * time.Second,
		Binary:  cfg.Claude.Binary,
	})

	runner := &executorRunner{exec: exec}

	fmt.Println("Adversarial Review")
	fmt.Println("==================")
	fmt.Println()

	roles := []string{"Advocate", "Critic", "Response", "Judge"}
	dots := []string{"...", ".....", "...", "......"}

	ctx := context.Background()
	start := time.Now()

	// Run review with progress output
	result, err := reasoning.RunReviewWithProgress(ctx, runner, proposal, func(step int, dur time.Duration) {
		fmt.Printf("%-11s%s done (%.1fs)\n", roles[step], dots[step], dur.Seconds())
	})
	if err != nil {
		return fmt.Errorf("review failed: %w", err)
	}

	totalDur := time.Since(start)

	fmt.Println()
	fmt.Printf("Verdict: %s\n", formatDecision(result.Verdict.Decision))
	fmt.Printf("Summary: %s\n", result.Verdict.Summary)

	if len(result.Verdict.Risks) > 0 {
		fmt.Println()
		fmt.Println("Risks:")
		for i, r := range result.Verdict.Risks {
			fmt.Printf("  %d. %s\n", i+1, r)
		}
	}

	if len(result.Verdict.Actions) > 0 {
		fmt.Println()
		fmt.Println("Suggested Actions:")
		for i, a := range result.Verdict.Actions {
			fmt.Printf("  %d. %s\n", i+1, a)
		}
	}

	// Save review
	reviewsDir := filepath.Join(home, ".apex", "reviews")
	if err := reasoning.SaveReview(reviewsDir, result); err != nil {
		fmt.Fprintf(os.Stderr, "warning: save review failed: %v\n", err)
	} else {
		fmt.Printf("\nFull report: %s/%s.json\n", reviewsDir, result.ID)
	}

	// Audit log
	auditDir := filepath.Join(home, ".apex", "audit")
	logger, logErr := audit.NewLogger(auditDir)
	if logErr == nil {
		truncated := proposal
		if len(truncated) > 80 {
			truncated = truncated[:80]
		}
		logger.Log(audit.Entry{
			Task:      "review: " + truncated,
			RiskLevel: "LOW",
			Outcome:   result.Verdict.Decision,
			Duration:  totalDur,
			Model:     cfg.Claude.Model,
		})
	}

	return nil
}

func formatDecision(d string) string {
	switch d {
	case "approve":
		return "APPROVE"
	case "reject":
		return "REJECT"
	case "revise":
		return "REVISE"
	default:
		return d
	}
}
```

**Step 2: Add RunReviewWithProgress to `internal/reasoning/review.go`**

Add after `RunReview`:

```go
// ProgressFunc is called after each debate step with the step index and duration.
type ProgressFunc func(step int, duration time.Duration)

// RunReviewWithProgress is like RunReview but calls progress after each step.
func RunReviewWithProgress(ctx context.Context, runner Runner, proposal string, progress ProgressFunc) (*ReviewResult, error) {
	start := time.Now()
	result := &ReviewResult{
		ID:        uuid.New().String(),
		Proposal:  proposal,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	type stepDef struct {
		role   string
		prompt func() string
	}

	var advocateOut, criticOut, responseOut string

	steps := []stepDef{
		{"advocate", func() string {
			return fmt.Sprintf("%s\n\n## Proposal\n%s", advocateSystem, proposal)
		}},
		{"critic", func() string {
			return fmt.Sprintf("%s\n\n## Proposal\n%s\n\n## Advocate's Argument\n%s", criticSystem, proposal, advocateOut)
		}},
		{"advocate", func() string {
			return fmt.Sprintf("%s\n\n## Proposal\n%s\n\n## Critic's Challenges\n%s", responseSystem, proposal, criticOut)
		}},
		{"judge", func() string {
			return fmt.Sprintf("%s\n\n## Proposal\n%s\n\n## Advocate's Argument\n%s\n\n## Critic's Challenges\n%s\n\n## Advocate's Response\n%s",
				judgeSystem, proposal, advocateOut, criticOut, responseOut)
		}},
	}

	for i, s := range steps {
		stepStart := time.Now()
		prompt := s.prompt()
		out, err := runner.RunTask(ctx, prompt)
		if err != nil {
			return nil, fmt.Errorf("%s step failed: %w", s.role, err)
		}
		result.Steps = append(result.Steps, Step{Role: s.role, Prompt: prompt, Output: out})

		switch i {
		case 0:
			advocateOut = out
		case 1:
			criticOut = out
		case 2:
			responseOut = out
		}

		if progress != nil {
			progress(i, time.Since(stepStart))
		}
	}

	verdict, err := parseVerdict(result.Steps[3].Output)
	if err != nil {
		return nil, fmt.Errorf("parse verdict: %w", err)
	}
	result.Verdict = verdict
	result.DurationMs = time.Since(start).Milliseconds()

	return result, nil
}
```

**Step 3: Register command in `cmd/apex/main.go`**

Add `rootCmd.AddCommand(reviewCmd)` to the `init()` function.

**Step 4: Verify compilation**

Run: `go build ./cmd/apex/`
Expected: success

**Step 5: Commit**

```bash
git add cmd/apex/review.go cmd/apex/main.go internal/reasoning/review.go
git commit -m "feat(cli): add apex review command with progress output"
```

---

### Task 5: E2E Tests

**Files:**
- Create: `e2e/review_test.go`

**Step 1: Write the E2E tests**

Create `e2e/review_test.go`:

```go
package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewHappyPath(t *testing.T) {
	env := newTestEnv(t)

	// The mock returns '{"result":"mock ok"}' for each step.
	// The judge output won't be valid JSON verdict, so it falls back to "revise".
	stdout, stderr, code := env.runApex("review", "Use Redis for caching")
	require.Equal(t, 0, code, "apex review should succeed; stdout=%s stderr=%s", stdout, stderr)

	assert.Contains(t, stdout, "Adversarial Review")
	assert.Contains(t, stdout, "Verdict:")
	assert.Contains(t, stdout, "Full report:")

	// Review file should be saved
	reviewsDir := filepath.Join(env.Home, ".apex", "reviews")
	entries, err := os.ReadDir(reviewsDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.True(t, strings.HasSuffix(entries[0].Name(), ".json"))
}

func TestReviewNoArgs(t *testing.T) {
	env := newTestEnv(t)
	_, stderr, code := env.runApex("review")
	assert.NotEqual(t, 0, code)
	assert.Contains(t, stderr, "requires at least 1 arg")
}

func TestReviewCreatesAuditEntry(t *testing.T) {
	env := newTestEnv(t)

	env.runApex("review", "Test proposal for audit")

	// Check audit directory has a file for today
	auditDir := env.auditDir()
	entries, err := os.ReadDir(auditDir)
	require.NoError(t, err)
	assert.Greater(t, len(entries), 0, "audit directory should have at least one file")
}
```

**Step 2: Run tests**

Run: `go test ./e2e/... -v -count=1 -timeout=120s`
Expected: ALL tests pass (33 existing + 3 new = 36 total)

**Step 3: Commit**

```bash
git add e2e/review_test.go
git commit -m "test(e2e): add adversarial review E2E tests"
```

---

### Task 6: Update PROGRESS.md

**Files:**
- Modify: `PROGRESS.md`

**Step 1: Update progress**

- Change Phase 13 row: `| 13 | Adversarial Review | \`2026-02-19-phase13-adversarial-review-design.md\` | Done |`
- Update "Current: Phase 14 — TBD"
- Update E2E test count to 36
- Add `internal/reasoning` to Key Packages: `| \`internal/reasoning\` | Adversarial review debate protocol |`

**Step 2: Commit**

```bash
git add PROGRESS.md
git commit -m "docs: mark Phase 13 Adversarial Review as complete"
```

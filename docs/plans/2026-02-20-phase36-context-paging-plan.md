# Context Paging Tool Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an `internal/paging` package that provides on-demand artifact content retrieval with per-task token budget enforcement.

**Architecture:** ContentStore interface for decoupled content access, Budget struct for quota tracking, Pager that orchestrates fetch → line extraction → token estimation → budget enforcement. Format functions + Cobra CLI.

**Tech Stack:** Go, `encoding/json`, Testify, Cobra CLI

---

### Task 1: Context Paging Core — Budget + Pager + EstimateTokens (7 tests)

**Files:**
- Create: `internal/paging/paging.go`
- Create: `internal/paging/paging_test.go`

**Step 1: Write 7 failing tests**

```go
package paging

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultBudget(t *testing.T) {
	b := DefaultBudget()
	assert.Equal(t, 10, b.MaxPages)
	assert.Equal(t, 8000, b.MaxTokens)
	assert.Equal(t, 0, b.PagesUsed)
	assert.Equal(t, 0, b.TokensUsed)
}

func TestNewBudget(t *testing.T) {
	b := NewBudget(5, 4000)
	assert.Equal(t, 5, b.MaxPages)
	assert.Equal(t, 4000, b.MaxTokens)
}

func TestBudgetCanPage(t *testing.T) {
	t.Run("fresh budget", func(t *testing.T) {
		b := DefaultBudget()
		assert.True(t, b.CanPage())
	})

	t.Run("pages exhausted", func(t *testing.T) {
		b := NewBudget(1, 8000)
		b.Record(100)
		assert.False(t, b.CanPage())
	})

	t.Run("tokens exhausted", func(t *testing.T) {
		b := NewBudget(10, 100)
		b.Record(100)
		assert.False(t, b.CanPage())
	})
}

func TestBudgetRecord(t *testing.T) {
	b := DefaultBudget()
	b.Record(500)
	assert.Equal(t, 1, b.PagesUsed)
	assert.Equal(t, 500, b.TokensUsed)

	b.Record(300)
	assert.Equal(t, 2, b.PagesUsed)
	assert.Equal(t, 800, b.TokensUsed)

	pages, tokens := b.Remaining()
	assert.Equal(t, 8, pages)
	assert.Equal(t, 7200, tokens)
}

func TestEstimateTokens(t *testing.T) {
	assert.Equal(t, 0, EstimateTokens(""))
	assert.Equal(t, 1, EstimateTokens("abcd"))
	assert.Equal(t, 3, EstimateTokens("hello world!"))
	assert.Equal(t, 25, EstimateTokens(strings.Repeat("x", 100)))
}

func TestPagerPage(t *testing.T) {
	store := &mockStore{
		data: map[string]string{
			"art-1": "line1\nline2\nline3\nline4\nline5",
		},
	}
	pager := NewPager(store, DefaultBudget())

	t.Run("full content", func(t *testing.T) {
		result, err := pager.Page(PageRequest{ArtifactID: "art-1"})
		require.NoError(t, err)
		assert.Equal(t, "art-1", result.ArtifactID)
		assert.Equal(t, 5, result.Lines)
		assert.Contains(t, result.Content, "line1")
		assert.Contains(t, result.Content, "line5")
		assert.Greater(t, result.Tokens, 0)
	})

	t.Run("line range", func(t *testing.T) {
		result, err := pager.Page(PageRequest{
			ArtifactID: "art-1",
			StartLine:  2,
			EndLine:    4,
		})
		require.NoError(t, err)
		assert.Equal(t, 3, result.Lines)
		assert.Contains(t, result.Content, "line2")
		assert.Contains(t, result.Content, "line4")
		assert.NotContains(t, result.Content, "line1")
		assert.NotContains(t, result.Content, "line5")
	})

	t.Run("not found", func(t *testing.T) {
		_, err := pager.Page(PageRequest{ArtifactID: "nope"})
		assert.Error(t, err)
	})
}

func TestPagerPageBudgetExhausted(t *testing.T) {
	store := &mockStore{
		data: map[string]string{
			"art-1": "content",
		},
	}
	budget := NewBudget(1, 8000)

	pager := NewPager(store, budget)

	// First page succeeds.
	_, err := pager.Page(PageRequest{ArtifactID: "art-1"})
	require.NoError(t, err)

	// Second page fails — budget exhausted.
	_, err = pager.Page(PageRequest{ArtifactID: "art-1"})
	assert.ErrorIs(t, err, ErrBudgetExhausted)
}

// mockStore is a simple in-memory ContentStore for testing.
type mockStore struct {
	data map[string]string
}

func (m *mockStore) GetContent(artifactID string) (string, error) {
	content, ok := m.data[artifactID]
	if !ok {
		return "", fmt.Errorf("artifact %q not found", artifactID)
	}
	return content, nil
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/paging/ -v -count=1`
Expected: FAIL — functions not defined

**Step 3: Write minimal implementation**

```go
// Package paging provides on-demand artifact content retrieval with
// per-task token budget enforcement.
package paging

import (
	"errors"
	"fmt"
	"strings"
)

// ErrBudgetExhausted is returned when a page request exceeds the budget.
var ErrBudgetExhausted = errors.New("paging: budget exhausted")

// ContentStore provides access to artifact content.
type ContentStore interface {
	GetContent(artifactID string) (string, error)
}

// PageRequest describes a request to fetch a segment of an artifact.
type PageRequest struct {
	ArtifactID string `json:"artifact_id"`
	StartLine  int    `json:"start_line"` // 1-based inclusive, 0 = from start
	EndLine    int    `json:"end_line"`   // 1-based inclusive, 0 = to end
}

// PageResult holds the fetched content segment.
type PageResult struct {
	ArtifactID string `json:"artifact_id"`
	Content    string `json:"content"`
	Lines      int    `json:"lines"`
	Tokens     int    `json:"tokens"`
}

// Budget tracks paging quota per task.
type Budget struct {
	MaxPages   int `json:"max_pages"`
	MaxTokens  int `json:"max_tokens"`
	PagesUsed  int `json:"pages_used"`
	TokensUsed int `json:"tokens_used"`
}

// DefaultBudget returns a budget with MaxPages=10, MaxTokens=8000.
func DefaultBudget() *Budget {
	return &Budget{MaxPages: 10, MaxTokens: 8000}
}

// NewBudget creates a budget with custom limits.
func NewBudget(maxPages, maxTokens int) *Budget {
	return &Budget{MaxPages: maxPages, MaxTokens: maxTokens}
}

// CanPage returns true if there is remaining page and token budget.
func (b *Budget) CanPage() bool {
	return b.PagesUsed < b.MaxPages && b.TokensUsed < b.MaxTokens
}

// Record records a page fetch consuming the given number of tokens.
func (b *Budget) Record(tokens int) {
	b.PagesUsed++
	b.TokensUsed += tokens
}

// Remaining returns the remaining page and token budget.
func (b *Budget) Remaining() (pages, tokens int) {
	return b.MaxPages - b.PagesUsed, b.MaxTokens - b.TokensUsed
}

// EstimateTokens estimates the token count for a string (len / 4).
func EstimateTokens(text string) int {
	return len(text) / 4
}

// Pager executes page requests against a content store with budget enforcement.
type Pager struct {
	store  ContentStore
	budget *Budget
}

// NewPager creates a Pager with the given content store and budget.
func NewPager(store ContentStore, budget *Budget) *Pager {
	return &Pager{store: store, budget: budget}
}

// Page fetches a content segment from the store, extracts the requested
// line range, estimates tokens, and records against the budget.
func (p *Pager) Page(req PageRequest) (PageResult, error) {
	if !p.budget.CanPage() {
		return PageResult{}, ErrBudgetExhausted
	}

	content, err := p.store.GetContent(req.ArtifactID)
	if err != nil {
		return PageResult{}, fmt.Errorf("paging: %w", err)
	}

	lines := strings.Split(content, "\n")

	// Apply line range (1-based, clamped).
	start := 0
	end := len(lines)

	if req.StartLine > 0 {
		start = req.StartLine - 1
		if start > len(lines) {
			start = len(lines)
		}
	}
	if req.EndLine > 0 {
		end = req.EndLine
		if end > len(lines) {
			end = len(lines)
		}
	}
	if start > end {
		start = end
	}

	selected := lines[start:end]
	result := PageResult{
		ArtifactID: req.ArtifactID,
		Content:    strings.Join(selected, "\n"),
		Lines:      len(selected),
		Tokens:     EstimateTokens(strings.Join(selected, "\n")),
	}

	p.budget.Record(result.Tokens)

	return result, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/paging/ -v -count=1 -race`
Expected: PASS (7 tests)

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/paging/paging.go internal/paging/paging_test.go
git commit -m "feat(paging): add context paging with budget enforcement, line extraction, and token estimation

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: Format Functions + CLI Commands

**Files:**
- Create: `internal/paging/format.go`
- Create: `cmd/apex/paging.go`
- Modify: `cmd/apex/main.go:53` (add `rootCmd.AddCommand(pagingCmd)`)

**Step 1: Write format.go**

```go
package paging

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatPageResult formats a page result as a human-readable summary.
func FormatPageResult(result PageResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Artifact: %s\n", result.ArtifactID)
	fmt.Fprintf(&b, "Lines:    %d\n", result.Lines)
	fmt.Fprintf(&b, "Tokens:   %d\n", result.Tokens)
	fmt.Fprintf(&b, "---\n%s\n", result.Content)
	return b.String()
}

// FormatBudget formats a budget as a human-readable table.
func FormatBudget(budget *Budget) string {
	if budget == nil {
		return "No budget configured."
	}
	pagesRem, tokensRem := budget.Remaining()
	var b strings.Builder
	fmt.Fprintf(&b, "PAGES    %d / %d  (%d remaining)\n", budget.PagesUsed, budget.MaxPages, pagesRem)
	fmt.Fprintf(&b, "TOKENS   %d / %d  (%d remaining)\n", budget.TokensUsed, budget.MaxTokens, tokensRem)
	return b.String()
}

// FormatBudgetJSON formats a budget as indented JSON.
func FormatBudgetJSON(budget *Budget) (string, error) {
	if budget == nil {
		return "{}", nil
	}
	data, err := json.MarshalIndent(budget, "", "  ")
	if err != nil {
		return "", fmt.Errorf("paging: json marshal: %w", err)
	}
	return string(data), nil
}
```

**Step 2: Write paging.go CLI**

```go
package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lyndonlyu/apex/internal/paging"
	"github.com/spf13/cobra"
)

var pagingLines string
var pagingFormat string

var pagingCmd = &cobra.Command{
	Use:     "paging",
	Aliases: []string{"page"},
	Short:   "Context paging for on-demand artifact retrieval",
}

var pagingPageCmd = &cobra.Command{
	Use:   "fetch <artifact-id>",
	Short: "Fetch content from an artifact",
	Args:  cobra.ExactArgs(1),
	RunE:  runPagingFetch,
}

var pagingBudgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Show default paging budget",
	RunE:  runPagingBudget,
}

func init() {
	pagingPageCmd.Flags().StringVar(&pagingLines, "lines", "", "Line range (e.g. 10-50)")
	pagingBudgetCmd.Flags().StringVar(&pagingFormat, "format", "", "Output format (json)")
	pagingCmd.AddCommand(pagingPageCmd, pagingBudgetCmd)
}

// parseLineRange parses "START-END" into two ints.
func parseLineRange(s string) (start, end int, err error) {
	if s == "" {
		return 0, 0, nil
	}
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid line range %q (expected START-END)", s)
	}
	start, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start line %q: %w", parts[0], err)
	}
	end, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end line %q: %w", parts[1], err)
	}
	return start, end, nil
}

func runPagingFetch(cmd *cobra.Command, args []string) error {
	artifactID := args[0]

	startLine, endLine, err := parseLineRange(pagingLines)
	if err != nil {
		return err
	}

	// Use an empty in-memory store — real integration would use artifact store.
	store := &memoryStore{data: map[string]string{}}
	budget := paging.DefaultBudget()
	pager := paging.NewPager(store, budget)

	result, err := pager.Page(paging.PageRequest{
		ArtifactID: artifactID,
		StartLine:  startLine,
		EndLine:    endLine,
	})
	if err != nil {
		return err
	}

	fmt.Print(paging.FormatPageResult(result))
	return nil
}

func runPagingBudget(cmd *cobra.Command, args []string) error {
	budget := paging.DefaultBudget()

	if pagingFormat == "json" {
		out, err := paging.FormatBudgetJSON(budget)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(paging.FormatBudget(budget))
	}
	return nil
}

// memoryStore is a simple in-memory ContentStore for CLI usage.
type memoryStore struct {
	data map[string]string
}

func (m *memoryStore) GetContent(artifactID string) (string, error) {
	content, ok := m.data[artifactID]
	if !ok {
		return "", fmt.Errorf("artifact %q not found", artifactID)
	}
	return content, nil
}
```

**Step 3: Add command to main.go**

Add `rootCmd.AddCommand(pagingCmd)` after the `credentialCmd` line in `cmd/apex/main.go`.

**Step 4: Run build + tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build ./cmd/apex/ && go test ./internal/paging/ -v -count=1`
Expected: BUILD OK, PASS

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/paging/format.go cmd/apex/paging.go cmd/apex/main.go
git commit -m "feat(paging): add format functions and CLI for paging fetch/budget

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: E2E Tests (3 tests)

**Files:**
- Create: `e2e/paging_test.go`

**Step 1: Write E2E tests**

```go
package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextBudget(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("paging", "budget")

	assert.Equal(t, 0, exitCode,
		"apex paging budget should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "PAGES"),
		"stdout should contain PAGES, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "10"),
		"stdout should contain default max pages 10, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "8000"),
		"stdout should contain default max tokens 8000, got: %s", stdout)
}

func TestContextBudgetJSON(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("paging", "budget", "--format", "json")

	assert.Equal(t, 0, exitCode,
		"apex paging budget --format json should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "max_pages"),
		"stdout should contain max_pages JSON key, got: %s", stdout)
}

func TestContextPageNotFound(t *testing.T) {
	env := newTestEnv(t)

	_, _, exitCode := env.runApex("paging", "fetch", "nonexistent-artifact")

	assert.NotEqual(t, 0, exitCode,
		"apex paging fetch with nonexistent artifact should exit non-zero")
}
```

**Step 2: Build and run E2E tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./e2e/ -run TestContext -v -count=1`
Expected: PASS (3 tests)

**Step 3: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add e2e/paging_test.go
git commit -m "test(e2e): add E2E tests for paging budget and fetch

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Update PROGRESS.md

**Files:**
- Modify: `PROGRESS.md`

**Step 1: Update completed phases table**

Add row: `| 36 | Context Paging Tool | \`2026-02-20-phase36-context-paging-design.md\` | Done |`

**Step 2: Update Current section**

Change "Phase 36" → "Phase 37 — TBD"

**Step 3: Update test counts**

- Unit tests: 41 → 42 packages
- E2E tests: 105 → 108 tests

**Step 4: Add Key Package**

Add: `| \`internal/paging\` | On-demand artifact content paging with line extraction, token estimation, and per-task budget enforcement |`

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add PROGRESS.md
git commit -m "docs: mark Phase 36 Context Paging Tool as complete

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

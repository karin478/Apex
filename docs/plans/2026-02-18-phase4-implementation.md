# Phase 4 Context Builder Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a Context Builder that assembles optimal prompts for DAG nodes by retrieving memory, reading files, and compressing content within a token budget.

**Architecture:** New `internal/context` package with Builder (orchestration), Compress (four-tier compression), and Token (estimation). Integrates with existing `search.Engine` for memory retrieval. Wired into `cmd/apex/run.go` before executor calls.

**Tech Stack:** Go, Testify, existing search.Engine/memory.Store

---

### Task 1: Config Update

**Files:**
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/config/config.go`
- Test: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/config/config_test.go`

**Step 1: Write the failing test**

Add to `config_test.go`:

```go
func TestDefaultConfigPhase4(t *testing.T) {
	cfg := Default()
	assert.Equal(t, 60000, cfg.Context.TokenBudget)
}

func TestLoadConfigPhase4Override(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`context:
  token_budget: 30000
`)
	require.NoError(t, os.WriteFile(configPath, content, 0644))
	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, 30000, cfg.Context.TokenBudget)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestDefaultConfigPhase4 -v`
Expected: FAIL — `cfg.Context` undefined

**Step 3: Write minimal implementation**

Add to `config.go`:

```go
type ContextConfig struct {
	TokenBudget int `yaml:"token_budget"`
}
```

Add `Context ContextConfig` field to `Config` struct:

```go
type Config struct {
	Claude     ClaudeConfig     `yaml:"claude"`
	Governance GovernanceConfig `yaml:"governance"`
	Planner    PlannerConfig    `yaml:"planner"`
	Pool       PoolConfig       `yaml:"pool"`
	Embedding  EmbeddingConfig  `yaml:"embedding"`
	Context    ContextConfig    `yaml:"context"`
	BaseDir    string           `yaml:"-"`
}
```

In `Default()` add:

```go
Context: ContextConfig{
	TokenBudget: 60000,
},
```

In `Load()` add zero-value guard:

```go
if cfg.Context.TokenBudget == 0 {
	cfg.Context.TokenBudget = 60000
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add context builder config for Phase 4"
```

---

### Task 2: Token Estimation Package

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/context/token.go`
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/context/token_test.go`

**Step 1: Write the failing tests**

```go
package context

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimateTokensEnglish(t *testing.T) {
	text := "Hello world, this is a test."
	tokens := EstimateTokens(text)
	assert.Greater(t, tokens, 0)
	// ~9 runes / 3 = 3, roughly
	assert.InDelta(t, len([]rune(text))/3, tokens, 5)
}

func TestEstimateTokensChinese(t *testing.T) {
	text := "你好世界这是一个测试"
	tokens := EstimateTokens(text)
	assert.Greater(t, tokens, 0)
}

func TestEstimateTokensEmpty(t *testing.T) {
	assert.Equal(t, 0, EstimateTokens(""))
}

func TestEstimateTokensLong(t *testing.T) {
	text := strings.Repeat("word ", 3000)
	tokens := EstimateTokens(text)
	// 15000 chars / 3 = 5000 roughly
	assert.Greater(t, tokens, 3000)
	assert.Less(t, tokens, 8000)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/context/ -run TestEstimateTokens -v`
Expected: FAIL — package does not exist

**Step 3: Write minimal implementation**

```go
package context

// EstimateTokens returns an approximate token count for the given text.
// Uses rune count / 3 as a rough approximation suitable for mixed CJK/Latin text.
func EstimateTokens(text string) int {
	runes := []rune(text)
	if len(runes) == 0 {
		return 0
	}
	est := len(runes) / 3
	if est == 0 {
		est = 1
	}
	return est
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/context/ -run TestEstimateTokens -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/context/token.go internal/context/token_test.go
git commit -m "feat(context): add token estimation utility"
```

---

### Task 3: Compression Functions

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/context/compress.go`
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/context/compress_test.go`

**Step 1: Write the failing tests**

```go
package context

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompressExact(t *testing.T) {
	text := "keep this exactly as is"
	result := CompressExact(text)
	assert.Equal(t, text, result)
}

func TestCompressStructuralGo(t *testing.T) {
	code := `package main

import "fmt"

// Greet says hello.
func Greet(name string) string {
	msg := fmt.Sprintf("Hello, %s!", name)
	return msg
}

func helper() {
	// long implementation
	x := 1
	y := 2
	z := x + y
	_ = z
}
`
	result := CompressStructural(code)
	// Should keep function signatures
	assert.Contains(t, result, "func Greet(name string) string")
	assert.Contains(t, result, "func helper()")
	// Should keep package and imports
	assert.Contains(t, result, "package main")
	// Should truncate bodies (shorter than original)
	assert.Less(t, len(result), len(code))
}

func TestCompressSummarizable(t *testing.T) {
	doc := `# Redis Migration Guide

This document describes the migration from Redis 6 to Redis 7.

## Prerequisites

You need to have Redis 7.2 installed on all servers.
Make sure to backup your data before proceeding.

## Step 1: Update Configuration

Edit redis.conf and change the ACL settings.
There are many detailed sub-steps here.
Line after line of detailed instructions.
More details that can be summarized.

## Step 2: Run Migration

Execute the migration script provided in the tools directory.
Additional details about running the migration.
`
	result := CompressSummarizable(doc)
	// Should keep headings
	assert.Contains(t, result, "# Redis Migration Guide")
	assert.Contains(t, result, "## Prerequisites")
	// Should keep first paragraph
	assert.Contains(t, result, "migration from Redis 6 to Redis 7")
	// Should be shorter than original
	assert.Less(t, len(result), len(doc))
}

func TestCompressReference(t *testing.T) {
	text := "first line of the file\nsecond line\nthird line\nmany more lines"
	result := CompressReference("path/to/file.go", text)
	assert.Contains(t, result, "path/to/file.go")
	assert.Contains(t, result, "first line of the file")
	// Should be much shorter
	assert.Less(t, len(result), len(text))
}

func TestCompressStructuralNonCode(t *testing.T) {
	// Non-code content falls back to summarizable behavior
	text := "Some plain text content\nwith multiple lines\nand more content here"
	result := CompressStructural(text)
	assert.NotEmpty(t, result)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/context/ -run TestCompress -v`
Expected: FAIL — functions undefined

**Step 3: Write minimal implementation**

```go
package context

import (
	"fmt"
	"strings"
)

// CompressionPolicy represents how content should be compressed.
type CompressionPolicy int

const (
	PolicyExact CompressionPolicy = iota
	PolicyStructural
	PolicySummarizable
	PolicyReference
)

func (p CompressionPolicy) String() string {
	switch p {
	case PolicyExact:
		return "exact"
	case PolicyStructural:
		return "structural"
	case PolicySummarizable:
		return "summarizable"
	case PolicyReference:
		return "reference"
	default:
		return "unknown"
	}
}

// CompressExact returns text unchanged.
func CompressExact(text string) string {
	return text
}

// CompressStructural extracts function/type signatures and structure skeleton from code.
// For non-code content, falls back to summarizable behavior.
func CompressStructural(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return text
	}

	// Detect if it looks like Go code
	isCode := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") ||
			strings.HasPrefix(trimmed, "func ") ||
			strings.HasPrefix(trimmed, "type ") ||
			strings.HasPrefix(trimmed, "import ") ||
			strings.HasPrefix(trimmed, "def ") ||
			strings.HasPrefix(trimmed, "class ") {
			isCode = true
			break
		}
	}

	if !isCode {
		return CompressSummarizable(text)
	}

	var result []string
	inBody := false
	braceDepth := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Always keep package, import, type, func signature, comments
		if strings.HasPrefix(trimmed, "package ") ||
			strings.HasPrefix(trimmed, "import ") ||
			strings.HasPrefix(trimmed, "//") ||
			strings.HasPrefix(trimmed, "type ") ||
			strings.HasPrefix(trimmed, "func ") ||
			strings.HasPrefix(trimmed, "def ") ||
			strings.HasPrefix(trimmed, "class ") ||
			trimmed == ")" || trimmed == "}" ||
			(strings.HasPrefix(trimmed, "\"") && braceDepth == 0) {
			result = append(result, line)
		}

		// Track brace depth for body skipping
		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
		if braceDepth > 1 {
			inBody = true
		}
		if braceDepth <= 1 {
			inBody = false
		}
		_ = inBody
	}

	return strings.Join(result, "\n")
}

// CompressSummarizable extracts headings, first paragraph, and key lines.
func CompressSummarizable(text string) string {
	lines := strings.Split(text, "\n")
	var result []string
	firstParaDone := false
	inFirstPara := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Keep headings (markdown)
		if strings.HasPrefix(trimmed, "#") {
			result = append(result, line)
			continue
		}

		// Keep first non-empty paragraph after title
		if !firstParaDone {
			if trimmed == "" {
				if inFirstPara {
					firstParaDone = true
					result = append(result, "")
				}
				continue
			}
			inFirstPara = true
			result = append(result, line)
			continue
		}
	}

	return strings.Join(result, "\n")
}

// CompressReference returns only path and first line summary.
func CompressReference(path string, text string) string {
	firstLine := text
	if idx := strings.Index(text, "\n"); idx >= 0 {
		firstLine = text[:idx]
	}
	if len(firstLine) > 120 {
		firstLine = firstLine[:120] + "..."
	}
	size := len(text)
	return fmt.Sprintf("[ref: %s (%d bytes)] %s", path, size, firstLine)
}

// Compress applies the given policy to text.
func Compress(policy CompressionPolicy, path string, text string) string {
	switch policy {
	case PolicyExact:
		return CompressExact(text)
	case PolicyStructural:
		return CompressStructural(text)
	case PolicySummarizable:
		return CompressSummarizable(text)
	case PolicyReference:
		return CompressReference(path, text)
	default:
		return text
	}
}

// Degrade returns the next lower compression policy.
// exact does not degrade (returns exact).
func Degrade(policy CompressionPolicy) CompressionPolicy {
	switch policy {
	case PolicySummarizable:
		return PolicyReference
	case PolicyStructural:
		return PolicySummarizable
	default:
		return policy
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/context/ -run TestCompress -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/context/compress.go internal/context/compress_test.go
git commit -m "feat(context): add four-tier compression functions"
```

---

### Task 4: Context Builder Core

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/context/builder.go`
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/context/builder_test.go`

**Step 1: Write the failing tests**

```go
package context

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSearchResult mimics search.Result for testing without importing search package
type mockSearchEngine struct {
	results []SearchResult
	err     error
}

func (m *mockSearchEngine) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	return m.results, m.err
}

func TestBuildBasicPrompt(t *testing.T) {
	b := NewBuilder(Options{TokenBudget: 60000})
	result, err := b.Build(context.Background(), "Write a hello world program")
	require.NoError(t, err)
	assert.Contains(t, result, "Write a hello world program")
}

func TestBuildWithMemory(t *testing.T) {
	engine := &mockSearchEngine{
		results: []SearchResult{
			{ID: "facts/go.md", Text: "We use Go 1.25", Score: 0.9, Type: "fact"},
		},
	}
	b := NewBuilder(Options{TokenBudget: 60000, Searcher: engine})
	result, err := b.Build(context.Background(), "Write a Go function")
	require.NoError(t, err)
	assert.Contains(t, result, "Write a Go function")
	assert.Contains(t, result, "We use Go 1.25")
}

func TestBuildWithFiles(t *testing.T) {
	dir := t.TempDir()
	// Create a test file
	writeTempFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")

	b := NewBuilder(Options{
		TokenBudget: 60000,
		Files:       []string{dir + "/main.go"},
	})
	result, err := b.Build(context.Background(), "Modify the main function")
	require.NoError(t, err)
	assert.Contains(t, result, "package main")
}

func TestBuildBudgetOverflow(t *testing.T) {
	// Very small budget — task alone fits, but extras get degraded
	b := NewBuilder(Options{TokenBudget: 50})
	result, err := b.Build(context.Background(), "short task")
	require.NoError(t, err)
	assert.Contains(t, result, "short task")
}

func TestBuildExactExceedsBudget(t *testing.T) {
	// Budget so small even the task doesn't fit — should still work (task is always included)
	b := NewBuilder(Options{TokenBudget: 1})
	result, err := b.Build(context.Background(), "task")
	require.NoError(t, err)
	assert.Contains(t, result, "task")
}

func TestBuildFileReadError(t *testing.T) {
	b := NewBuilder(Options{
		TokenBudget: 60000,
		Files:       []string{"/nonexistent/file.go"},
	})
	// Should not error, just skip the file
	result, err := b.Build(context.Background(), "do something")
	require.NoError(t, err)
	assert.Contains(t, result, "do something")
}

func TestBuildSearchError(t *testing.T) {
	engine := &mockSearchEngine{
		err: assert.AnError,
	}
	b := NewBuilder(Options{TokenBudget: 60000, Searcher: engine})
	// Should not error, just skip memory
	result, err := b.Build(context.Background(), "do something")
	require.NoError(t, err)
	assert.Contains(t, result, "do something")
}

func TestBuildDegradation(t *testing.T) {
	// Create a builder with tight budget and multiple content blocks
	engine := &mockSearchEngine{
		results: []SearchResult{
			{ID: "facts/a.md", Text: "fact A content that is reasonably long to consume tokens", Score: 0.9, Type: "fact"},
			{ID: "facts/b.md", Text: "fact B content that is also reasonably long to consume tokens", Score: 0.5, Type: "fact"},
		},
	}
	// Budget just enough for task + one memory result
	b := NewBuilder(Options{TokenBudget: 80, Searcher: engine})
	result, err := b.Build(context.Background(), "task")
	require.NoError(t, err)
	assert.Contains(t, result, "task")
	// Lower priority content should be degraded or dropped
}
```

Helper function (add at bottom of test file):

```go
func writeTempFile(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(dir+"/"+name, []byte(content), 0644)
	require.NoError(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/context/ -run TestBuild -v`
Expected: FAIL — NewBuilder undefined

**Step 3: Write minimal implementation**

```go
package context

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SearchResult is the interface-compatible result from hybrid search.
type SearchResult struct {
	ID    string
	Text  string
	Score float32
	Type  string
}

// Searcher retrieves relevant memory for a query.
type Searcher interface {
	Search(ctx context.Context, query string, topK int) ([]SearchResult, error)
}

// Options configures the Context Builder.
type Options struct {
	TokenBudget int
	Searcher    Searcher
	Files       []string
}

// Builder assembles optimized prompts within a token budget.
type Builder struct {
	opts Options
}

// ContentBlock represents a single piece of context.
type ContentBlock struct {
	ID       string
	Source   string // "task" | "memory" | "file"
	Path     string
	Text     string
	Policy   CompressionPolicy
	Priority int
}

// NewBuilder creates a new Context Builder.
func NewBuilder(opts Options) *Builder {
	if opts.TokenBudget <= 0 {
		opts.TokenBudget = 60000
	}
	return &Builder{opts: opts}
}

// Build assembles an optimized prompt for the given task.
func (b *Builder) Build(ctx context.Context, task string) (string, error) {
	var blocks []ContentBlock

	// Task description is always highest priority, exact policy
	blocks = append(blocks, ContentBlock{
		ID:       "task",
		Source:   "task",
		Text:     task,
		Policy:   PolicyExact,
		Priority: 100,
	})

	// Retrieve memory context
	if b.opts.Searcher != nil {
		results, err := b.opts.Searcher.Search(ctx, task, 10)
		if err == nil {
			for _, r := range results {
				blocks = append(blocks, ContentBlock{
					ID:       fmt.Sprintf("memory:%s", r.ID),
					Source:   "memory",
					Path:     r.ID,
					Text:     r.Text,
					Policy:   PolicySummarizable,
					Priority: 80 - int(10*(1.0-r.Score)),
				})
			}
		}
	}

	// Read file context
	for _, f := range b.opts.Files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue // graceful degradation
		}
		policy := classifyFile(f)
		blocks = append(blocks, ContentBlock{
			ID:       fmt.Sprintf("file:%s", f),
			Source:   "file",
			Path:     f,
			Text:     string(data),
			Policy:   policy,
			Priority: 60,
		})
	}

	// Sort by priority descending (highest priority = compressed last during degradation)
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Priority > blocks[j].Priority
	})

	// Apply compression and fit within budget
	compressed := b.fitBudget(blocks)

	// Assemble prompt
	return b.assemble(compressed), nil
}

// fitBudget compresses blocks to fit within token budget.
// Degrades lowest-priority blocks first when over budget.
func (b *Builder) fitBudget(blocks []ContentBlock) []ContentBlock {
	// Apply initial compression
	for i := range blocks {
		blocks[i].Text = Compress(blocks[i].Policy, blocks[i].Path, blocks[i].Text)
	}

	// Check total tokens
	total := 0
	for _, bl := range blocks {
		total += EstimateTokens(bl.Text)
	}

	// Degrade from lowest priority until under budget
	for total > b.opts.TokenBudget {
		degraded := false
		// Iterate from lowest priority (end of sorted list) to highest
		for i := len(blocks) - 1; i >= 0; i-- {
			if blocks[i].Policy == PolicyExact || blocks[i].Policy == PolicyReference {
				continue // can't degrade further
			}
			oldTokens := EstimateTokens(blocks[i].Text)
			blocks[i].Policy = Degrade(blocks[i].Policy)
			blocks[i].Text = Compress(blocks[i].Policy, blocks[i].Path, blocks[i].Text)
			newTokens := EstimateTokens(blocks[i].Text)
			total -= (oldTokens - newTokens)
			degraded = true
			break
		}
		if !degraded {
			// Remove lowest priority non-task block if nothing can be degraded
			for i := len(blocks) - 1; i >= 0; i-- {
				if blocks[i].Source != "task" {
					total -= EstimateTokens(blocks[i].Text)
					blocks = append(blocks[:i], blocks[i+1:]...)
					break
				}
			}
			break
		}
	}

	return blocks
}

// classifyFile assigns a default compression policy based on file extension.
func classifyFile(path string) CompressionPolicy {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".py", ".js", ".ts", ".java", ".rs", ".c", ".cpp":
		return PolicyStructural
	case ".md", ".txt", ".rst":
		return PolicySummarizable
	case ".json", ".yaml", ".yml", ".toml":
		return PolicyExact
	default:
		return PolicySummarizable
	}
}

// assemble joins compressed blocks into a final prompt string.
func (b *Builder) assemble(blocks []ContentBlock) string {
	var sections []string

	// Task always first
	for _, bl := range blocks {
		if bl.Source == "task" {
			sections = append(sections, fmt.Sprintf("## Task\n\n%s", bl.Text))
			break
		}
	}

	// Memory context
	var memParts []string
	for _, bl := range blocks {
		if bl.Source == "memory" {
			memParts = append(memParts, fmt.Sprintf("- [%s] %s", bl.Path, bl.Text))
		}
	}
	if len(memParts) > 0 {
		sections = append(sections, fmt.Sprintf("## Relevant Memory\n\n%s", strings.Join(memParts, "\n")))
	}

	// File context
	for _, bl := range blocks {
		if bl.Source == "file" {
			sections = append(sections, fmt.Sprintf("## File: %s\n\n%s", bl.Path, bl.Text))
		}
	}

	return strings.Join(sections, "\n\n")
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/context/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/context/builder.go internal/context/builder_test.go
git commit -m "feat(context): add Context Builder with budget management and degradation"
```

---

### Task 5: Wire into CLI

**Files:**
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/run.go`

**Step 1: Update run.go to use Context Builder**

In `run.go`, the current code sends raw task strings to the executor. We need to:
1. Import the context package
2. Before DAG execution, create a Context Builder
3. In the pool runner, wrap tasks with context-built prompts

Since the pool uses `ClaudeRunner.RunTask(ctx, task)` which takes a plain string, the simplest integration point is to build context for each DAG node's task before the pool executes it.

Update `run.go` to build context for each node before execution:

```go
// Add import
"github.com/lyndonlyu/apex/internal/context"

// After DAG creation (line ~83), before pool execution:
// Build context for each DAG node
ctxBuilder := apexctx.NewBuilder(apexctx.Options{
	TokenBudget: cfg.Context.TokenBudget,
})

for _, node := range d.Nodes {
	enriched, buildErr := ctxBuilder.Build(context.Background(), node.Task)
	if buildErr == nil {
		node.Task = enriched
	}
}
```

Note: Use import alias `apexctx "github.com/lyndonlyu/apex/internal/context"` to avoid collision with standard `context` package.

**Step 2: Run all tests**

Run: `go test ./... 2>&1`
Expected: ALL 13 packages PASS

**Step 3: Build and verify**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build -o bin/apex ./cmd/apex/ && ./bin/apex run --help`
Expected: Build succeeds, help text displayed

**Step 4: Commit**

```bash
git add cmd/apex/run.go
git commit -m "feat: integrate Context Builder into DAG execution pipeline"
```

---

### Task 6: E2E Verification

**Step 1: Run all tests**

```bash
go test ./... -v 2>&1
```

Expected: All 13 packages pass (12 existing + 1 new `internal/context`)

**Step 2: Build binary**

```bash
go build -o bin/apex ./cmd/apex/
```

Expected: Clean build (only macOS sqlite-vec deprecation warnings)

**Step 3: Verify CLI**

```bash
./bin/apex --help
./bin/apex run --help
```

Expected: All commands work

**Step 4: Final commit (if any remaining changes)**

```bash
git status
# If clean, no commit needed
```

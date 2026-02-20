# Phase 36: Context Paging Tool

> Design doc for Apex Agent CLI — on-demand artifact content retrieval with token budget enforcement.

## Problem

The Context Builder (Phase 4) compresses content to fit token limits, but compression permanently discards detail. When an agent needs specific lines from a compressed artifact, it has no mechanism to recover the original content. This forces a choice between over-compression (lose details) and under-compression (blow token budget).

## Solution

A `paging` package that provides on-demand retrieval of artifact content segments with per-task token budget enforcement. Agents can call `Page()` to fetch specific line ranges from artifacts, with quota limits preventing unbounded retrieval.

## Architecture

```
internal/paging/
├── paging.go       # PageRequest, PageResult, Budget, Pager, ContentStore
└── paging_test.go  # 7 unit tests
```

## Key Types

### PageRequest

```go
type PageRequest struct {
    ArtifactID string `json:"artifact_id"`
    StartLine  int    `json:"start_line"` // 1-based inclusive
    EndLine    int    `json:"end_line"`   // 1-based inclusive, 0 = to end
}
```

### PageResult

```go
type PageResult struct {
    ArtifactID string `json:"artifact_id"`
    Content    string `json:"content"`
    Lines      int    `json:"lines"`
    Tokens     int    `json:"tokens"`
}
```

### Budget

```go
type Budget struct {
    MaxPages   int `json:"max_pages"`
    MaxTokens  int `json:"max_tokens"`
    PagesUsed  int `json:"pages_used"`
    TokensUsed int `json:"tokens_used"`
}
```

### ContentStore Interface

```go
type ContentStore interface {
    GetContent(artifactID string) (string, error)
}
```

Implemented by callers (e.g., `internal/artifact` adapter or test mock).

### Pager

```go
type Pager struct {
    store  ContentStore
    budget *Budget
}
```

## Core Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `DefaultBudget` | `() *Budget` | Returns budget with MaxPages=10, MaxTokens=8000 |
| `NewBudget` | `(maxPages, maxTokens int) *Budget` | Creates custom budget |
| `(*Budget) CanPage` | `() bool` | Returns true if pages remaining > 0 |
| `(*Budget) Record` | `(tokens int)` | Increments PagesUsed and TokensUsed |
| `(*Budget) Remaining` | `() (pages, tokens int)` | Returns remaining quota |
| `NewPager` | `(store ContentStore, budget *Budget) *Pager` | Creates Pager with injected store |
| `(*Pager) Page` | `(req PageRequest) (PageResult, error)` | Execute: check budget → fetch content → extract lines → estimate tokens → record → return |
| `EstimateTokens` | `(text string) int` | Simple estimation: `len(text) / 4` |

## Design Decisions

### Token Estimation

`EstimateTokens` uses `len(text) / 4` — a simple, fast heuristic that's accurate enough for budget enforcement. No external tokenizer dependency.

### Budget Exhaustion

`Page()` returns `ErrBudgetExhausted` when:
- `PagesUsed >= MaxPages`, OR
- `TokensUsed >= MaxTokens`

The caller (CLI or agent framework) maps this to ESCALATED state. The paging package itself has no knowledge of execution states.

### Line Range Extraction

- `StartLine` is 1-based inclusive
- `EndLine` is 1-based inclusive; 0 means "to end of content"
- Out-of-range lines are clamped (not errored)
- Content is split by `\n`, joined back after extraction

### ContentStore Decoupling

The `ContentStore` interface decouples paging from artifact storage. In production, an adapter wraps `internal/artifact`; in tests, a simple map-based mock suffices.

## CLI Commands

### `apex context page <artifact-id> [--lines 10-50]`
Fetches content from the specified artifact, optionally limited to a line range. Uses an in-memory content store for CLI demonstration.

### `apex context budget`
Displays the current default paging budget (max pages, max tokens).

## Unit Tests (7)

| Test | Description |
|------|-------------|
| `TestDefaultBudget` | Default values: MaxPages=10, MaxTokens=8000 |
| `TestNewBudget` | Custom values set correctly |
| `TestBudgetCanPage` | Fresh budget → true; exhausted pages → false; exhausted tokens → false |
| `TestBudgetRecord` | Record increments PagesUsed and adds TokensUsed |
| `TestEstimateTokens` | Various strings → correct `len/4` estimation |
| `TestPagerPage` | Mock store → correct content extraction, line range, token count |
| `TestPagerPageBudgetExhausted` | Budget at limit → ErrBudgetExhausted |

## E2E Tests (3)

| Test | Description |
|------|-------------|
| `TestContextPage` | CLI invocation → content output |
| `TestContextBudget` | CLI invocation → budget display |
| `TestContextPageNotFound` | Missing artifact → appropriate error |

## Format Functions

| Function | Description |
|----------|-------------|
| `FormatPageResult(result PageResult) string` | Human-readable: ArtifactID, Lines, Tokens, Content |
| `FormatBudget(budget *Budget) string` | Table: MaxPages, MaxTokens, Used, Remaining |
| `FormatBudgetJSON(budget *Budget) (string, error)` | JSON output |

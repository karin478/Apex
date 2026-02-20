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
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
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

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

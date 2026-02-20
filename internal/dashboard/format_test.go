package dashboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatTerminal(t *testing.T) {
	sections := []Section{
		{Title: "System Health", Content: "Level: GREEN\nChecks: config OK | audit OK\n"},
		{Title: "Recent Runs (5)", Content: "run-001  deploy-api  success  1500ms\n"},
	}

	out := FormatTerminal(sections)

	// Header box must be present.
	assert.Contains(t, out, "APEX SYSTEM DASHBOARD", "should contain dashboard title")

	// Box-drawing characters.
	assert.Contains(t, out, "══", "should contain box-drawing double lines")
	assert.Contains(t, out, "──", "should contain section separator lines")

	// Section titles.
	assert.Contains(t, out, "System Health", "should contain first section title")
	assert.Contains(t, out, "Recent Runs (5)", "should contain second section title")

	// Section content.
	assert.Contains(t, out, "Level: GREEN", "should contain health level")
	assert.Contains(t, out, "run-001", "should contain run ID")
}

func TestFormatMarkdown(t *testing.T) {
	sections := []Section{
		{Title: "Health", Content: "Level: GREEN\n"},
		{Title: "Metrics", Content: "Total runs: 42\n"},
	}

	out := FormatMarkdown(sections)

	// Top-level heading.
	assert.Contains(t, out, "# Apex System Dashboard", "should contain main heading")

	// Section headings.
	assert.Contains(t, out, "## Health", "should contain Health section heading")
	assert.Contains(t, out, "## Metrics", "should contain Metrics section heading")

	// Content.
	assert.Contains(t, out, "Level: GREEN", "should contain health content")
	assert.Contains(t, out, "Total runs: 42", "should contain metrics content")
}

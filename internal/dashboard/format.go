package dashboard

import (
	"fmt"
	"strings"
)

const (
	// boxWidth is the inner width of the terminal header box (excluding border chars).
	boxWidth = 38
	// lineWidth is the total width of section separator lines.
	lineWidth = 40
)

// FormatTerminal renders sections as a box-drawing terminal dashboard.
//
// Output structure:
//
//	╔══════════════════════════════════════╗
//	║       APEX SYSTEM DASHBOARD         ║
//	╚══════════════════════════════════════╝
//
//	── Section Title ──────────────────────
//	content lines...
func FormatTerminal(sections []Section) string {
	var b strings.Builder

	// ── Header box ──
	title := "APEX SYSTEM DASHBOARD"
	topBorder := "╔" + strings.Repeat("═", boxWidth) + "╗"
	botBorder := "╚" + strings.Repeat("═", boxWidth) + "╝"

	// Center the title inside the box.
	pad := boxWidth - len(title)
	leftPad := pad / 2
	rightPad := pad - leftPad
	midLine := "║" + strings.Repeat(" ", leftPad) + title + strings.Repeat(" ", rightPad) + "║"

	fmt.Fprintln(&b, topBorder)
	fmt.Fprintln(&b, midLine)
	fmt.Fprintln(&b, botBorder)

	// ── Sections ──
	for _, sec := range sections {
		b.WriteString("\n")
		// Build section header: ── Title ──────────...
		prefix := "── " + sec.Title + " "
		remaining := lineWidth - len(prefix)
		if remaining < 0 {
			remaining = 0
		}
		fmt.Fprintln(&b, prefix+strings.Repeat("─", remaining))

		content := strings.TrimRight(sec.Content, "\n")
		if content != "" {
			fmt.Fprintln(&b, content)
		}
	}

	return b.String()
}

// FormatMarkdown renders sections as standard Markdown.
//
// Output structure:
//
//	# Apex System Dashboard
//
//	## Section Title
//
//	content lines...
func FormatMarkdown(sections []Section) string {
	var b strings.Builder

	fmt.Fprintln(&b, "# Apex System Dashboard")

	for _, sec := range sections {
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "## %s\n", sec.Title)
		fmt.Fprintln(&b)

		content := strings.TrimRight(sec.Content, "\n")
		if content != "" {
			fmt.Fprintln(&b, content)
		}
	}

	return b.String()
}

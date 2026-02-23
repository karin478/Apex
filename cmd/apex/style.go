package main

import "github.com/charmbracelet/lipgloss"

// Theme defines the colour palette for all TUI elements.
type Theme struct {
	Name    string
	Accent  lipgloss.Color
	Text    lipgloss.Color
	Dim     lipgloss.Color
	Meta    lipgloss.Color
	Success lipgloss.Color
	Error   lipgloss.Color
	Warning lipgloss.Color
	Info    lipgloss.Color
	Border  lipgloss.Color
}

var darkTheme = Theme{
	Name:    "dark",
	Accent:  lipgloss.Color("99"),
	Text:    lipgloss.Color("252"),
	Dim:     lipgloss.Color("242"),
	Meta:    lipgloss.Color("242"),
	Success: lipgloss.Color("10"),
	Error:   lipgloss.Color("9"),
	Warning: lipgloss.Color("11"),
	Info:    lipgloss.Color("245"),
	Border:  lipgloss.Color("237"),
}

var lightTheme = Theme{
	Name:    "light",
	Accent:  lipgloss.Color("55"),
	Text:    lipgloss.Color("235"),
	Dim:     lipgloss.Color("245"),
	Meta:    lipgloss.Color("245"),
	Success: lipgloss.Color("28"),
	Error:   lipgloss.Color("160"),
	Warning: lipgloss.Color("172"),
	Info:    lipgloss.Color("240"),
	Border:  lipgloss.Color("250"),
}

var activeTheme Theme

// Style variables — refreshed when the theme changes.
var (
	styleBannerTitle lipgloss.Style
	styleBannerInfo  lipgloss.Style
	stylePrompt      lipgloss.Style
	styleSuccess     lipgloss.Style
	styleError       lipgloss.Style
	styleSpinner     lipgloss.Style
	styleInfo        lipgloss.Style
	styleDim         lipgloss.Style
	styleMeta        lipgloss.Style
	styleStepBorder  lipgloss.Style
	styleStepName    lipgloss.Style
	styleRespTitle   lipgloss.Style
	styleRespRule    lipgloss.Style
)

// styleRisk uses hardcoded colours independent of the theme.
var styleRisk = map[string]lipgloss.Style{
	"LOW":      lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
	"MEDIUM":   lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
	"HIGH":     lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	"CRITICAL": lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1")),
}

func init() {
	initTheme()
	refreshStyles()
}

// initTheme auto-detects the terminal background and selects a theme.
func initTheme() {
	if lipgloss.HasDarkBackground() {
		activeTheme = darkTheme
	} else {
		activeTheme = lightTheme
	}
}

// SetTheme switches to a named theme ("dark" or "light").
// Returns false if the name is not recognised.
func SetTheme(name string) bool {
	switch name {
	case "dark":
		activeTheme = darkTheme
	case "light":
		activeTheme = lightTheme
	default:
		return false
	}
	refreshStyles()
	return true
}

// refreshStyles rebuilds every style variable from the active theme.
func refreshStyles() {
	t := activeTheme

	styleBannerTitle = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	styleBannerInfo = lipgloss.NewStyle().Foreground(t.Text)
	stylePrompt = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	styleSuccess = lipgloss.NewStyle().Foreground(t.Success)
	styleError = lipgloss.NewStyle().Foreground(t.Error)
	styleSpinner = lipgloss.NewStyle().Foreground(t.Accent)
	styleInfo = lipgloss.NewStyle().Foreground(t.Info)
	styleDim = lipgloss.NewStyle().Foreground(t.Dim)
	styleMeta = lipgloss.NewStyle().Foreground(t.Meta)
	styleStepBorder = lipgloss.NewStyle().Foreground(t.Border)
	styleStepName = lipgloss.NewStyle().Foreground(t.Text)
	styleRespTitle = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	styleRespRule = lipgloss.NewStyle().Foreground(t.Border)
}

// separator returns a dim horizontal rule.
func separator() string {
	return styleDim.Render("  ─────")
}

func renderRisk(level string) string {
	if s, ok := styleRisk[level]; ok {
		return s.Render(level)
	}
	return level
}

// responseHeader returns a styled "Response" header with a rule.
func responseHeader() string {
	title := styleRespTitle.Render("◆ Response")
	rule := styleRespRule.Render(" ─────────────────────────────")
	return "  " + title + rule
}

// errorHeader returns a styled "Error" header with a rule.
func errorHeader() string {
	title := styleError.Render("✗ Error")
	rule := styleRespRule.Render(" ────────────────────────────────")
	return "  " + title + rule
}

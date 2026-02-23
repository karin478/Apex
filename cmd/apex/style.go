package main

import "github.com/charmbracelet/lipgloss"

var (
	styleBanner  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	stylePrompt  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	styleRisk    = map[string]lipgloss.Style{
		"LOW":      lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		"MEDIUM":   lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		"HIGH":     lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		"CRITICAL": lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1")),
	}
	styleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleInfo    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func renderRisk(level string) string {
	if s, ok := styleRisk[level]; ok {
		return s.Render("[" + level + "]")
	}
	return "[" + level + "]"
}

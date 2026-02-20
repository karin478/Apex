package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/dashboard"
	"github.com/spf13/cobra"
)

var dashboardFormat string

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Show system status overview",
	Long:  "Generate a dashboard with health, runs, metrics, policy changes, and audit integrity.",
	RunE:  showDashboard,
}

func init() {
	dashboardCmd.Flags().StringVar(&dashboardFormat, "format", "terminal", "Output format: terminal or md")
}

func showDashboard(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, ".apex")

	d := dashboard.New(baseDir)
	sections, err := d.Generate()
	if err != nil {
		return fmt.Errorf("dashboard generation failed: %w", err)
	}

	switch dashboardFormat {
	case "md":
		fmt.Print(dashboard.FormatMarkdown(sections))
	default:
		fmt.Print(dashboard.FormatTerminal(sections))
	}
	return nil
}

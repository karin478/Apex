package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/audit"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show recent task history",
	RunE:  showHistory,
}

var historyCount int

func init() {
	historyCmd.Flags().IntVarP(&historyCount, "count", "n", 10, "Number of entries to show")
}

func showHistory(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	auditDir := filepath.Join(home, ".apex", "audit")

	logger, err := audit.NewLogger(auditDir)
	if err != nil {
		return fmt.Errorf("audit error: %w", err)
	}

	entries, err := logger.Recent(historyCount)
	if err != nil {
		return fmt.Errorf("failed to read history: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No history yet.")
		return nil
	}

	for _, e := range entries {
		icon := "[OK]"
		if e.Outcome == "failure" {
			icon = "[FAIL]"
		} else if e.Outcome == "timeout" {
			icon = "[TIMEOUT]"
		} else if e.Outcome == "rejected" {
			icon = "[REJECTED]"
		}
		fmt.Printf("%s %s [%s] %s (%dms)\n", icon, e.Timestamp[:19], e.RiskLevel, e.Task, e.DurationMs)
	}
	return nil
}

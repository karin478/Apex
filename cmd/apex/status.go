package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/manifest"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show recent run history",
	RunE:  showStatus,
}

var statusLast int

func init() {
	statusCmd.Flags().IntVar(&statusLast, "last", 5, "Number of recent runs to show")
}

func showStatus(cmd *cobra.Command, args []string) error {
	if statusLast < 1 {
		return fmt.Errorf("--last must be at least 1, got %d", statusLast)
	}
	home, _ := os.UserHomeDir()
	runsDir := filepath.Join(home, ".apex", "runs")
	store := manifest.NewStore(runsDir)

	recent, err := store.Recent(statusLast)
	if err != nil {
		return fmt.Errorf("failed to read runs: %w", err)
	}

	if len(recent) == 0 {
		fmt.Println("No runs found.")
		return nil
	}

	fmt.Printf("%-10s %-40s %-10s %-10s %-6s %s\n",
		"RUN_ID", "TASK", "OUTCOME", "DURATION", "NODES", "TIMESTAMP")
	fmt.Println("---------- ---------------------------------------- ---------- ---------- ------ --------------------")

	for _, m := range recent {
		runID := m.RunID
		if len(runID) > 8 {
			runID = runID[:8]
		}
		task := m.Task
		if len(task) > 40 {
			task = task[:37] + "..."
		}
		duration := fmt.Sprintf("%.1fs", float64(m.DurationMs)/1000)
		fmt.Printf("%-10s %-40s %-10s %-10s %-6d %s\n",
			runID, task, m.Outcome, duration, m.NodeCount, m.Timestamp)
	}

	return nil
}

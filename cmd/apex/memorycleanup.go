package main

import (
	"fmt"
	"time"

	"github.com/lyndonlyu/apex/internal/memclean"
	"github.com/spf13/cobra"
)

var (
	cleanupDryRun     bool
	cleanupMaxEntries int
)

var memoryCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Auto-cleanup stale memory entries",
	RunE:  runMemoryCleanup,
}

func init() {
	memoryCleanupCmd.Flags().BoolVar(&cleanupDryRun, "dry-run", false, "Preview what would be removed without deleting")
	memoryCleanupCmd.Flags().IntVar(&cleanupMaxEntries, "max-entries", 0, "Override max entries threshold")
	memoryCmd.AddCommand(memoryCleanupCmd)
}

func runMemoryCleanup(cmd *cobra.Command, args []string) error {
	memDir, err := memoryDir()
	if err != nil {
		return err
	}

	cfg := memclean.DefaultConfig()
	if cleanupMaxEntries > 0 {
		cfg.MaxEntries = cleanupMaxEntries
	}

	if cleanupDryRun {
		result, toRemove, err := memclean.DryRun(memDir, cfg)
		if err != nil {
			return fmt.Errorf("memory cleanup dry-run: %w", err)
		}

		fmt.Println("Dry run — no files removed")
		fmt.Printf("Memory cleanup summary: Scanned=%d Removed=%d Remaining=%d\n",
			result.Scanned, result.Removed, result.Remaining)

		if len(toRemove) > 0 {
			fmt.Println("\nWould remove:")
			for _, entry := range toRemove {
				fmt.Printf("  %s\n", entry.Path)
			}
		}

		return nil
	}

	// Full cleanup: Scan → Evaluate → Execute
	entries, err := memclean.Scan(memDir)
	if err != nil {
		return fmt.Errorf("memory cleanup scan: %w", err)
	}

	toRemove, toKeep := memclean.Evaluate(entries, cfg, time.Now())

	_, removed, err := memclean.Execute(memDir, toRemove)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", err)
	}

	remaining := len(toKeep) + (len(toRemove) - removed)
	fmt.Printf("Memory cleanup complete: Scanned=%d Removed=%d Remaining=%d\n",
		len(entries), removed, remaining)

	return nil
}

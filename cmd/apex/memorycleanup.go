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
	if cleanupMaxEntries < 0 {
		return fmt.Errorf("--max-entries must be a positive integer, got %d", cleanupMaxEntries)
	}
	if cleanupMaxEntries > 0 {
		cfg.MaxEntries = cleanupMaxEntries
	}

	if cleanupDryRun {
		result, toRemove, err := memclean.DryRun(memDir, cfg)
		if err != nil {
			return fmt.Errorf("memory cleanup dry-run: %w", err)
		}

		if result.Scanned == 0 {
			fmt.Println("Nothing to clean — memory directory is empty.")
			return nil
		}

		fmt.Println("Dry run — no files removed")
		fmt.Printf("Memory cleanup summary: Scanned=%d Removed=%d Exempted=%d Remaining=%d\n",
			result.Scanned, result.Removed, result.Exempted, result.Remaining)

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

	if len(entries) == 0 {
		fmt.Println("Nothing to clean — memory directory is empty.")
		return nil
	}

	toRemove, toKeep := memclean.Evaluate(entries, cfg, time.Now())

	_, removed, err := memclean.Execute(memDir, toRemove)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", err)
	}

	// Count exempted entries (entries in exempt categories that were kept).
	exempt := make(map[string]bool, len(cfg.ExemptCategories))
	for _, cat := range cfg.ExemptCategories {
		exempt[cat] = true
	}
	exempted := 0
	for _, entry := range toKeep {
		if exempt[entry.Category] {
			exempted++
		}
	}

	remaining := len(toKeep) + (len(toRemove) - removed)
	fmt.Printf("Memory cleanup complete: Scanned=%d Removed=%d Exempted=%d Remaining=%d\n",
		len(entries), removed, exempted, remaining)

	return nil
}

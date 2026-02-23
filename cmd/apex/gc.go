package main

import (
	"fmt"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/gc"
	"github.com/spf13/cobra"
)

var (
	gcDryRun   bool
	gcMaxAge   int
	gcMaxRuns  int
	gcMaxAudit int
)

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Clean up old runs, audit logs, and snapshots",
	Long:  "Remove old run manifests, audit log files, and stale snapshots to free disk space.",
	RunE:  runGC,
}

func init() {
	gcCmd.Flags().BoolVar(&gcDryRun, "dry-run", false, "Show what would be deleted without deleting")
	gcCmd.Flags().IntVar(&gcMaxAge, "max-age", 30, "Delete runs older than N days")
	gcCmd.Flags().IntVar(&gcMaxRuns, "max-runs", 100, "Keep at most N recent runs")
	gcCmd.Flags().IntVar(&gcMaxAudit, "max-audit", 90, "Keep audit logs for N days")
}

func runGC(cmd *cobra.Command, args []string) error {
	home, err := homeDir()
	if err != nil {
		return err
	}
	baseDir := filepath.Join(home, ".apex")

	policy := gc.Policy{
		MaxAgeDays:   gcMaxAge,
		MaxRuns:      gcMaxRuns,
		MaxAuditDays: gcMaxAudit,
		DryRun:       gcDryRun,
	}

	if gcDryRun {
		fmt.Println("[GC] Dry run mode — no files will be deleted")
	}

	result, err := gc.Run(baseDir, policy)
	if err != nil {
		return fmt.Errorf("gc failed: %w", err)
	}

	if result.RunsRemoved > 0 {
		fmt.Printf("[GC] Removed %d old runs\n", result.RunsRemoved)
	}
	if result.AuditFilesRemoved > 0 {
		fmt.Printf("[GC] Removed %d audit log files\n", result.AuditFilesRemoved)
	}

	if result.BytesFreed > 0 {
		fmt.Printf("[GC] Freed %s\n", formatBytes(result.BytesFreed))
	}

	if result.RunsRemoved == 0 && result.AuditFilesRemoved == 0 {
		fmt.Println("[GC] Nothing to clean up")
	}

	return nil
}

func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

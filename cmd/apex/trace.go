package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/audit"
	"github.com/lyndonlyu/apex/internal/manifest"
	"github.com/spf13/cobra"
)

var traceCmd = &cobra.Command{
	Use:   "trace [run-id]",
	Short: "Show causal chain for a run",
	Long:  "Display the trace (causal chain) of audit entries for a given run. If no run-id is given, uses the most recent run.",
	RunE:  showTrace,
}

func showTrace(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, ".apex")
	runsDir := filepath.Join(baseDir, "runs")
	auditDir := filepath.Join(baseDir, "audit")

	store := manifest.NewStore(runsDir)

	var m *manifest.Manifest
	var err error

	if len(args) > 0 {
		m, err = store.Load(args[0])
		if err != nil {
			return fmt.Errorf("failed to load run %s: %w", args[0], err)
		}
	} else {
		recent, recentErr := store.Recent(1)
		if recentErr != nil {
			return fmt.Errorf("failed to read runs: %w", recentErr)
		}
		if len(recent) == 0 {
			fmt.Println("No runs found.")
			return nil
		}
		m = recent[0]
	}

	if m.TraceID == "" {
		fmt.Printf("Run %s has no trace ID.\n", m.RunID)
		return nil
	}

	runIDShort := m.RunID
	if len(runIDShort) > 8 {
		runIDShort = runIDShort[:8]
	}
	traceIDShort := m.TraceID
	if len(traceIDShort) > 8 {
		traceIDShort = traceIDShort[:8]
	}

	fmt.Printf("Trace: %s (run: %s)\n\n", traceIDShort, runIDShort)

	logger, logErr := audit.NewLogger(auditDir)
	if logErr != nil {
		return fmt.Errorf("failed to open audit log: %w", logErr)
	}

	records, findErr := logger.FindByTraceID(m.TraceID)
	if findErr != nil {
		return fmt.Errorf("failed to find trace entries: %w", findErr)
	}

	if len(records) == 0 {
		fmt.Println("No audit entries found for this trace.")
		return nil
	}

	for _, r := range records {
		actionShort := r.ActionID
		if len(actionShort) > 8 {
			actionShort = actionShort[:8]
		}
		fmt.Printf("%-10s  %-40s  %-10s  %dms\n", actionShort, r.Task, r.Outcome, r.DurationMs)
	}

	return nil
}

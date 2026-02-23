package main

import (
	"fmt"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/metrics"
	"github.com/spf13/cobra"
)

var metricsFormat string

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Show system metrics",
	Long:  "Collect and display metrics from runs, DAG execution, health, and audit subsystems.",
	RunE:  showMetrics,
}

func init() {
	metricsCmd.Flags().StringVar(&metricsFormat, "format", "human", "Output format: human or jsonl")
}

func showMetrics(cmd *cobra.Command, args []string) error {
	home, err := homeDir()
	if err != nil {
		return err
	}
	baseDir := filepath.Join(home, ".apex")

	collector := metrics.NewCollector(baseDir)
	collected, err := collector.Collect()
	if err != nil {
		return fmt.Errorf("metrics collection failed: %w", err)
	}

	switch metricsFormat {
	case "jsonl":
		output, fmtErr := metrics.FormatJSONL(collected)
		if fmtErr != nil {
			return fmt.Errorf("format error: %w", fmtErr)
		}
		fmt.Print(output)
	default:
		fmt.Print(metrics.FormatHuman(collected))
	}
	return nil
}

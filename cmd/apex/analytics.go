package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/analytics"
	"github.com/lyndonlyu/apex/internal/statedb"
	"github.com/spf13/cobra"
)

var analyticsFormat string
var analyticsLimit int

var analyticsCmd = &cobra.Command{
	Use:   "analytics",
	Short: "Run history analytics",
}

var analyticsReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate analytics report",
	RunE:  runAnalyticsReport,
}

var analyticsSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show run summary",
	RunE:  runAnalyticsSummary,
}

func init() {
	analyticsReportCmd.Flags().StringVar(&analyticsFormat, "format", "", "Output format (json)")
	analyticsReportCmd.Flags().IntVar(&analyticsLimit, "limit", 100, "Number of runs to analyze")
	analyticsSummaryCmd.Flags().IntVar(&analyticsLimit, "limit", 100, "Number of runs to analyze")
	analyticsCmd.AddCommand(analyticsReportCmd, analyticsSummaryCmd)
}

func runAnalyticsReport(cmd *cobra.Command, args []string) error {
	dbPath := filepath.Join(os.Getenv("HOME"), ".apex", "runtime.db")
	db, err := statedb.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	runs, err := db.ListRuns(analyticsLimit)
	if err != nil {
		return err
	}

	report := analytics.GenerateReport(runs)
	if analyticsFormat == "json" {
		out, err := analytics.FormatReportJSON(report)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(analytics.FormatReport(report))
	}
	return nil
}

func runAnalyticsSummary(cmd *cobra.Command, args []string) error {
	dbPath := filepath.Join(os.Getenv("HOME"), ".apex", "runtime.db")
	db, err := statedb.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	runs, err := db.ListRuns(analyticsLimit)
	if err != nil {
		return err
	}

	summary := analytics.Summarize(runs)
	fmt.Print(analytics.FormatSummary(summary))
	return nil
}

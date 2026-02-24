package analytics

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatSummary returns a human-readable string for a RunSummary.
func FormatSummary(summary RunSummary) string {
	completed := summary.ByStatus["COMPLETED"]
	failed := summary.ByStatus["FAILED"]
	return fmt.Sprintf("Run Summary:\n"+
		"  Total:        %d\n"+
		"  Completed:    %d\n"+
		"  Failed:       %d\n"+
		"  Success Rate: %.1f%%\n"+
		"  Failure Rate: %.1f%%\n",
		summary.TotalRuns,
		completed,
		failed,
		summary.SuccessRate*100,
		summary.FailureRate*100,
	)
}

// FormatReport returns a full human-readable report string including
// summary, duration stats, failure patterns, and generated timestamp.
func FormatReport(report Report) string {
	var b strings.Builder

	b.WriteString(FormatSummary(report.Summary))

	fmt.Fprintf(&b, "\nDuration Stats:\n"+
		"  Runs:    %d\n"+
		"  Min:     %.1fs\n"+
		"  Max:     %.1fs\n"+
		"  Avg:     %.1fs\n"+
		"  P50:     %.1fs\n"+
		"  P90:     %.1fs\n",
		report.Duration.Count,
		report.Duration.MinSecs,
		report.Duration.MaxSecs,
		report.Duration.AvgSecs,
		report.Duration.P50Secs,
		report.Duration.P90Secs,
	)

	if len(report.Failures) > 0 {
		b.WriteString("\nFailure Patterns:\n")
		b.WriteString("  STATUS    COUNT  RATE\n")
		for _, f := range report.Failures {
			fmt.Fprintf(&b, "  %-9s %5d  %.1f%%\n", f.Status, f.Count, f.Rate*100)
		}
	}

	fmt.Fprintf(&b, "\nGenerated: %s\n", report.Generated)

	return b.String()
}

// FormatReportJSON returns the report as indented JSON.
func FormatReportJSON(report Report) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("analytics: json marshal: %w", err)
	}
	return string(data), nil
}

package progress

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// FormatProgressList formats a list of reports as a human-readable table.
func FormatProgressList(reports []ProgressReport) string {
	if len(reports) == 0 {
		return "No tasks tracked.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-20s %-15s %-9s %-12s %s\n",
		"TASK_ID", "PHASE", "PERCENT", "STATUS", "UPDATED")
	for _, r := range reports {
		fmt.Fprintf(&b, "%-20s %-15s %-9s %-12s %s\n",
			r.TaskID, r.Phase, fmt.Sprintf("%d%%", r.Percent), r.Status,
			r.UpdatedAt.Format("15:04:05"))
	}
	return b.String()
}

// FormatProgressReport formats a single progress report for display.
func FormatProgressReport(report ProgressReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Task ID:   %s\n", report.TaskID)
	fmt.Fprintf(&b, "Phase:     %s\n", report.Phase)
	fmt.Fprintf(&b, "Percent:   %d%%\n", report.Percent)
	fmt.Fprintf(&b, "Status:    %s\n", report.Status)
	fmt.Fprintf(&b, "Message:   %s\n", report.Message)
	fmt.Fprintf(&b, "Started:   %s\n", report.StartedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "Updated:   %s\n", report.UpdatedAt.Format(time.RFC3339))
	return b.String()
}

// FormatProgressListJSON formats progress reports as indented JSON.
func FormatProgressListJSON(reports []ProgressReport) (string, error) {
	data, err := json.MarshalIndent(reports, "", "  ")
	if err != nil {
		return "", fmt.Errorf("progress: json marshal: %w", err)
	}
	return string(data), nil
}

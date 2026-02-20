package event

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatQueueStats formats a QueueStats snapshot as a human-readable table.
// Columns: PRIORITY, COUNT.
func FormatQueueStats(stats QueueStats) string {
	var b strings.Builder

	// Header row.
	fmt.Fprintf(&b, "%-15s%s\n", "PRIORITY", "COUNT")

	// Data rows.
	fmt.Fprintf(&b, "%-15s%d\n", "Urgent", stats.Urgent)
	fmt.Fprintf(&b, "%-15s%d\n", "Normal", stats.Normal)
	fmt.Fprintf(&b, "%-15s%d\n", "LongRunning", stats.LongRunning)
	fmt.Fprintf(&b, "%-15s%d\n", "Total", stats.Total)

	return b.String()
}

// FormatTypes formats a sorted list of registered event types as a
// newline-separated list. If the slice is empty or nil, returns
// "No event types registered."
func FormatTypes(types []string) string {
	if len(types) == 0 {
		return "No event types registered."
	}

	var b strings.Builder
	for _, t := range types {
		fmt.Fprintf(&b, "  %s\n", t)
	}
	return b.String()
}

// FormatQueueStatsJSON formats a QueueStats snapshot as indented JSON.
func FormatQueueStatsJSON(stats QueueStats) string {
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return fmt.Sprintf("json error: %v", err)
	}
	return string(data)
}

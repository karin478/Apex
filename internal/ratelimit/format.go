package ratelimit

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatStatus formats limiter statuses as a human-readable table.
// Columns: NAME | RATE | BURST | AVAILABLE
// If empty, returns "No rate limit groups configured."
func FormatStatus(statuses []LimiterStatus) string {
	if len(statuses) == 0 {
		return "No rate limit groups configured."
	}

	// Determine column widths using header labels as minimums.
	nameW := len("NAME")
	rateW := len("RATE")
	burstW := len("BURST")
	availW := len("AVAILABLE")

	for _, s := range statuses {
		if l := len(s.Name); l > nameW {
			nameW = l
		}
		rateStr := fmt.Sprintf("%.1f/s", s.Rate)
		if l := len(rateStr); l > rateW {
			rateW = l
		}
		burstStr := fmt.Sprintf("%d", s.Burst)
		if l := len(burstStr); l > burstW {
			burstW = l
		}
		availStr := fmt.Sprintf("%.1f", s.Available)
		if l := len(availStr); l > availW {
			availW = l
		}
	}

	var b strings.Builder

	// Header row.
	rowFmt := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%s\n", nameW, rateW, burstW)
	fmt.Fprintf(&b, rowFmt, "NAME", "RATE", "BURST", "AVAILABLE")

	// Data rows.
	for _, s := range statuses {
		rateStr := fmt.Sprintf("%.1f/s", s.Rate)
		burstStr := fmt.Sprintf("%d", s.Burst)
		availStr := fmt.Sprintf("%.1f", s.Available)
		fmt.Fprintf(&b, rowFmt, s.Name, rateStr, burstStr, availStr)
	}

	return b.String()
}

// FormatStatusJSON formats limiter statuses as indented JSON.
// A nil or empty slice is rendered as "[]".
func FormatStatusJSON(statuses []LimiterStatus) string {
	if len(statuses) == 0 {
		return "[]"
	}
	data, err := json.MarshalIndent(statuses, "", "  ")
	if err != nil {
		return fmt.Sprintf("json error: %v", err)
	}
	return string(data)
}

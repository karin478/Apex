package qos

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// FormatUsage returns a human-readable summary of slot pool usage.
func FormatUsage(usage PoolUsage) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Slot Pool Usage:\n")
	fmt.Fprintf(&b, "  Total:     %d\n", usage.Total)
	fmt.Fprintf(&b, "  Used:      %d\n", usage.Used)
	fmt.Fprintf(&b, "  Available: %d\n", usage.Available)
	fmt.Fprintf(&b, "\nBy Priority:\n")
	fmt.Fprintf(&b, "  PRIORITY  ALLOCATED\n")

	if len(usage.ByPriority) == 0 {
		fmt.Fprintf(&b, "  (none)\n")
	} else {
		// Sort priorities by PriorityValue for deterministic output.
		priorities := make([]string, 0, len(usage.ByPriority))
		for p := range usage.ByPriority {
			priorities = append(priorities, p)
		}
		sort.Slice(priorities, func(i, j int) bool {
			return PriorityValue(priorities[i]) < PriorityValue(priorities[j])
		})
		for _, p := range priorities {
			fmt.Fprintf(&b, "  %-10s%d\n", p, usage.ByPriority[p])
		}
	}

	return b.String()
}

// FormatReservations returns a human-readable table of slot reservations.
func FormatReservations(reservations []Reservation) string {
	if len(reservations) == 0 {
		return "No reservations.\n"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "PRIORITY    RESERVED\n")
	for _, r := range reservations {
		fmt.Fprintf(&b, "%-12s%-8d\n", r.Priority, r.Reserved)
	}
	return b.String()
}

// FormatUsageJSON returns the pool usage as indented JSON.
func FormatUsageJSON(usage PoolUsage) (string, error) {
	data, err := json.MarshalIndent(usage, "", "  ")
	if err != nil {
		return "", fmt.Errorf("qos: json marshal: %w", err)
	}
	return string(data), nil
}

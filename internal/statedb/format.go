package statedb

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatStatus returns a human-readable summary of the database location
// and row counts for state entries and run records.
func FormatStatus(path string, stateCount, runCount int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Database: %s\n", path)
	fmt.Fprintf(&b, "State entries: %d\n", stateCount)
	fmt.Fprintf(&b, "Run records: %d\n", runCount)
	return b.String()
}

// FormatStateList returns a formatted table of state entries with columns
// KEY, VALUE, and UPDATED_AT. Returns "No state entries.\n" if the slice
// is empty.
func FormatStateList(entries []StateEntry) string {
	if len(entries) == 0 {
		return "No state entries.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-30s %-30s %-25s\n", "KEY", "VALUE", "UPDATED_AT")
	for _, e := range entries {
		fmt.Fprintf(&b, "%-30s %-30s %-25s\n", e.Key, e.Value, e.UpdatedAt)
	}
	return b.String()
}

// FormatRunList returns a formatted table of run records with columns
// ID, STATUS, TASKS, STARTED, and ENDED. Returns "No run records.\n"
// if the slice is empty.
func FormatRunList(runs []RunRecord) string {
	if len(runs) == 0 {
		return "No run records.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-20s %-12s %-8s %-22s %-22s\n", "ID", "STATUS", "TASKS", "STARTED", "ENDED")
	for _, r := range runs {
		fmt.Fprintf(&b, "%-20s %-12s %-8d %-22s %-22s\n", r.ID, r.Status, r.TaskCount, r.StartedAt, r.EndedAt)
	}
	return b.String()
}

// FormatStateListJSON returns the state entries as indented JSON.
func FormatStateListJSON(entries []StateEntry) (string, error) {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("statedb: json marshal: %w", err)
	}
	return string(data), nil
}

// FormatRunListJSON returns the run records as indented JSON.
func FormatRunListJSON(runs []RunRecord) (string, error) {
	data, err := json.MarshalIndent(runs, "", "  ")
	if err != nil {
		return "", fmt.Errorf("statedb: json marshal: %w", err)
	}
	return string(data), nil
}

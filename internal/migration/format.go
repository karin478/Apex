package migration

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatStatus returns a human-readable status string showing the current
// schema version versus the latest, plus a status message indicating whether
// migrations are pending.
func FormatStatus(current, latest int) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Schema version: %d/%d\n", current, latest)

	pending := latest - current
	if pending <= 0 {
		b.WriteString("Status: up to date\n")
	} else {
		fmt.Fprintf(&b, "Status: %d migrations pending\n", pending)
	}

	return b.String()
}

// FormatPlan formats a slice of pending migrations as a two-column table
// (VERSION, DESCRIPTION). If the slice is empty or nil, returns
// "No pending migrations."
func FormatPlan(migrations []Migration) string {
	if len(migrations) == 0 {
		return "No pending migrations."
	}

	var b strings.Builder

	// Header row.
	fmt.Fprintf(&b, "%-10s%s\n", "VERSION", "DESCRIPTION")

	// Data rows.
	for _, m := range migrations {
		fmt.Fprintf(&b, "%-10d%s\n", m.Version, m.Description)
	}

	return b.String()
}

// statusJSON is the structure for FormatStatusJSON output.
type statusJSON struct {
	Current int `json:"current"`
	Latest  int `json:"latest"`
	Pending int `json:"pending"`
}

// FormatStatusJSON returns the migration status as indented JSON containing
// current version, latest version, and pending migration count.
func FormatStatusJSON(current, latest int) string {
	s := statusJSON{
		Current: current,
		Latest:  latest,
		Pending: latest - current,
	}
	if s.Pending < 0 {
		s.Pending = 0
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Sprintf("json error: %v", err)
	}
	return string(data)
}

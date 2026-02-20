package connector

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// FormatConnectorList formats a slice of connector specs as a human-readable table.
// Columns: NAME, TYPE, BASE_URL, RISK_LEVEL.
// If the slice is empty or nil, returns "No connectors registered."
func FormatConnectorList(specs []*ConnectorSpec) string {
	if len(specs) == 0 {
		return "No connectors registered."
	}

	// Determine column widths using header labels as minimums.
	nameW := len("NAME")
	typeW := len("TYPE")
	urlW := len("BASE_URL")
	riskW := len("RISK_LEVEL")

	for _, s := range specs {
		if l := len(s.Name); l > nameW {
			nameW = l
		}
		if l := len(s.Type); l > typeW {
			typeW = l
		}
		if l := len(s.BaseURL); l > urlW {
			urlW = l
		}
		if l := len(s.RiskLevel); l > riskW {
			riskW = l
		}
	}

	var b strings.Builder

	// Header row.
	rowFmt := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%s\n", nameW, typeW, urlW)
	fmt.Fprintf(&b, rowFmt, "NAME", "TYPE", "BASE_URL", "RISK_LEVEL")

	// Data rows.
	for _, s := range specs {
		fmt.Fprintf(&b, rowFmt, s.Name, s.Type, s.BaseURL, s.RiskLevel)
	}

	return b.String()
}

// FormatBreakerStatus formats circuit breaker statuses as a human-readable table.
// Columns: NAME, STATE, FAILURES, COOLDOWN.
// If the map is empty or nil, returns "No connectors registered."
func FormatBreakerStatus(statuses map[string]CBStatus) string {
	if len(statuses) == 0 {
		return "No connectors registered."
	}

	// Collect and sort names for deterministic output.
	names := make([]string, 0, len(statuses))
	for name := range statuses {
		names = append(names, name)
	}
	sort.Strings(names)

	// Determine column widths using header labels as minimums.
	nameW := len("NAME")
	stateW := len("STATE")
	failW := len("FAILURES")
	coolW := len("COOLDOWN")

	for _, name := range names {
		s := statuses[name]
		if l := len(name); l > nameW {
			nameW = l
		}
		if l := len(string(s.State)); l > stateW {
			stateW = l
		}
		failStr := fmt.Sprintf("%d", s.Failures)
		if l := len(failStr); l > failW {
			failW = l
		}
		coolStr := s.Cooldown.String()
		if l := len(coolStr); l > coolW {
			coolW = l
		}
	}

	var b strings.Builder

	// Header row.
	rowFmt := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%s\n", nameW, stateW, failW)
	fmt.Fprintf(&b, rowFmt, "NAME", "STATE", "FAILURES", "COOLDOWN")

	// Data rows.
	for _, name := range names {
		s := statuses[name]
		failStr := fmt.Sprintf("%d", s.Failures)
		fmt.Fprintf(&b, rowFmt, name, string(s.State), failStr, s.Cooldown.String())
	}

	return b.String()
}

// FormatBreakerStatusJSON formats circuit breaker statuses as indented JSON.
// A nil or empty map is rendered as "{}".
func FormatBreakerStatusJSON(statuses map[string]CBStatus) string {
	if len(statuses) == 0 {
		return "{}"
	}
	data, err := json.MarshalIndent(statuses, "", "  ")
	if err != nil {
		return fmt.Sprintf("json error: %v", err)
	}
	return string(data)
}


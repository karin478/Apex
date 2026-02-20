package profile

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatProfileList formats a list of profiles as a human-readable table.
func FormatProfileList(profiles []Profile) string {
	if len(profiles) == 0 {
		return "No profiles registered.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-12s %-14s %-10s %-12s %s\n",
		"NAME", "MODE", "SANDBOX", "RATE_LIMIT", "CONCURRENCY")
	for _, p := range profiles {
		fmt.Fprintf(&b, "%-12s %-14s %-10s %-12s %d\n",
			p.Name, p.Mode, p.Sandbox, p.RateLimit, p.Concurrency)
	}
	return b.String()
}

// FormatProfile formats a single profile for display.
func FormatProfile(p Profile) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name:        %s\n", p.Name)
	fmt.Fprintf(&b, "Mode:        %s\n", p.Mode)
	fmt.Fprintf(&b, "Sandbox:     %s\n", p.Sandbox)
	fmt.Fprintf(&b, "Rate Limit:  %s\n", p.RateLimit)
	fmt.Fprintf(&b, "Concurrency: %d\n", p.Concurrency)
	fmt.Fprintf(&b, "Description: %s\n", p.Description)
	return b.String()
}

// FormatProfileListJSON formats profiles as indented JSON.
func FormatProfileListJSON(profiles []Profile) (string, error) {
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return "", fmt.Errorf("profile: json marshal: %w", err)
	}
	return string(data), nil
}

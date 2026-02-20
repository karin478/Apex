package mode

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatModeList formats a list of modes as a human-readable table.
func FormatModeList(modes []ModeConfig) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-15s %-14s %-13s %-16s %s\n",
		"NAME", "TOKEN_RESERVE", "CONCURRENCY", "SKIP_VALIDATION", "TIMEOUT")
	for _, m := range modes {
		fmt.Fprintf(&b, "%-15s %-14d %-13d %-16v %s\n",
			m.Name, m.TokenReserve, m.Concurrency, m.SkipValidation, m.Timeout)
	}
	return b.String()
}

// FormatModeConfig formats a single mode configuration for display.
func FormatModeConfig(config ModeConfig) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name:            %s\n", config.Name)
	fmt.Fprintf(&b, "Token Reserve:   %d\n", config.TokenReserve)
	fmt.Fprintf(&b, "Concurrency:     %d\n", config.Concurrency)
	fmt.Fprintf(&b, "Skip Validation: %v\n", config.SkipValidation)
	fmt.Fprintf(&b, "Timeout:         %s\n", config.Timeout)
	return b.String()
}

// FormatModeListJSON formats mode configs as indented JSON.
func FormatModeListJSON(modes []ModeConfig) (string, error) {
	data, err := json.MarshalIndent(modes, "", "  ")
	if err != nil {
		return "", fmt.Errorf("mode: json marshal: %w", err)
	}
	return string(data), nil
}

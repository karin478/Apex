package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatChannelList formats channel names as a list.
func FormatChannelList(names []string) string {
	if len(names) == 0 {
		return "No channels registered.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-20s\n", "CHANNEL")
	for _, name := range names {
		fmt.Fprintf(&b, "%-20s\n", name)
	}
	return b.String()
}

// FormatRuleList formats rules as a human-readable table.
func FormatRuleList(rules []Rule) string {
	if len(rules) == 0 {
		return "No rules configured.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-20s %-10s %s\n", "EVENT_TYPE", "MIN_LEVEL", "CHANNEL")
	for _, r := range rules {
		fmt.Fprintf(&b, "%-20s %-10s %s\n", r.EventType, r.MinLevel, r.Channel)
	}
	return b.String()
}

// FormatRuleListJSON formats rules as indented JSON.
func FormatRuleListJSON(rules []Rule) (string, error) {
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return "", fmt.Errorf("notify: json marshal: %w", err)
	}
	return string(data), nil
}

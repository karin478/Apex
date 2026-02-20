package datapuller

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatSourceList formats a slice of source specs as a human-readable table.
func FormatSourceList(specs []*SourceSpec) string {
	if len(specs) == 0 {
		return "No data sources configured."
	}

	nameW := len("NAME")
	urlW := len("URL")
	schedW := len("SCHEDULE")
	authW := len("AUTH")

	for _, s := range specs {
		if l := len(s.Name); l > nameW {
			nameW = l
		}
		if l := len(s.URL); l > urlW {
			urlW = l
		}
		if l := len(s.Schedule); l > schedW {
			schedW = l
		}
		if l := len(s.AuthType); l > authW {
			authW = l
		}
	}

	var b strings.Builder
	rowFmt := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%s\n", nameW, urlW, schedW)
	fmt.Fprintf(&b, rowFmt, "NAME", "URL", "SCHEDULE", "AUTH")
	for _, s := range specs {
		auth := s.AuthType
		if auth == "" {
			auth = "none"
		}
		fmt.Fprintf(&b, rowFmt, s.Name, s.URL, s.Schedule, auth)
	}
	return b.String()
}

// FormatPullResult formats a pull result as a human-readable summary.
func FormatPullResult(result PullResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Source:    %s\n", result.Source)
	fmt.Fprintf(&b, "Status:   %d\n", result.StatusCode)
	fmt.Fprintf(&b, "Bytes:    %d\n", result.RawBytes)
	if result.EventEmitted != "" {
		fmt.Fprintf(&b, "Event:    %s\n", result.EventEmitted)
	}
	fmt.Fprintf(&b, "Pulled:   %s\n", result.PulledAt.Format("2006-01-02 15:04:05"))
	if result.Error != nil {
		fmt.Fprintf(&b, "Error:    %v\n", result.Error)
	}
	return b.String()
}

// FormatSourceListJSON formats source specs as indented JSON.
func FormatSourceListJSON(specs []*SourceSpec) (string, error) {
	if len(specs) == 0 {
		return "[]", nil
	}
	data, err := json.MarshalIndent(specs, "", "  ")
	if err != nil {
		return "", fmt.Errorf("datapuller: json marshal: %w", err)
	}
	return string(data), nil
}

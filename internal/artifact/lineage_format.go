package artifact

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatImpact formats an ImpactResult as human-readable text.
// Output format:
//
//	Impact for: <root_hash[:12]>
//
//	Affected (N):
//	  <hash[:12]>
//	  <hash[:12]>
//
//	Max depth: <depth>
//
// If no affected, output: "No downstream impact for <hash[:12]>"
func FormatImpact(result *ImpactResult) string {
	rootShort := shortHash(result.RootHash)

	if len(result.Affected) == 0 {
		return fmt.Sprintf("No downstream impact for %s", rootShort)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Impact for: %s\n\n", rootShort)
	fmt.Fprintf(&b, "Affected (%d):\n", len(result.Affected))
	for _, h := range result.Affected {
		fmt.Fprintf(&b, "  %s\n", shortHash(h))
	}
	fmt.Fprintf(&b, "\nMax depth: %d", result.Depth)
	return b.String()
}

// FormatImpactJSON formats an ImpactResult as indented JSON.
func FormatImpactJSON(result *ImpactResult) string {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}
	return string(data)
}

// shortHash returns the first 12 characters of a hash, or the full string if
// it is shorter than 12 characters.
func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

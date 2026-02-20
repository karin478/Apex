package manifest

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatDiffHuman returns a human-readable table representation of a DiffResult.
// When no differences exist it returns "No differences found."
func FormatDiffHuman(d *DiffResult) string {
	if len(d.Fields) == 0 && len(d.NodeDiffs) == 0 {
		return "No differences found."
	}

	var b strings.Builder

	fmt.Fprintf(&b, "=== Run Diff: %s vs %s ===\n", d.LeftRunID, d.RightRunID)

	if len(d.Fields) > 0 {
		b.WriteString("\n")

		// Determine column widths.
		fieldW := len("Field")
		leftW := len(d.LeftRunID)
		rightW := len(d.RightRunID)

		for _, f := range d.Fields {
			if len(f.Field) > fieldW {
				fieldW = len(f.Field)
			}
			if len(f.Left) > leftW {
				leftW = len(f.Left)
			}
			if len(f.Right) > rightW {
				rightW = len(f.Right)
			}
		}

		// Header row.
		headerFmt := fmt.Sprintf("%%-%ds | %%-%ds | %%s\n", fieldW, leftW)
		fmt.Fprintf(&b, headerFmt, "Field", d.LeftRunID, d.RightRunID)

		// Separator line.
		lineLen := fieldW + 3 + leftW + 3 + rightW
		b.WriteString(strings.Repeat("-", lineLen))
		b.WriteString("\n")

		// Data rows.
		rowFmt := fmt.Sprintf("%%-%ds | %%-%ds | %%s\n", fieldW, leftW)
		for _, f := range d.Fields {
			fmt.Fprintf(&b, rowFmt, f.Field, f.Left, f.Right)
		}
	}

	if len(d.NodeDiffs) > 0 {
		b.WriteString("\nNode Differences:\n")
		for _, nd := range d.NodeDiffs {
			switch nd.Type {
			case DiffLeftOnly:
				fmt.Fprintf(&b, "  [left_only]  %s: (not in %s)\n", nd.NodeID, d.RightRunID)
			case DiffRightOnly:
				fmt.Fprintf(&b, "  [right_only] %s: (not in %s)\n", nd.NodeID, d.LeftRunID)
			case DiffChanged:
				for _, f := range nd.Fields {
					fmt.Fprintf(&b, "  [changed]    %s: %s %s → %s\n", nd.NodeID, f.Field, f.Left, f.Right)
				}
			}
		}
	}

	return b.String()
}

// FormatDiffJSON returns the DiffResult serialised as indented JSON.
func FormatDiffJSON(d *DiffResult) (string, error) {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

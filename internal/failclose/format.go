package failclose

import (
	"encoding/json"
	"fmt"
	"strings"
)

func FormatGateResult(result GateResult) string {
	var b strings.Builder
	if result.Allowed {
		fmt.Fprintf(&b, "Gate: ALLOWED\n")
	} else {
		fmt.Fprintf(&b, "Gate: BLOCKED\n")
	}
	fmt.Fprintf(&b, "\nPassed (%d):\n", len(result.Passed))
	for _, p := range result.Passed {
		fmt.Fprintf(&b, "  [PASS] %s: %s\n", p.Name, p.Reason)
	}
	if len(result.Failures) > 0 {
		fmt.Fprintf(&b, "\nFailed (%d):\n", len(result.Failures))
		for _, f := range result.Failures {
			fmt.Fprintf(&b, "  [FAIL] %s: %s\n", f.Name, f.Reason)
		}
	}
	return b.String()
}

func FormatConditionList(names []string) string {
	if len(names) == 0 {
		return "No conditions registered.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-20s\n", "CONDITION")
	for _, name := range names {
		fmt.Fprintf(&b, "%-20s\n", name)
	}
	return b.String()
}

func FormatGateResultJSON(result GateResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failclose: json marshal: %w", err)
	}
	return string(data), nil
}

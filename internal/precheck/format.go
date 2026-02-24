package precheck

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatRunResult returns a human-readable summary of the precheck results.
func FormatRunResult(result RunResult) string {
	var b strings.Builder
	b.WriteString("Environment Precheck:\n\n")
	for _, r := range result.Results {
		tag := "[PASS]"
		if !r.Passed {
			tag = "[FAIL]"
		}
		fmt.Fprintf(&b, "  %s %s: %s\n", tag, r.Name, r.Message)
	}
	b.WriteString("\n")
	verdict := "ALL PASSED"
	if !result.AllPassed {
		verdict = "SOME FAILED"
	}
	fmt.Fprintf(&b, "Result: %s (%s)\n", verdict, result.Duration)
	return b.String()
}

// FormatRunResultJSON returns the RunResult as indented JSON.
func FormatRunResultJSON(result RunResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("precheck: json marshal: %w", err)
	}
	return string(data), nil
}

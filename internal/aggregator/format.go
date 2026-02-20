package aggregator

import (
	"encoding/json"
	"fmt"
)

// FormatResult returns a human-readable string representation of a Result.
// The output format is:
//
//	Strategy: <strategy>
//
//	<output text>
//
//	Inputs: <count>
func FormatResult(result *Result) string {
	return fmt.Sprintf("Strategy: %s\n\n%s\n\nInputs: %d\n", result.Strategy, result.Output, result.InputCount)
}

// FormatResultJSON returns the Result serialized as pretty-printed JSON
// with 2-space indentation.
func FormatResultJSON(result *Result) string {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("{\"error\": %q}", err.Error())
	}
	return string(data)
}

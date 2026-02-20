package aggregator

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatResult(t *testing.T) {
	tests := []struct {
		name     string
		result   *Result
		wantHas  []string
	}{
		{
			name: "summarize strategy",
			result: &Result{
				Strategy:   StrategySummarize,
				Output:     "--- [node-1] ---\nHello from node 1\n--- [node-2] ---\nHello from node 2",
				InputCount: 2,
				CreatedAt:  time.Now().UTC(),
			},
			wantHas: []string{
				"Strategy: summarize",
				"--- [node-1] ---",
				"Hello from node 1",
				"Inputs: 2",
			},
		},
		{
			name: "merge strategy",
			result: &Result{
				Strategy:   StrategyMerge,
				Output:     "Merged 3 items from 2 inputs",
				InputCount: 2,
				CreatedAt:  time.Now().UTC(),
			},
			wantHas: []string{
				"Strategy: merge",
				"Merged 3 items from 2 inputs",
				"Inputs: 2",
			},
		},
		{
			name: "reduce strategy",
			result: &Result{
				Strategy:   StrategyReduce,
				Output:     "Count: 4, Sum: 100.00, Min: 10.00, Max: 40.00, Avg: 25.00",
				InputCount: 4,
				CreatedAt:  time.Now().UTC(),
			},
			wantHas: []string{
				"Strategy: reduce",
				"Count: 4, Sum: 100.00",
				"Inputs: 4",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatResult(tc.result)
			for _, s := range tc.wantHas {
				assert.Contains(t, got, s)
			}
		})
	}
}

func TestFormatResultJSON(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	result := &Result{
		Strategy:   StrategySummarize,
		Output:     "test output",
		InputCount: 3,
		CreatedAt:  now,
	}

	got := FormatResultJSON(result)

	// Verify it is valid JSON.
	var parsed map[string]interface{}
	err := json.Unmarshal([]byte(got), &parsed)
	require.NoError(t, err, "output should be valid JSON")

	// Verify fields are present and correct.
	assert.Equal(t, "summarize", parsed["strategy"])
	assert.Equal(t, "test output", parsed["output"])
	assert.Equal(t, float64(3), parsed["input_count"])

	// Verify 2-space indent formatting.
	assert.Contains(t, got, "  \"strategy\"")
	assert.Contains(t, got, "  \"output\"")
}

package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatDiffHumanNoDiff(t *testing.T) {
	d := &DiffResult{
		LeftRunID:  "run-A",
		RightRunID: "run-B",
	}
	got := FormatDiffHuman(d)
	assert.Equal(t, "No differences found.", got)
}

func TestFormatDiffHumanWithChanges(t *testing.T) {
	d := &DiffResult{
		LeftRunID:  "run-A",
		RightRunID: "run-B",
		Fields: []FieldDiff{
			{Field: "model", Left: "sonnet", Right: "opus"},
		},
		NodeDiffs: []NodeDiff{
			{NodeID: "n3", Type: DiffLeftOnly},
			{NodeID: "n4", Type: DiffRightOnly},
			{NodeID: "n1", Type: DiffChanged, Fields: []FieldDiff{
				{Field: "status", Left: "SUCCESS", Right: "FAILED"},
			}},
		},
	}
	got := FormatDiffHuman(d)

	// Header line.
	assert.Contains(t, got, "=== Run Diff: run-A vs run-B ===")

	// Field diff table.
	assert.Contains(t, got, "model")
	assert.Contains(t, got, "sonnet")
	assert.Contains(t, got, "opus")

	// Node diffs.
	assert.Contains(t, got, "[left_only]  n3: (not in run-B)")
	assert.Contains(t, got, "[right_only] n4: (not in run-A)")
	assert.Contains(t, got, "[changed]    n1: status SUCCESS")
	assert.Contains(t, got, "FAILED")
}

func TestFormatDiffJSON(t *testing.T) {
	d := &DiffResult{
		LeftRunID:  "run-A",
		RightRunID: "run-B",
		Fields: []FieldDiff{
			{Field: "model", Left: "sonnet", Right: "opus"},
		},
		NodeDiffs: []NodeDiff{
			{NodeID: "n1", Type: DiffChanged, Fields: []FieldDiff{
				{Field: "status", Left: "SUCCESS", Right: "FAILED"},
			}},
		},
	}

	got, err := FormatDiffJSON(d)
	require.NoError(t, err)

	// Must be valid JSON.
	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(got), &parsed)
	require.NoError(t, err)

	// Verify expected top-level keys exist.
	assert.Equal(t, "run-A", parsed["left_run_id"])
	assert.Equal(t, "run-B", parsed["right_run_id"])
	assert.NotNil(t, parsed["fields"])
	assert.NotNil(t, parsed["node_diffs"])

	// Verify field diff content.
	fields, ok := parsed["fields"].([]interface{})
	require.True(t, ok)
	require.Len(t, fields, 1)
	firstField, ok := fields[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "model", firstField["field"])
	assert.Equal(t, "sonnet", firstField["left"])
	assert.Equal(t, "opus", firstField["right"])

	// Verify node diff content.
	nodeDiffs, ok := parsed["node_diffs"].([]interface{})
	require.True(t, ok)
	require.Len(t, nodeDiffs, 1)
	firstNode, ok := nodeDiffs[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "n1", firstNode["node_id"])
	assert.Equal(t, "changed", firstNode["type"])
}

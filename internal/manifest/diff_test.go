package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// helper builds a base manifest with sensible defaults.
func baseDiffManifest(runID string) *Manifest {
	return &Manifest{
		RunID:      runID,
		Task:       "deploy",
		Timestamp:  "2026-02-20T10:00:00Z",
		Model:      "gpt-4",
		Effort:     "high",
		RiskLevel:  "low",
		NodeCount:  2,
		DurationMs: 1500,
		Outcome:    "success",
		TraceID:    "trace-abc",
		Nodes: []NodeResult{
			{ID: "n1", Task: "step-1", Status: "done", Error: ""},
			{ID: "n2", Task: "step-2", Status: "done", Error: ""},
		},
	}
}

func TestDiffIdentical(t *testing.T) {
	m := baseDiffManifest("run-1")
	result := Diff(m, m)

	assert.Equal(t, "run-1", result.LeftRunID)
	assert.Equal(t, "run-1", result.RightRunID)
	assert.Empty(t, result.Fields, "identical manifests should have no field diffs")
	assert.Empty(t, result.NodeDiffs, "identical manifests should have no node diffs")
}

func TestDiffTopLevelFields(t *testing.T) {
	left := baseDiffManifest("run-left")
	right := baseDiffManifest("run-right")

	right.Model = "gpt-3.5"
	right.Effort = "low"
	right.Outcome = "failure"

	result := Diff(left, right)

	assert.Len(t, result.Fields, 3)

	fieldMap := make(map[string]FieldDiff, len(result.Fields))
	for _, f := range result.Fields {
		fieldMap[f.Field] = f
	}

	assert.Equal(t, FieldDiff{Field: "Model", Left: "gpt-4", Right: "gpt-3.5"}, fieldMap["Model"])
	assert.Equal(t, FieldDiff{Field: "Effort", Left: "high", Right: "low"}, fieldMap["Effort"])
	assert.Equal(t, FieldDiff{Field: "Outcome", Left: "success", Right: "failure"}, fieldMap["Outcome"])

	assert.Empty(t, result.NodeDiffs, "nodes are the same so no node diffs expected")
}

func TestDiffNodeChanged(t *testing.T) {
	left := baseDiffManifest("run-left")
	right := baseDiffManifest("run-right")

	right.Nodes[0].Status = "failed"
	right.Nodes[0].Error = "timeout"

	result := Diff(left, right)

	assert.Empty(t, result.Fields, "top-level fields are the same")
	assert.Len(t, result.NodeDiffs, 1)

	nd := result.NodeDiffs[0]
	assert.Equal(t, "n1", nd.NodeID)
	assert.Equal(t, DiffChanged, nd.Type)
	assert.Len(t, nd.Fields, 2)

	fieldMap := make(map[string]FieldDiff, len(nd.Fields))
	for _, f := range nd.Fields {
		fieldMap[f.Field] = f
	}

	assert.Equal(t, FieldDiff{Field: "Status", Left: "done", Right: "failed"}, fieldMap["Status"])
	assert.Equal(t, FieldDiff{Field: "Error", Left: "", Right: "timeout"}, fieldMap["Error"])
}

func TestDiffNodeLeftOnly(t *testing.T) {
	left := baseDiffManifest("run-left")
	right := baseDiffManifest("run-right")

	// Add an extra node only in left.
	left.Nodes = append(left.Nodes, NodeResult{
		ID: "n3", Task: "step-3", Status: "done",
	})
	left.NodeCount = 3

	result := Diff(left, right)

	// NodeCount differs (3 vs 2).
	var nodeCountDiff *FieldDiff
	for _, f := range result.Fields {
		if f.Field == "NodeCount" {
			nodeCountDiff = &f
			break
		}
	}
	assert.NotNil(t, nodeCountDiff)
	assert.Equal(t, "3", nodeCountDiff.Left)
	assert.Equal(t, "2", nodeCountDiff.Right)

	// One node diff: n3 is left_only.
	assert.Len(t, result.NodeDiffs, 1)
	assert.Equal(t, "n3", result.NodeDiffs[0].NodeID)
	assert.Equal(t, DiffLeftOnly, result.NodeDiffs[0].Type)
}

func TestDiffNodeRightOnly(t *testing.T) {
	left := baseDiffManifest("run-left")
	right := baseDiffManifest("run-right")

	// Add an extra node only in right.
	right.Nodes = append(right.Nodes, NodeResult{
		ID: "n4", Task: "step-4", Status: "pending",
	})
	right.NodeCount = 3

	result := Diff(left, right)

	// NodeCount differs (2 vs 3).
	var nodeCountDiff *FieldDiff
	for _, f := range result.Fields {
		if f.Field == "NodeCount" {
			nodeCountDiff = &f
			break
		}
	}
	assert.NotNil(t, nodeCountDiff)
	assert.Equal(t, "2", nodeCountDiff.Left)
	assert.Equal(t, "3", nodeCountDiff.Right)

	// One node diff: n4 is right_only.
	assert.Len(t, result.NodeDiffs, 1)
	assert.Equal(t, "n4", result.NodeDiffs[0].NodeID)
	assert.Equal(t, DiffRightOnly, result.NodeDiffs[0].Type)
}

func TestDiffNoDifferences(t *testing.T) {
	left := baseDiffManifest("run-AAA")
	right := baseDiffManifest("run-BBB")

	// RunIDs differ but all compared content is the same.
	result := Diff(left, right)

	assert.Equal(t, "run-AAA", result.LeftRunID)
	assert.Equal(t, "run-BBB", result.RightRunID)
	assert.Empty(t, result.Fields, "same content should produce no field diffs")
	assert.Empty(t, result.NodeDiffs, "same nodes should produce no node diffs")
}

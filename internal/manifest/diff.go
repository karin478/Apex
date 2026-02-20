package manifest

import "fmt"

// FieldDiff records a single field-level difference between two manifests.
type FieldDiff struct {
	Field string `json:"field"`
	Left  string `json:"left"`
	Right string `json:"right"`
}

// DiffType classifies the kind of node-level difference.
type DiffType string

const (
	DiffChanged   DiffType = "changed"
	DiffLeftOnly  DiffType = "left_only"
	DiffRightOnly DiffType = "right_only"
)

// NodeDiff records a node-level difference between two manifests.
type NodeDiff struct {
	NodeID string      `json:"node_id"`
	Type   DiffType    `json:"type"`
	Fields []FieldDiff `json:"fields,omitempty"`
}

// DiffResult holds the complete comparison between two manifests.
type DiffResult struct {
	LeftRunID  string      `json:"left_run_id"`
	RightRunID string      `json:"right_run_id"`
	Fields     []FieldDiff `json:"fields,omitempty"`
	NodeDiffs  []NodeDiff  `json:"node_diffs,omitempty"`
}

// Diff compares two manifests and returns a DiffResult describing all
// differences. Top-level fields (Task, Model, Effort, RiskLevel, NodeCount,
// DurationMs, Outcome, TraceID) are compared by value. Nodes are matched
// by ID: nodes present in both are checked for Status/Error changes,
// nodes present only in left or right are flagged accordingly.
func Diff(left, right *Manifest) *DiffResult {
	result := &DiffResult{
		LeftRunID:  left.RunID,
		RightRunID: right.RunID,
	}

	// Compare top-level fields. Timestamp is excluded because it always
	// differs between distinct runs and does not indicate a meaningful change.
	compareField := func(name, l, r string) {
		if l != r {
			result.Fields = append(result.Fields, FieldDiff{
				Field: name,
				Left:  l,
				Right: r,
			})
		}
	}

	compareField("Task", left.Task, right.Task)
	compareField("Model", left.Model, right.Model)
	compareField("Effort", left.Effort, right.Effort)
	compareField("RiskLevel", left.RiskLevel, right.RiskLevel)
	compareField("NodeCount", fmt.Sprintf("%d", left.NodeCount), fmt.Sprintf("%d", right.NodeCount))
	compareField("DurationMs", fmt.Sprintf("%d", left.DurationMs), fmt.Sprintf("%d", right.DurationMs))
	compareField("Outcome", left.Outcome, right.Outcome)
	compareField("TraceID", left.TraceID, right.TraceID)

	// Index right nodes by ID for lookup.
	rightNodes := make(map[string]NodeResult, len(right.Nodes))
	for _, n := range right.Nodes {
		rightNodes[n.ID] = n
	}

	// Track which right-side node IDs we've visited.
	visited := make(map[string]bool, len(right.Nodes))

	// Walk left nodes: detect changed and left_only.
	for _, ln := range left.Nodes {
		rn, ok := rightNodes[ln.ID]
		if !ok {
			result.NodeDiffs = append(result.NodeDiffs, NodeDiff{
				NodeID: ln.ID,
				Type:   DiffLeftOnly,
			})
			continue
		}
		visited[ln.ID] = true

		// Compare Status and Error.
		var fields []FieldDiff
		if ln.Status != rn.Status {
			fields = append(fields, FieldDiff{
				Field: "Status",
				Left:  ln.Status,
				Right: rn.Status,
			})
		}
		if ln.Error != rn.Error {
			fields = append(fields, FieldDiff{
				Field: "Error",
				Left:  ln.Error,
				Right: rn.Error,
			})
		}
		if len(fields) > 0 {
			result.NodeDiffs = append(result.NodeDiffs, NodeDiff{
				NodeID: ln.ID,
				Type:   DiffChanged,
				Fields: fields,
			})
		}
	}

	// Detect right_only nodes.
	for _, rn := range right.Nodes {
		if !visited[rn.ID] {
			result.NodeDiffs = append(result.NodeDiffs, NodeDiff{
				NodeID: rn.ID,
				Type:   DiffRightOnly,
			})
		}
	}

	return result
}

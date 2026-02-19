package dag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDAG(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{"a"}},
		{ID: "c", Task: "task c", Depends: []string{"a"}},
		{ID: "d", Task: "task d", Depends: []string{"b", "c"}},
	}

	d, err := New(nodes)
	require.NoError(t, err)
	assert.Len(t, d.Nodes, 4)
}

func TestNewDAGCycleDetection(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{"b"}},
		{ID: "b", Task: "task b", Depends: []string{"a"}},
	}

	_, err := New(nodes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

func TestNewDAGMissingDependency(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{"nonexistent"}},
	}

	_, err := New(nodes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestReadyNodes(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{"a"}},
		{ID: "c", Task: "task c", Depends: []string{}},
	}

	d, _ := New(nodes)
	ready := d.ReadyNodes()

	ids := readyIDs(ready)
	assert.Contains(t, ids, "a")
	assert.Contains(t, ids, "c")
	assert.NotContains(t, ids, "b")
}

func TestMarkCompleted(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{"a"}},
	}

	d, _ := New(nodes)
	d.MarkRunning("a")
	assert.Equal(t, Running, d.Nodes["a"].Status)

	d.MarkCompleted("a", "done")
	assert.Equal(t, Completed, d.Nodes["a"].Status)
	assert.Equal(t, "done", d.Nodes["a"].Result)

	ready := d.ReadyNodes()
	ids := readyIDs(ready)
	assert.Contains(t, ids, "b")
}

func TestMarkFailedCascade(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{"a"}},
		{ID: "c", Task: "task c", Depends: []string{"b"}},
		{ID: "d", Task: "task d", Depends: []string{}},
	}

	d, _ := New(nodes)
	d.MarkRunning("a")
	d.MarkFailed("a", "error")

	assert.Equal(t, Failed, d.Nodes["a"].Status)
	assert.Equal(t, Failed, d.Nodes["b"].Status)
	assert.Equal(t, Failed, d.Nodes["c"].Status)
	assert.Equal(t, Pending, d.Nodes["d"].Status)
}

func TestIsComplete(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{}},
	}

	d, _ := New(nodes)
	assert.False(t, d.IsComplete())

	d.MarkRunning("a")
	d.MarkCompleted("a", "done")
	assert.False(t, d.IsComplete())

	d.MarkRunning("b")
	d.MarkCompleted("b", "done")
	assert.True(t, d.IsComplete())
}

func TestHasFailure(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
	}

	d, _ := New(nodes)
	assert.False(t, d.HasFailure())

	d.MarkRunning("a")
	d.MarkFailed("a", "error")
	assert.True(t, d.HasFailure())
}

func TestSummary(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{"a"}},
	}

	d, _ := New(nodes)
	d.MarkRunning("a")
	d.MarkCompleted("a", "done")

	summary := d.Summary()
	assert.Contains(t, summary, "a")
	assert.Contains(t, summary, "COMPLETED")
	assert.Contains(t, summary, "b")
	assert.Contains(t, summary, "PENDING")
}

func TestSingleNode(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "single", Task: "do one thing", Depends: []string{}},
	}

	d, err := New(nodes)
	require.NoError(t, err)

	ready := d.ReadyNodes()
	assert.Len(t, ready, 1)
	assert.Equal(t, "single", ready[0].ID)
}

func TestEmptyDAG(t *testing.T) {
	_, err := New([]NodeSpec{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestNodeSlice(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{"a"}},
		{ID: "c", Task: "task c", Depends: []string{"a"}},
	}
	d, _ := New(nodes)
	slice := d.NodeSlice()
	assert.Len(t, slice, 3)
	// Verify topological order: a before b, a before c
	indexMap := make(map[string]int)
	for i, n := range slice {
		indexMap[n.ID] = i
	}
	assert.Less(t, indexMap["a"], indexMap["b"])
	assert.Less(t, indexMap["a"], indexMap["c"])
}

func TestRemoveNode(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{"a"}},
		{ID: "c", Task: "task c", Depends: []string{"b"}},
		{ID: "d", Task: "task d", Depends: []string{"a"}},
	}
	d, _ := New(nodes)
	d.RemoveNode("b")

	assert.Len(t, d.Nodes, 3)
	assert.Nil(t, d.Nodes["b"])
	// c's dependency on b should be removed
	assert.NotContains(t, d.Nodes["c"].Depends, "b")
	// a and d are unchanged
	assert.NotNil(t, d.Nodes["a"])
	assert.NotNil(t, d.Nodes["d"])
}

func TestRemoveNodeNonexistent(t *testing.T) {
	nodes := []NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
	}
	d, _ := New(nodes)
	d.RemoveNode("nonexistent") // should not panic
	assert.Len(t, d.Nodes, 1)
}

func readyIDs(nodes []*Node) []string {
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	return ids
}

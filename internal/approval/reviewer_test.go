package approval

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lyndonlyu/apex/internal/dag"
	"github.com/lyndonlyu/apex/internal/governance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeNodes() []*dag.Node {
	return []*dag.Node{
		{ID: "1", Task: "migrate database schema"},
		{ID: "2", Task: "update API endpoints"},
		{ID: "3", Task: "run integration tests"},
	}
}

func classifyFunc(task string) governance.RiskLevel {
	return governance.Classify(task)
}

func TestApproveAll(t *testing.T) {
	in := strings.NewReader("a\n")
	out := &bytes.Buffer{}
	r := NewReviewer(in, out)
	result, err := r.Review(makeNodes(), classifyFunc)
	require.NoError(t, err)
	assert.True(t, result.Approved)
	assert.Len(t, result.Nodes, 3)
	for _, nd := range result.Nodes {
		assert.Equal(t, Approved, nd.Decision)
	}
	assert.Contains(t, out.String(), "Approval Required")
}

func TestQuitRejectsAll(t *testing.T) {
	in := strings.NewReader("q\n")
	out := &bytes.Buffer{}
	r := NewReviewer(in, out)
	result, err := r.Review(makeNodes(), classifyFunc)
	require.NoError(t, err)
	assert.False(t, result.Approved)
	for _, nd := range result.Nodes {
		assert.Equal(t, Rejected, nd.Decision)
	}
}

func TestReviewOneByOne(t *testing.T) {
	// r to review, then: approve node 1, skip node 2, approve node 3
	in := strings.NewReader("r\na\ns\na\n")
	out := &bytes.Buffer{}
	r := NewReviewer(in, out)
	result, err := r.Review(makeNodes(), classifyFunc)
	require.NoError(t, err)
	assert.True(t, result.Approved)
	assert.Equal(t, Approved, result.Nodes[0].Decision)
	assert.Equal(t, Skipped, result.Nodes[1].Decision)
	assert.Equal(t, Approved, result.Nodes[2].Decision)
}

func TestReviewRejectMidway(t *testing.T) {
	// r to review, approve node 1, then reject all
	in := strings.NewReader("r\na\nr\n")
	out := &bytes.Buffer{}
	r := NewReviewer(in, out)
	result, err := r.Review(makeNodes(), classifyFunc)
	require.NoError(t, err)
	assert.False(t, result.Approved)
	assert.Equal(t, Approved, result.Nodes[0].Decision)
	assert.Equal(t, Rejected, result.Nodes[1].Decision)
	assert.Equal(t, Rejected, result.Nodes[2].Decision)
}

func TestSkipAllNodes(t *testing.T) {
	// r to review, skip all 3 nodes — should be Approved (not rejected)
	in := strings.NewReader("r\ns\ns\ns\n")
	out := &bytes.Buffer{}
	r := NewReviewer(in, out)
	result, err := r.Review(makeNodes(), classifyFunc)
	require.NoError(t, err)
	assert.True(t, result.Approved) // skip-all is not rejection
	for _, nd := range result.Nodes {
		assert.Equal(t, Skipped, nd.Decision)
	}
}

func TestEmptyNodeList(t *testing.T) {
	in := strings.NewReader("")
	out := &bytes.Buffer{}
	r := NewReviewer(in, out)
	result, err := r.Review([]*dag.Node{}, classifyFunc)
	require.NoError(t, err)
	assert.True(t, result.Approved)
	assert.Empty(t, result.Nodes)
}

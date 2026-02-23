package planner

import (
	"testing"

	"github.com/lyndonlyu/apex/internal/dag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNodes(t *testing.T) {
	raw := `[{"id":"step1","task":"analyze code","depends":[]},{"id":"step2","task":"refactor","depends":["step1"]}]`

	nodes, err := ParseNodes(raw)
	require.NoError(t, err)
	assert.Len(t, nodes, 2)
	assert.Equal(t, "step1", nodes[0].ID)
	assert.Equal(t, "analyze code", nodes[0].Task)
	assert.Empty(t, nodes[0].Depends)
	assert.Equal(t, "step2", nodes[1].ID)
	assert.Equal(t, []string{"step1"}, nodes[1].Depends)
}

func TestParseNodesInvalid(t *testing.T) {
	_, err := ParseNodes("not json")
	assert.Error(t, err)
}

func TestParseNodesEmpty(t *testing.T) {
	_, err := ParseNodes("[]")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestParseNodesFromWrappedJSON(t *testing.T) {
	raw := "```json\n[{\"id\":\"a\",\"task\":\"do thing\",\"depends\":[]}]\n```"

	nodes, err := ParseNodes(raw)
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
}

func TestIsSimpleTask(t *testing.T) {
	// English simple tasks
	assert.True(t, IsSimpleTask("explain this function"))
	assert.True(t, IsSimpleTask("read the README"))
	assert.True(t, IsSimpleTask("run the tests"))

	// English multi-step tasks
	assert.False(t, IsSimpleTask("refactor the auth module and then update all tests and deploy"))
	assert.False(t, IsSimpleTask("first analyze the code, then refactor, after that write tests"))

	// Additional multi-step patterns
	assert.False(t, IsSimpleTask("analyze deps, afterward generate a chart and finally deploy"))
	assert.False(t, IsSimpleTask("once setup is done, configure the database subsequently run migrations"))
	assert.False(t, IsSimpleTask("step 1 analyze, step 2 build, step 3 test"))
	assert.False(t, IsSimpleTask("create the directory, then create files, lastly write the index"))
}

func TestBuildPlannerPrompt(t *testing.T) {
	prompt := BuildPlannerPrompt("refactor auth and update tests")
	assert.Contains(t, prompt, "refactor auth and update tests")
	assert.Contains(t, prompt, "JSON")
	assert.Contains(t, prompt, "id")
	assert.Contains(t, prompt, "task")
	assert.Contains(t, prompt, "depends")
}

func TestSingleNodeFallback(t *testing.T) {
	nodes := SingleNodeFallback("simple task")
	assert.Len(t, nodes, 1)
	assert.Equal(t, "task", nodes[0].ID)
	assert.Equal(t, "simple task", nodes[0].Task)
	assert.Empty(t, nodes[0].Depends)
}

// Verify that SingleNodeFallback returns a valid dag.NodeSpec slice.
func TestSingleNodeFallbackType(t *testing.T) {
	nodes := SingleNodeFallback("type check")
	var _ []dag.NodeSpec = nodes // compile-time type check
	assert.Equal(t, "task", nodes[0].ID)
}

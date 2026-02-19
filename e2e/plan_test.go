package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlanSimpleTask verifies that a simple task produces a single-step
// execution plan without calling the LLM planner.
func TestPlanSimpleTask(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("plan", "say hello")

	require.Equal(t, 0, exitCode, "expected exit 0; stderr: %s", stderr)
	assert.Contains(t, stdout, "Execution Plan")
	assert.Contains(t, stdout, "1 steps")
}

// TestPlanComplexTask verifies that a complex multi-step task is decomposed
// into multiple steps via the planner. The mock planner response is injected
// through MOCK_PLANNER_RESPONSE to return a 3-step DAG.
func TestPlanComplexTask(t *testing.T) {
	env := newTestEnv(t)

	mockPlannerResponse := `[{"id":"analyze","task":"analyze codebase","depends":[]},{"id":"refactor","task":"refactor module","depends":["analyze"]},{"id":"test","task":"run tests","depends":["refactor"]}]`

	stdout, stderr, exitCode := env.runApexWithEnv(
		map[string]string{
			"MOCK_PLANNER_RESPONSE": mockPlannerResponse,
		},
		"plan", "first analyze then refactor and test",
	)

	require.Equal(t, 0, exitCode, "expected exit 0; stderr: %s", stderr)
	assert.Contains(t, stdout, "3 steps")
	assert.Contains(t, stdout, "analyze")
	assert.Contains(t, stdout, "refactor")
}

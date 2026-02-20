package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregateFromFile(t *testing.T) {
	env := newTestEnv(t)

	// Create a temp JSON file with two summarize inputs.
	inputs := []map[string]interface{}{
		{"node_id": "n1", "content": "Hello from node 1", "data": nil},
		{"node_id": "n2", "content": "Hello from node 2", "data": nil},
	}
	data, err := json.Marshal(inputs)
	require.NoError(t, err)

	filePath := filepath.Join(env.Home, "summarize_input.json")
	require.NoError(t, os.WriteFile(filePath, data, 0644))

	stdout, stderr, exitCode := env.runApex("aggregate", "--strategy", "summarize", "--file", filePath)

	assert.Equal(t, 0, exitCode, "apex aggregate --strategy summarize should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "n1", "output should contain node ID n1")
	assert.Contains(t, stdout, "n2", "output should contain node ID n2")
	assert.Contains(t, stdout, "Hello from node 1", "output should contain content from node 1")
	assert.Contains(t, stdout, "Hello from node 2", "output should contain content from node 2")
}

func TestAggregateReduce(t *testing.T) {
	env := newTestEnv(t)

	// Create a temp JSON file with three numeric data inputs.
	inputs := []map[string]interface{}{
		{"node_id": "n1", "content": "", "data": 10},
		{"node_id": "n2", "content": "", "data": 20},
		{"node_id": "n3", "content": "", "data": 30},
	}
	data, err := json.Marshal(inputs)
	require.NoError(t, err)

	filePath := filepath.Join(env.Home, "reduce_input.json")
	require.NoError(t, os.WriteFile(filePath, data, 0644))

	stdout, stderr, exitCode := env.runApex("aggregate", "--strategy", "reduce", "--file", filePath)

	assert.Equal(t, 0, exitCode, "apex aggregate --strategy reduce should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "Count: 3", "output should contain Count: 3")
	assert.Contains(t, stdout, "Sum: 60", "output should contain Sum: 60")
}

func TestAggregateInvalidStrategy(t *testing.T) {
	env := newTestEnv(t)

	// Create a minimal input file (content doesn't matter, strategy validation happens first).
	inputs := []map[string]interface{}{
		{"node_id": "n1", "content": "test", "data": nil},
	}
	data, err := json.Marshal(inputs)
	require.NoError(t, err)

	filePath := filepath.Join(env.Home, "invalid_input.json")
	require.NoError(t, os.WriteFile(filePath, data, 0644))

	_, stderr, exitCode := env.runApex("aggregate", "--strategy", "invalid", "--file", filePath)

	assert.NotEqual(t, 0, exitCode, "apex aggregate --strategy invalid should exit non-zero")
	assert.Contains(t, stderr, "unknown strategy", "stderr should mention 'unknown strategy'")
}

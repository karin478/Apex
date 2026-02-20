package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKGListEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("kg", "list")

	assert.Equal(t, 0, exitCode, "apex kg list should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "No entities found", "empty graph should report no entities")
}

func TestKGQueryNotFound(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("kg", "query", "nonexistent")

	assert.Equal(t, 0, exitCode, "apex kg query should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "No entities matching", "query for nonexistent name should report no matches")
}

func TestKGStats(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("kg", "stats")

	assert.Equal(t, 0, exitCode, "apex kg stats should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "Entities:      0", "empty graph should show 0 entities")
	assert.Contains(t, stdout, "Relationships: 0", "empty graph should show 0 relationships")
}

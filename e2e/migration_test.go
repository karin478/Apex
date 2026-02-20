package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMigrationStatus verifies that "apex migration status" prints a
// "Schema version" line and exits 0.
func TestMigrationStatus(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("migration", "status")

	assert.Equal(t, 0, exitCode,
		"apex migration status should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "Schema version"),
		"stdout should contain 'Schema version', got: %s", stdout)
}

// TestMigrationPlanEmpty verifies that "apex migration plan" with no
// pending migrations prints "no pending" (case-insensitive) and exits 0.
func TestMigrationPlanEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("migration", "plan")

	assert.Equal(t, 0, exitCode,
		"apex migration plan should exit 0; stderr=%s", stderr)

	lower := strings.ToLower(stdout)
	assert.True(t, strings.Contains(lower, "no pending"),
		"stdout should contain 'no pending' (case-insensitive), got: %s", stdout)
}

// TestMigrationStatusRuns verifies that "apex migration status" executes
// cleanly and exits with code 0.
func TestMigrationStatusRuns(t *testing.T) {
	env := newTestEnv(t)

	_, stderr, exitCode := env.runApex("migration", "status")

	assert.Equal(t, 0, exitCode,
		"apex migration status should exit 0; stderr=%s", stderr)
}

package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStateDBStatus(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("statedb", "status")

	assert.Equal(t, 0, exitCode,
		"apex statedb status should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "Database:"),
		"stdout should contain Database:, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "State entries:"),
		"stdout should contain State entries:, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "Run records:"),
		"stdout should contain Run records:, got: %s", stdout)
}

func TestStateDBStateList(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("statedb", "state", "list")

	assert.Equal(t, 0, exitCode,
		"apex statedb state list should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "No state entries"),
		"stdout should contain No state entries, got: %s", stdout)
}

func TestStateDBRunsList(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("statedb", "runs", "list")

	assert.Equal(t, 0, exitCode,
		"apex statedb runs list should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "No run records"),
		"stdout should contain No run records, got: %s", stdout)
}

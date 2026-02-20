package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProgressList(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("progress", "list")

	assert.Equal(t, 0, exitCode,
		"apex progress list should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "No tasks tracked"),
		"stdout should indicate no tasks, got: %s", stdout)
}

func TestProgressStart(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("progress", "start", "test-task", "--phase", "build")

	assert.Equal(t, 0, exitCode,
		"apex progress start should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "test-task"),
		"stdout should contain task ID, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "build"),
		"stdout should contain phase name, got: %s", stdout)
}

func TestProgressShowNotFound(t *testing.T) {
	env := newTestEnv(t)

	_, _, exitCode := env.runApex("progress", "show", "nonexistent-task")

	assert.NotEqual(t, 0, exitCode,
		"apex progress show with nonexistent task should exit non-zero")
}

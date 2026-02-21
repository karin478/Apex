package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrecheck(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("precheck")

	assert.Equal(t, 0, exitCode,
		"apex precheck should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "Environment Precheck"),
		"stdout should contain 'Environment Precheck', got: %s", stdout)
}

func TestPrecheckJSON(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("precheck", "--format", "json")

	assert.Equal(t, 0, exitCode,
		"apex precheck --format json should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "all_passed"),
		"stdout should contain 'all_passed', got: %s", stdout)
}

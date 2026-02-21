package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGateCheck(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("gate", "check")

	assert.Equal(t, 0, exitCode,
		"apex gate check should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "ALLOWED"),
		"stdout should contain ALLOWED, got: %s", stdout)
}

func TestGateCheckJSON(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("gate", "check", "--format", "json")

	assert.Equal(t, 0, exitCode,
		"apex gate check --format json should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "allowed"),
		"stdout should contain allowed JSON key, got: %s", stdout)
}

func TestGateList(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("gate", "list")

	assert.Equal(t, 0, exitCode,
		"apex gate list should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "health"),
		"stdout should contain health condition, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "killswitch"),
		"stdout should contain killswitch condition, got: %s", stdout)
}

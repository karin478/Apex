package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProfileList(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("profile", "list")

	assert.Equal(t, 0, exitCode,
		"apex profile list should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "dev"),
		"stdout should contain dev, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "staging"),
		"stdout should contain staging, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "prod"),
		"stdout should contain prod, got: %s", stdout)
}

func TestProfileActivate(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("profile", "activate", "dev")

	assert.Equal(t, 0, exitCode,
		"apex profile activate dev should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "dev"),
		"stdout should contain dev, got: %s", stdout)
}

func TestProfileShow(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("profile", "show", "prod")

	assert.Equal(t, 0, exitCode,
		"apex profile show prod should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "prod"),
		"stdout should contain prod, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "BATCH"),
		"stdout should contain BATCH mode, got: %s", stdout)
}

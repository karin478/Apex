package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModeList(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("mode", "list")

	assert.Equal(t, 0, exitCode,
		"apex mode list should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "NORMAL"),
		"stdout should contain NORMAL, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "URGENT"),
		"stdout should contain URGENT, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "EXPLORATORY"),
		"stdout should contain EXPLORATORY, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "BATCH"),
		"stdout should contain BATCH, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "LONG_RUNNING"),
		"stdout should contain LONG_RUNNING, got: %s", stdout)
}

func TestModeSelect(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("mode", "select", "URGENT")

	assert.Equal(t, 0, exitCode,
		"apex mode select URGENT should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "URGENT"),
		"stdout should contain URGENT, got: %s", stdout)
}

func TestModeConfig(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("mode", "config", "BATCH")

	assert.Equal(t, 0, exitCode,
		"apex mode config BATCH should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "BATCH"),
		"stdout should contain BATCH, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "8"),
		"stdout should contain concurrency 8, got: %s", stdout)
}

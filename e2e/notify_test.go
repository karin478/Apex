package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotifyChannels(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("notify", "channels")

	assert.Equal(t, 0, exitCode,
		"apex notify channels should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "stdout"),
		"stdout should contain 'stdout' channel, got: %s", stdout)
}

func TestNotifySend(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("notify", "send", "test.event", "hello world")

	assert.Equal(t, 0, exitCode,
		"apex notify send should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "test.event"),
		"stdout should contain event type, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "hello world"),
		"stdout should contain message, got: %s", stdout)
}

func TestNotifyList(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("notify", "list")

	assert.Equal(t, 0, exitCode,
		"apex notify list should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "EVENT_TYPE"),
		"stdout should contain EVENT_TYPE header, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "stdout"),
		"stdout should contain stdout channel in rules, got: %s", stdout)
}

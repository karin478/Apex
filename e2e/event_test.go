package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEventQueueStats verifies that "apex event queue" prints queue
// statistics with a PRIORITY/COUNT header and a Total summary row.
func TestEventQueueStats(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("event", "queue")

	assert.Equal(t, 0, exitCode,
		"apex event queue should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "PRIORITY"),
		"stdout should contain 'PRIORITY', got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "Total"),
		"stdout should contain 'Total', got: %s", stdout)
}

// TestEventTypesEmpty verifies that "apex event types" with no registered
// types prints "no event types" (case-insensitive) and exits 0.
func TestEventTypesEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("event", "types")

	assert.Equal(t, 0, exitCode,
		"apex event types should exit 0; stderr=%s", stderr)

	lower := strings.ToLower(stdout)
	assert.True(t, strings.Contains(lower, "no event types"),
		"stdout should contain 'no event types' (case-insensitive), got: %s", stdout)
}

// TestEventQueueRuns verifies that "apex event queue" executes cleanly
// and exits with code 0.
func TestEventQueueRuns(t *testing.T) {
	env := newTestEnv(t)

	_, stderr, exitCode := env.runApex("event", "queue")

	assert.Equal(t, 0, exitCode,
		"apex event queue should exit 0; stderr=%s", stderr)
}

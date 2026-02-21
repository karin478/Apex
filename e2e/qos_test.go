package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestQoSStatus verifies that "apex qos status" prints slot pool usage
// information and exits cleanly.
func TestQoSStatus(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("qos", "status")

	assert.Equal(t, 0, exitCode,
		"apex qos status should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "Slot Pool Usage"),
		"stdout should contain Slot Pool Usage, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "Total"),
		"stdout should contain Total, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "Available"),
		"stdout should contain Available, got: %s", stdout)
}

// TestQoSReservations verifies that "apex qos reservations" lists the
// default URGENT and HIGH priority reservations.
func TestQoSReservations(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("qos", "reservations")

	assert.Equal(t, 0, exitCode,
		"apex qos reservations should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "URGENT"),
		"stdout should contain URGENT, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "HIGH"),
		"stdout should contain HIGH, got: %s", stdout)
}

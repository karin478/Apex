package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRateLimitStatusEmpty verifies that "apex ratelimit status" with no
// groups configured prints the empty-state message and exits cleanly.
func TestRateLimitStatusEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("ratelimit", "status")

	assert.Equal(t, 0, exitCode, "apex ratelimit status should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "No rate limit groups"),
		"stdout should contain 'No rate limit groups', got: %s", stdout)
}

// TestRateLimitStatusHelp verifies that "apex ratelimit --help" lists the
// "status" subcommand.
func TestRateLimitStatusHelp(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("ratelimit", "--help")

	assert.Equal(t, 0, exitCode, "apex ratelimit --help should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "status"),
		"help output should mention 'status' subcommand, got: %s", stdout)
}

// TestRateLimitStatusRuns verifies that "apex ratelimit status --format json"
// returns an empty JSON array and exits cleanly.
func TestRateLimitStatusRuns(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("ratelimit", "status", "--format", "json")

	assert.Equal(t, 0, exitCode, "apex ratelimit status --format json should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "[]"),
		"stdout should contain '[]' for empty JSON output, got: %s", stdout)
}

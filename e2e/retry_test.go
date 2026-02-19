package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRetrySuccessOnSecondAttempt verifies that a retriable failure (exit 1 +
// "timeout error") is retried and succeeds on the second attempt.
// MOCK_FAIL_COUNT=1 makes the mock fail the first call, then succeed.
// Simple task "say hello" ensures the planner uses SingleNodeFallback and
// does NOT call the mock, so only the executor increments the counter.
func TestRetrySuccessOnSecondAttempt(t *testing.T) {
	env := newTestEnv(t)

	counterFile := filepath.Join(env.Home, "retry_counter")

	stdout, _, exitCode := env.runApexWithEnv(
		map[string]string{
			"MOCK_FAIL_COUNT":  "1",
			"MOCK_COUNTER_FILE": counterFile,
			"MOCK_EXIT_CODE":   "1",
			"MOCK_STDERR":      "timeout error",
		},
		"run", "say hello",
	)

	require.Equal(t, 0, exitCode, "should succeed after retry")
	assert.Contains(t, stdout, "Done", "stdout should contain Done on success")
}

// TestRetryExhausted verifies that when all retry attempts fail, the command
// exits with a non-zero code. MOCK_FAIL_COUNT=100 ensures every call fails.
// With max_attempts=3 in config, the executor tries 3 times and gives up.
func TestRetryExhausted(t *testing.T) {
	env := newTestEnv(t)

	counterFile := filepath.Join(env.Home, "retry_counter")

	_, _, exitCode := env.runApexWithEnv(
		map[string]string{
			"MOCK_FAIL_COUNT":  "100",
			"MOCK_COUNTER_FILE": counterFile,
			"MOCK_EXIT_CODE":   "1",
			"MOCK_STDERR":      "timeout error",
		},
		"run", "say hello",
	)

	assert.NotEqual(t, 0, exitCode, "should fail after exhausting all retry attempts")
}

// TestNonRetriableDoesNotRetry verifies that a non-retriable error (exit code 2
// + "permission denied") causes immediate failure without retrying.
// The counter file should show exactly 1 call — no retries attempted.
func TestNonRetriableDoesNotRetry(t *testing.T) {
	env := newTestEnv(t)

	counterFile := filepath.Join(env.Home, "retry_counter")

	_, _, exitCode := env.runApexWithEnv(
		map[string]string{
			"MOCK_FAIL_COUNT":  "100",
			"MOCK_COUNTER_FILE": counterFile,
			"MOCK_EXIT_CODE":   "2",
			"MOCK_STDERR":      "permission denied",
		},
		"run", "say hello",
	)

	assert.NotEqual(t, 0, exitCode, "should fail immediately for non-retriable error")

	// Read counter file to verify only 1 call was made (no retries).
	data, err := os.ReadFile(counterFile)
	require.NoError(t, err, "counter file should exist")

	count := strings.TrimSpace(string(data))
	assert.Equal(t, "1", count, "non-retriable error should not trigger retries — expected 1 call")
}

package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnalyticsReport(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("analytics", "report")

	assert.Equal(t, 0, exitCode,
		"apex analytics report should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "Run Summary"),
		"stdout should contain 'Run Summary', got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "Total"),
		"stdout should contain 'Total', got: %s", stdout)
}

func TestAnalyticsSummary(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("analytics", "summary")

	assert.Equal(t, 0, exitCode,
		"apex analytics summary should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "Run Summary"),
		"stdout should contain 'Run Summary', got: %s", stdout)
}

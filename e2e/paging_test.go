package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextBudget(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("paging", "budget")

	assert.Equal(t, 0, exitCode,
		"apex paging budget should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "PAGES"),
		"stdout should contain PAGES, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "10"),
		"stdout should contain default max pages 10, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "8000"),
		"stdout should contain default max tokens 8000, got: %s", stdout)
}

func TestContextBudgetJSON(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("paging", "budget", "--format", "json")

	assert.Equal(t, 0, exitCode,
		"apex paging budget --format json should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "max_pages"),
		"stdout should contain max_pages JSON key, got: %s", stdout)
}

func TestContextPageNotFound(t *testing.T) {
	env := newTestEnv(t)

	_, _, exitCode := env.runApex("paging", "fetch", "nonexistent-artifact")

	assert.NotEqual(t, 0, exitCode,
		"apex paging fetch with nonexistent artifact should exit non-zero")
}

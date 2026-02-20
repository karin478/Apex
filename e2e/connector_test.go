package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectorList verifies that "apex connector list" discovers YAML
// connector specs placed in $HOME/.claude/connectors/ and includes their
// names in the output.
func TestConnectorList(t *testing.T) {
	env := newTestEnv(t)

	// Create the connectors directory inside the temp HOME.
	connectorsDir := filepath.Join(env.Home, ".claude", "connectors")
	require.NoError(t, os.MkdirAll(connectorsDir, 0755),
		"creating .claude/connectors should not fail")

	// Write a minimal connector spec YAML file.
	specContent := `name: test-api
type: http_api
spec_version: "1.0"
base_url: https://api.example.com
risk_level: LOW
`
	specPath := filepath.Join(connectorsDir, "test-api.yaml")
	require.NoError(t, os.WriteFile(specPath, []byte(specContent), 0644),
		"writing connector spec YAML should not fail")

	stdout, stderr, exitCode := env.runApex("connector", "list")

	assert.Equal(t, 0, exitCode,
		"apex connector list should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "test-api"),
		"stdout should contain connector name 'test-api', got: %s", stdout)
}

// TestConnectorStatusEmpty verifies that "apex connector status" with no
// connectors directory prints "No connectors registered." and exits 0.
func TestConnectorStatusEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("connector", "status")

	assert.Equal(t, 0, exitCode,
		"apex connector status should exit 0; stderr=%s", stderr)

	lower := strings.ToLower(stdout)
	assert.True(t, strings.Contains(lower, "no connectors"),
		"stdout should contain 'No connectors' (case-insensitive), got: %s", stdout)
}

// TestConnectorStatusRuns verifies that "apex connector status" executes
// cleanly and exits with code 0.
func TestConnectorStatusRuns(t *testing.T) {
	env := newTestEnv(t)

	_, stderr, exitCode := env.runApex("connector", "status")

	assert.Equal(t, 0, exitCode,
		"apex connector status should exit 0; stderr=%s", stderr)
}

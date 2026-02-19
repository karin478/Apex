//go:build live

package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newLiveEnv creates a test environment configured for the real Claude CLI.
// It starts from newTestEnv and overwrites config.yaml so that no mock binary
// is used — the executor defaults to the real "claude" command.
func newLiveEnv(t *testing.T) *TestEnv {
	t.Helper()

	env := newTestEnv(t)

	// Overwrite config.yaml with real-Claude settings (no binary field).
	configContent := `claude:
  model: "claude-sonnet-4-5-20250514"
  effort: "low"
  timeout: 60
planner:
  model: "claude-sonnet-4-5-20250514"
  timeout: 30
pool:
  max_concurrent: 1
retry:
  max_attempts: 1
  init_delay_seconds: 1
  multiplier: 2.0
  max_delay_seconds: 10
`
	configPath := filepath.Join(env.Home, ".apex", "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write live config.yaml: %v", err)
	}

	return env
}

// ---------------------------------------------------------------------------
// Live smoke tests — require -tags=live to compile
// ---------------------------------------------------------------------------

// TestLiveVersion runs "apex version" against the real binary.
// Does NOT need a real Claude installation, just basic CLI plumbing.
func TestLiveVersion(t *testing.T) {
	env := newLiveEnv(t)

	stdout, stderr, exitCode := env.runApex("version")

	require.Equal(t, 0, exitCode,
		fmt.Sprintf("apex version exited %d; stderr: %s", exitCode, stderr))
	assert.Contains(t, stdout, "apex v",
		"expected 'apex v' in version output, got: %s", stdout)
}

// TestLiveDoctor runs "apex doctor" against the real binary.
// Does NOT need a real Claude installation, just basic CLI plumbing.
func TestLiveDoctor(t *testing.T) {
	env := newLiveEnv(t)

	stdout, stderr, exitCode := env.runApex("doctor")

	require.Equal(t, 0, exitCode,
		fmt.Sprintf("apex doctor exited %d; stderr: %s", exitCode, stderr))
	assert.Contains(t, stdout, "Apex Doctor",
		"expected 'Apex Doctor' in output, got: %s", stdout)
}

// TestLiveRunSimple runs a real Claude task via "apex run".
// Skipped unless APEX_LIVE_TESTS is set (costs real tokens).
func TestLiveRunSimple(t *testing.T) {
	if os.Getenv("APEX_LIVE_TESTS") == "" {
		t.Skip("Set APEX_LIVE_TESTS=1 to run live Claude tests (costs tokens)")
	}

	env := newLiveEnv(t)

	stdout, stderr, exitCode := env.runApex("run", "echo hello world to stdout using a shell command")

	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)

	require.Equal(t, 0, exitCode,
		fmt.Sprintf("apex run exited %d; stderr: %s", exitCode, stderr))
}

// TestLivePlan runs a real Claude planning task via "apex plan".
// Skipped unless APEX_LIVE_TESTS is set (costs real tokens).
func TestLivePlan(t *testing.T) {
	if os.Getenv("APEX_LIVE_TESTS") == "" {
		t.Skip("Set APEX_LIVE_TESTS=1 to run live Claude tests (costs tokens)")
	}

	env := newLiveEnv(t)

	stdout, stderr, exitCode := env.runApex("plan", "create a hello world Go program")

	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)

	require.Equal(t, 0, exitCode,
		fmt.Sprintf("apex plan exited %d; stderr: %s", exitCode, stderr))
	assert.True(t, strings.Contains(stdout, "Execution Plan"),
		"expected 'Execution Plan' in output, got: %s", stdout)
}

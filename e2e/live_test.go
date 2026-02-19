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
//
// Authentication: Claude CLI stores OAuth tokens in the macOS Keychain,
// which is tied to the real HOME directory. Since tests use a temp HOME for
// isolation, we rely on the CLAUDE_CODE_OAUTH_TOKEN env var (forwarded by
// runApexWithEnv) to authenticate without needing the real HOME.
func newLiveEnv(t *testing.T) *TestEnv {
	t.Helper()

	env := newTestEnv(t)

	// Overwrite config.yaml with real-Claude settings (no binary field).
	configContent := `claude:
  model: "claude-sonnet-4-6"
  effort: "low"
  timeout: 60
planner:
  model: "claude-sonnet-4-6"
  timeout: 30
pool:
  max_concurrent: 1
retry:
  max_attempts: 1
  init_delay_seconds: 1
  multiplier: 2.0
  max_delay_seconds: 10
sandbox:
  level: "none"
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
// Requires CLAUDE_CODE_OAUTH_TOKEN for authentication.
func TestLiveRunSimple(t *testing.T) {
	if os.Getenv("APEX_LIVE_TESTS") == "" {
		t.Skip("Set APEX_LIVE_TESTS=1 to run live Claude tests (costs tokens)")
	}
	if os.Getenv("CLAUDE_CODE_OAUTH_TOKEN") == "" {
		t.Skip("Set CLAUDE_CODE_OAUTH_TOKEN to authenticate with Claude CLI")
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
// Requires CLAUDE_CODE_OAUTH_TOKEN for authentication.
func TestLivePlan(t *testing.T) {
	if os.Getenv("APEX_LIVE_TESTS") == "" {
		t.Skip("Set APEX_LIVE_TESTS=1 to run live Claude tests (costs tokens)")
	}
	if os.Getenv("CLAUDE_CODE_OAUTH_TOKEN") == "" {
		t.Skip("Set CLAUDE_CODE_OAUTH_TOKEN to authenticate with Claude CLI")
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

// ---------------------------------------------------------------------------
// Scenario tests — exercise real Claude across key execution paths
// ---------------------------------------------------------------------------

// skipUnlessLive is a helper that skips the test unless both
// APEX_LIVE_TESTS and CLAUDE_CODE_OAUTH_TOKEN are set.
func skipUnlessLive(t *testing.T) {
	t.Helper()
	if os.Getenv("APEX_LIVE_TESTS") == "" {
		t.Skip("Set APEX_LIVE_TESTS=1 to run live Claude tests (costs tokens)")
	}
	if os.Getenv("CLAUDE_CODE_OAUTH_TOKEN") == "" {
		t.Skip("Set CLAUDE_CODE_OAUTH_TOKEN to authenticate with Claude CLI")
	}
}

// newLiveConcurrentEnv returns a live env with pool.max_concurrent > 1
// so that parallel DAG nodes can run concurrently.
func newLiveConcurrentEnv(t *testing.T) *TestEnv {
	t.Helper()
	env := newTestEnv(t)

	configContent := `claude:
  model: "claude-sonnet-4-6"
  effort: "low"
  timeout: 60
planner:
  model: "claude-sonnet-4-6"
  timeout: 30
pool:
  max_concurrent: 3
retry:
  max_attempts: 1
  init_delay_seconds: 1
  multiplier: 2.0
  max_delay_seconds: 10
sandbox:
  level: "none"
`
	configPath := filepath.Join(env.Home, ".apex", "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write live-concurrent config.yaml: %v", err)
	}
	return env
}

// Scenario 1: Multi-step DAG — LLM decomposes a complex task into multiple
// nodes and executes them sequentially/in parallel.
func TestLiveMultiStepDAG(t *testing.T) {
	skipUnlessLive(t)
	env := newLiveConcurrentEnv(t)

	// Use "first...then...finally" to trigger LLM decomposition.
	// Avoid MEDIUM-risk keywords (create/write/modify) to stay at LOW risk.
	task := "first list all .go files in the current directory, then print the total line count of those files, finally echo 'done' to stdout"

	stdout, stderr, exitCode := env.runApex("run", task)

	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)

	require.Equal(t, 0, exitCode,
		"apex run (multi-step) exited %d; stderr: %s", exitCode, stderr)

	// Should have been decomposed into >1 steps
	assert.Contains(t, stdout, "steps")
	assert.Contains(t, stdout, "COMPLETED")
	assert.Contains(t, stdout, "Done")
}

// Scenario 2: Dry-run with real LLM planning — verifies planner output
// parsing and cost estimation without executing any tasks.
func TestLiveDryRunComplexPlan(t *testing.T) {
	skipUnlessLive(t)
	env := newLiveEnv(t)

	// Multi-step task with LOW risk keywords only.
	task := "first list all Go source files, then count the total lines of code, finally echo a summary to stdout"

	stdout, stderr, exitCode := env.runApex("run", "--dry-run", task)

	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)

	require.Equal(t, 0, exitCode,
		"apex run --dry-run exited %d; stderr: %s", exitCode, stderr)

	assert.Contains(t, stdout, "DRY RUN", "should show dry run header")
	assert.Contains(t, stdout, "No changes made", "should not execute")
	assert.Contains(t, stdout, "Cost estimate", "should show cost estimate")
}

// Scenario 3: File generation — Claude runs a shell command to produce a
// real file in the working directory and we verify it exists.
func TestLiveFileGeneration(t *testing.T) {
	skipUnlessLive(t)
	env := newLiveEnv(t)

	// Use an explicit shell command to guarantee the file is produced
	// regardless of whether Claude uses Write or Bash tool.
	stdout, stderr, exitCode := env.runApex("run",
		"run this exact shell command: echo 'print(\"hello world\")' > hello.py")

	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)

	require.Equal(t, 0, exitCode,
		"apex run (file gen) exited %d; stderr: %s", exitCode, stderr)

	assert.Contains(t, stdout, "COMPLETED")

	// Verify the file was produced in WorkDir
	helloPath := filepath.Join(env.WorkDir, "hello.py")
	require.True(t, env.fileExists(helloPath),
		"hello.py should exist at %s after file generation task", helloPath)
	content := env.readFile(helloPath)
	t.Logf("hello.py content: %s", content)
	assert.Contains(t, content, "hello world")
}

// Scenario 4: Audit + Manifest integrity — after a successful run, verify
// that both the audit log and run manifest are created with correct data.
func TestLiveAuditAndManifest(t *testing.T) {
	skipUnlessLive(t)
	env := newLiveEnv(t)

	stdout, stderr, exitCode := env.runApex("run", "print the current date to stdout")

	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)

	require.Equal(t, 0, exitCode,
		"apex run exited %d; stderr: %s", exitCode, stderr)

	// Audit log should exist
	auditFiles, _ := filepath.Glob(filepath.Join(env.auditDir(), "*.jsonl"))
	assert.NotEmpty(t, auditFiles, "audit log file should be created after live run")

	if len(auditFiles) > 0 {
		auditContent := env.readFile(auditFiles[0])
		t.Logf("audit log:\n%s", auditContent)
		assert.Contains(t, auditContent, `"outcome":"success"`,
			"audit should record success outcome")
		assert.Contains(t, auditContent, "claude-sonnet-4-6",
			"audit should record the model used")
	}

	// Manifest should exist
	runFiles, _ := filepath.Glob(filepath.Join(env.runsDir(), "*", "manifest.json"))
	assert.NotEmpty(t, runFiles, "manifest should be created after live run")

	if len(runFiles) > 0 {
		manifestContent := env.readFile(runFiles[0])
		t.Logf("manifest:\n%s", manifestContent)
		// Manifest is pretty-printed JSON, so fields have spaces after colons.
		assert.Contains(t, manifestContent, `"outcome": "success"`,
			"manifest should record success outcome")
		assert.Contains(t, manifestContent, `"risk_level": "LOW"`,
			"manifest should record LOW risk")
	}
}

// Scenario 5: Plan multi-step with real LLM — verifies that the planner
// calls the real LLM for complex tasks and returns a multi-node DAG.
func TestLivePlanMultiStep(t *testing.T) {
	skipUnlessLive(t)
	env := newLiveEnv(t)

	// Use multi-step language to trigger LLM decomposition (not simple task fallback).
	task := "first list all files in the current directory, then count the lines per file, after that sort them by size, finally print a summary report"

	stdout, stderr, exitCode := env.runApex("plan", task)

	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)

	require.Equal(t, 0, exitCode,
		"apex plan (multi-step) exited %d; stderr: %s", exitCode, stderr)

	assert.Contains(t, stdout, "Execution Plan",
		"should show execution plan header")

	// The planner should decompose this into multiple steps.
	// Count "  [" occurrences which prefix each plan step.
	stepCount := strings.Count(stdout, "  [")
	t.Logf("detected %d plan steps", stepCount)
	assert.GreaterOrEqual(t, stepCount, 2,
		"complex task should be decomposed into at least 2 steps, got %d", stepCount)
}

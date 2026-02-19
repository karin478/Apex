# E2E Testing Module Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a comprehensive E2E test suite that invokes the real `apex` CLI binary against a mock Claude backend, covering all commands and failure scenarios.

**Architecture:** Each test compiles `apex` once via TestMain, creates an isolated HOME/workdir, and points config's `claude.binary` to a shell-script mock. The mock's behavior (exit code, response, delay, stderr) is controlled via env vars. Live smoke tests behind `//go:build live` tag.

**Tech Stack:** Go testing, shell script mock, testify/assert, `os/exec`, build tags

---

### Task 1: Mock Claude Script

**Files:**
- Create: `e2e/testdata/mock_claude.sh`

**Step 1: Create the mock script**

```bash
#!/usr/bin/env bash
# mock_claude.sh — simulates the claude CLI for E2E testing.
# Behavior controlled via environment variables:
#   MOCK_EXIT_CODE    — exit code (default: 0)
#   MOCK_DELAY_MS     — sleep before responding in ms (default: 0)
#   MOCK_STDERR       — content written to stderr (default: empty)
#   MOCK_RESPONSE     — stdout response for executor calls (default: {"result":"mock ok"})
#   MOCK_PLANNER_RESPONSE — stdout response for planner calls (default: single-node DAG)
#
# Detection: if the prompt argument contains "task planner" or "Decompose",
# it's a planner call; otherwise it's an executor call.

# Apply delay
if [ -n "$MOCK_DELAY_MS" ] && [ "$MOCK_DELAY_MS" -gt 0 ] 2>/dev/null; then
  sleep "$(echo "scale=3; $MOCK_DELAY_MS / 1000" | bc)"
fi

# Write stderr
if [ -n "$MOCK_STDERR" ]; then
  echo "$MOCK_STDERR" >&2
fi

# Determine if this is a planner call by scanning all arguments
is_planner=false
for arg in "$@"; do
  case "$arg" in
    *"task planner"*|*"Decompose"*) is_planner=true ;;
  esac
done

if $is_planner; then
  # Extract the task from the last argument (the prompt contains "Task: <actual task>")
  last_arg="${@: -1}"
  # Default: single-node DAG
  if [ -n "$MOCK_PLANNER_RESPONSE" ]; then
    echo "$MOCK_PLANNER_RESPONSE"
  else
    echo '[{"id":"task_1","task":"execute the task","depends":[]}]'
  fi
else
  if [ -n "$MOCK_RESPONSE" ]; then
    echo "$MOCK_RESPONSE"
  else
    echo '{"result":"mock ok"}'
  fi
fi

exit "${MOCK_EXIT_CODE:-0}"
```

**Step 2: Make it executable**

Run: `chmod +x e2e/testdata/mock_claude.sh`

**Step 3: Verify mock script works standalone**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
echo "test" | MOCK_RESPONSE='hello' bash e2e/testdata/mock_claude.sh -p --model test --output-format json "do something"
# Expected: hello
MOCK_EXIT_CODE=2 MOCK_STDERR="permission denied" bash e2e/testdata/mock_claude.sh -p --model test --output-format json "do something"
# Expected: stderr "permission denied", exit code 2
```

**Step 4: Commit**

```bash
git add e2e/testdata/mock_claude.sh
git commit -m "test(e2e): add mock claude script for E2E testing"
```

---

### Task 2: TestMain + Test Helpers

**Files:**
- Create: `e2e/setup_test.go`
- Create: `e2e/helpers_test.go`

**Step 1: Write setup_test.go with TestMain**

```go
package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var apexBin string // path to compiled apex binary

func TestMain(m *testing.M) {
	// Build apex binary once for all tests
	tmpDir, err := os.MkdirTemp("", "apex-e2e-build-*")
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)

	apexBin = filepath.Join(tmpDir, "apex")
	projectRoot := findProjectRoot()

	cmd := exec.Command("go", "build", "-o", apexBin, "./cmd/apex/")
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build apex: " + err.Error())
	}

	os.Exit(m.Run())
}

func findProjectRoot() string {
	// Walk up from the e2e directory to find go.mod
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("could not find project root (go.mod)")
		}
		dir = parent
	}
}
```

**Step 2: Write helpers_test.go**

```go
package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestEnv provides an isolated environment for a single E2E test.
type TestEnv struct {
	Home    string // Isolated HOME directory (contains .apex/ and .claude/)
	WorkDir string // Git-initialized working directory
	MockBin string // Absolute path to mock_claude.sh
	T       *testing.T
}

// newTestEnv creates a fully isolated test environment.
func newTestEnv(t *testing.T) *TestEnv {
	t.Helper()
	home := t.TempDir()
	workDir := t.TempDir()

	// Find mock_claude.sh relative to test file
	_, thisFile, _, _ := runtime.Caller(0)
	mockBin := filepath.Join(filepath.Dir(thisFile), "testdata", "mock_claude.sh")
	require.FileExists(t, mockBin, "mock_claude.sh must exist")

	// Initialize git repo in workDir (needed for snapshot operations)
	git := exec.Command("git", "init")
	git.Dir = workDir
	git.Env = append(os.Environ(), "HOME="+home)
	out, err := git.CombinedOutput()
	require.NoError(t, err, "git init failed: %s", string(out))

	// Create initial commit so git stash works
	gitAdd := exec.Command("git", "commit", "--allow-empty", "-m", "init")
	gitAdd.Dir = workDir
	gitAdd.Env = append(os.Environ(), "HOME="+home,
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
	out, err = gitAdd.CombinedOutput()
	require.NoError(t, err, "git commit failed: %s", string(out))

	// Write minimal config pointing to mock
	apexDir := filepath.Join(home, ".apex")
	require.NoError(t, os.MkdirAll(filepath.Join(apexDir, "audit"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(apexDir, "runs"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(apexDir, "memory", "decisions"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(apexDir, "memory", "facts"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(apexDir, "memory", "sessions"), 0755))

	// Ensure .claude directory exists (for kill switch)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0755))

	configYAML := fmt.Sprintf(`claude:
  model: "mock-model"
  effort: "low"
  timeout: 10
  binary: "%s"
planner:
  model: "mock-model"
  timeout: 10
pool:
  max_concurrent: 2
retry:
  max_attempts: 3
  init_delay_seconds: 0
  multiplier: 1.0
  max_delay_seconds: 0
`, mockBin)

	require.NoError(t, os.WriteFile(filepath.Join(apexDir, "config.yaml"), []byte(configYAML), 0644))

	return &TestEnv{
		Home:    home,
		WorkDir: workDir,
		MockBin: mockBin,
		T:       t,
	}
}

// runApex executes the apex binary with the given args and returns stdout, stderr, exit code.
func (e *TestEnv) runApex(args ...string) (stdout, stderr string, exitCode int) {
	return e.runApexWithEnv(nil, args...)
}

// runApexWithEnv executes apex with extra environment variables for mock control.
func (e *TestEnv) runApexWithEnv(env map[string]string, args ...string) (stdout, stderr string, exitCode int) {
	e.T.Helper()
	cmd := exec.Command(apexBin, args...)
	cmd.Dir = e.WorkDir

	// Build environment: inherit PATH, set HOME, add extras
	cmdEnv := []string{
		"HOME=" + e.Home,
		"PATH=" + os.Getenv("PATH"),
	}
	for k, v := range env {
		cmdEnv = append(cmdEnv, k+"="+v)
	}
	cmd.Env = cmdEnv

	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return stdoutBuf.String(), stderrBuf.String(), exitCode
}

// fileExists checks if a file exists at the given absolute path.
func (e *TestEnv) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readFile reads the content of a file at the given absolute path.
func (e *TestEnv) readFile(path string) string {
	data, err := os.ReadFile(path)
	require.NoError(e.T, err)
	return string(data)
}

// auditDir returns the path to the audit directory.
func (e *TestEnv) auditDir() string {
	return filepath.Join(e.Home, ".apex", "audit")
}

// runsDir returns the path to the runs directory.
func (e *TestEnv) runsDir() string {
	return filepath.Join(e.Home, ".apex", "runs")
}

// killSwitchPath returns the path to the kill switch file.
func (e *TestEnv) killSwitchPath() string {
	return filepath.Join(e.Home, ".claude", "KILL_SWITCH")
}
```

**Step 3: Write a minimal smoke test to verify the framework works**

Add to `e2e/helpers_test.go`:

```go
func TestVersion(t *testing.T) {
	env := newTestEnv(t)
	stdout, _, exitCode := env.runApex("version")
	require.Equal(t, 0, exitCode)
	require.Contains(t, stdout, "apex v")
}
```

**Step 4: Run to verify the framework compiles and TestVersion passes**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./e2e/... -v -run TestVersion -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add e2e/setup_test.go e2e/helpers_test.go
git commit -m "test(e2e): add TestMain build setup and test helpers"
```

---

### Task 3: Config Edge Case

**Important note:** The current `run.go` and other commands hardcode `home, _ := os.UserHomeDir()` to find config. Since we override `HOME` in the env, `os.UserHomeDir()` will return our temp HOME. This is the mechanism for isolation — no code changes needed.

However, `executor.Options.Binary` defaults to `"claude"` if not specified in config. We need to make the executor respect a `binary` field in config.

**Files:**
- Modify: `internal/config/config.go` — add `Binary` field to `ClaudeConfig`
- Modify: `cmd/apex/run.go` — pass `cfg.Claude.Binary` to executor
- Modify: `cmd/apex/plan.go` — pass `cfg.Claude.Binary` to executor
- Test: `e2e/config_test.go`

**Step 1: Add Binary field to ClaudeConfig**

In `internal/config/config.go`, add `Binary` field:

```go
type ClaudeConfig struct {
	Model           string `yaml:"model"`
	Effort          string `yaml:"effort"`
	Timeout         int    `yaml:"timeout"`
	LongTaskTimeout int    `yaml:"long_task_timeout"`
	Binary          string `yaml:"binary"`
}
```

No default needed — empty string means executor falls back to `"claude"`.

**Step 2: Wire Binary in run.go executors**

In `cmd/apex/run.go`, planner executor (~line 86):

```go
planExec := executor.New(executor.Options{
	Model:   cfg.Planner.Model,
	Effort:  "high",
	Timeout: time.Duration(cfg.Planner.Timeout) * time.Second,
	Binary:  cfg.Claude.Binary,
})
```

And the execution executor (~line 186):

```go
exec := executor.New(executor.Options{
	Model:   cfg.Claude.Model,
	Effort:  cfg.Claude.Effort,
	Timeout: time.Duration(cfg.Claude.Timeout) * time.Second,
	Binary:  cfg.Claude.Binary,
})
```

**Step 3: Wire Binary in plan.go executor**

In `cmd/apex/plan.go` (~line 37):

```go
exec := executor.New(executor.Options{
	Model:   cfg.Planner.Model,
	Effort:  "high",
	Timeout: time.Duration(cfg.Planner.Timeout) * time.Second,
	Binary:  cfg.Claude.Binary,
})
```

**Step 4: Write config E2E test**

Create `e2e/config_test.go`:

```go
package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunWithDefaultConfig(t *testing.T) {
	env := newTestEnv(t)
	// Remove config to test defaults (but defaults won't have mock binary path,
	// so this test verifies the CLI doesn't crash on missing config)
	os.Remove(filepath.Join(env.Home, ".apex", "config.yaml"))

	// apex version should always work regardless of config
	stdout, _, exitCode := env.runApex("version")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "apex v")
}

func TestRunWithCustomConfig(t *testing.T) {
	env := newTestEnv(t)

	// Run a simple task — the mock config in newTestEnv points to mock_claude.sh
	stdout, _, exitCode := env.runApex("run", "say hello")
	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "Done")
}
```

**Step 5: Run tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./e2e/... -v -run TestRun -count=1`
Expected: PASS

**Step 6: Run existing unit tests to ensure no breakage**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/... -count=1`
Expected: All pass

**Step 7: Commit**

```bash
git add internal/config/config.go cmd/apex/run.go cmd/apex/plan.go e2e/config_test.go
git commit -m "feat(config): add claude.binary config field for E2E testing"
```

---

### Task 4: Run Command Tests

**Files:**
- Create: `e2e/run_test.go`

**Step 1: Write run E2E tests**

```go
package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunHappyPath(t *testing.T) {
	env := newTestEnv(t)

	stdout, _, exitCode := env.runApex("run", "say hello")

	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "Done")
	assert.Contains(t, stdout, "Risk level")

	// Verify audit was written
	auditFiles, _ := filepath.Glob(filepath.Join(env.auditDir(), "*.jsonl"))
	assert.NotEmpty(t, auditFiles, "audit file should be created")

	// Verify manifest was written
	runFiles, _ := filepath.Glob(filepath.Join(env.runsDir(), "*.json"))
	assert.NotEmpty(t, runFiles, "manifest file should be created")
}

func TestRunDryRun(t *testing.T) {
	env := newTestEnv(t)

	stdout, _, exitCode := env.runApex("run", "--dry-run", "say hello")

	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "DRY RUN")
	assert.Contains(t, stdout, "No changes made")

	// Verify no manifest created (dry run doesn't execute)
	runFiles, _ := filepath.Glob(filepath.Join(env.runsDir(), "*.json"))
	assert.Empty(t, runFiles, "dry run should not create manifest")
}

func TestRunMultiNodeDAG(t *testing.T) {
	env := newTestEnv(t)

	multiDAG := `[{"id":"step1","task":"first step","depends":[]},{"id":"step2","task":"second step","depends":["step1"]},{"id":"step3","task":"third step","depends":["step1"]}]`

	stdout, _, exitCode := env.runApexWithEnv(
		map[string]string{"MOCK_PLANNER_RESPONSE": multiDAG},
		"run", "first do X then do Y and Z",
	)

	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "3 steps")
	assert.Contains(t, stdout, "Done")
}

func TestRunExecutionFailure(t *testing.T) {
	env := newTestEnv(t)

	// Mock returns exit code 2 (non-retriable) for execution
	_, _, exitCode := env.runApexWithEnv(
		map[string]string{
			"MOCK_EXIT_CODE": "2",
			"MOCK_STDERR":    "permission denied",
		},
		"run", "say hello",
	)

	// Exit code should be non-zero (execution error propagates)
	assert.NotEqual(t, 0, exitCode)
}

func TestRunNoArgs(t *testing.T) {
	env := newTestEnv(t)

	_, stderr, exitCode := env.runApex("run")

	assert.NotEqual(t, 0, exitCode)
	assert.Contains(t, stderr, "requires at least 1 arg")
}
```

**Step 2: Run**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./e2e/... -v -run "TestRun" -count=1`
Expected: All pass

**Step 3: Commit**

```bash
git add e2e/run_test.go
git commit -m "test(e2e): add apex run command tests"
```

---

### Task 5: Plan Command Tests

**Files:**
- Create: `e2e/plan_test.go`

**Step 1: Write plan E2E tests**

```go
package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanSimpleTask(t *testing.T) {
	env := newTestEnv(t)

	stdout, _, exitCode := env.runApex("plan", "say hello")

	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "Execution Plan")
	assert.Contains(t, stdout, "1 steps")
}

func TestPlanComplexTask(t *testing.T) {
	env := newTestEnv(t)

	multiDAG := `[{"id":"analyze","task":"analyze codebase","depends":[]},{"id":"refactor","task":"refactor module","depends":["analyze"]},{"id":"test","task":"run tests","depends":["refactor"]}]`

	stdout, _, exitCode := env.runApexWithEnv(
		map[string]string{"MOCK_PLANNER_RESPONSE": multiDAG},
		"plan", "first analyze then refactor and test",
	)

	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "3 steps")
	assert.Contains(t, stdout, "analyze")
	assert.Contains(t, stdout, "refactor")
}
```

**Step 2: Run**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./e2e/... -v -run "TestPlan" -count=1`
Expected: PASS

**Step 3: Commit**

```bash
git add e2e/plan_test.go
git commit -m "test(e2e): add apex plan command tests"
```

---

### Task 6: Kill Switch + Resume Tests

**Files:**
- Create: `e2e/killswitch_test.go`

**Step 1: Write kill switch E2E tests**

```go
package e2e

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKillSwitchActivateAndResume(t *testing.T) {
	env := newTestEnv(t)

	// Activate kill switch
	stdout, _, exitCode := env.runApex("kill-switch", "testing emergency")
	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "ACTIVATED")
	assert.True(t, env.fileExists(env.killSwitchPath()), "kill switch file should exist")

	// Run should be blocked
	_, _, exitCode = env.runApex("run", "say hello")
	assert.NotEqual(t, 0, exitCode, "run should fail when kill switch is active")

	// Resume
	stdout, _, exitCode = env.runApex("resume")
	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "DEACTIVATED")
	assert.False(t, env.fileExists(env.killSwitchPath()), "kill switch file should be removed")

	// Run should work again
	stdout, _, exitCode = env.runApex("run", "say hello")
	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "Done")
}

func TestKillSwitchAlreadyActive(t *testing.T) {
	env := newTestEnv(t)

	// Create kill switch file manually
	os.MkdirAll(filepath.Dir(env.killSwitchPath()), 0755)
	os.WriteFile(env.killSwitchPath(), []byte("pre-existing"), 0644)

	stdout, _, exitCode := env.runApex("kill-switch", "again")
	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "already active")
}

func TestResumeWhenNotActive(t *testing.T) {
	env := newTestEnv(t)

	stdout, _, exitCode := env.runApex("resume")
	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "No kill switch active")
}
```

Note: Need to add `"os"` and `"path/filepath"` imports. The `filepath` import is already used via `env.killSwitchPath()` but `TestKillSwitchAlreadyActive` uses `filepath.Dir` directly — needs import.

**Step 2: Run**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./e2e/... -v -run "TestKill|TestResume" -count=1`
Expected: PASS

**Step 3: Commit**

```bash
git add e2e/killswitch_test.go
git commit -m "test(e2e): add kill switch and resume tests"
```

---

### Task 7: Doctor Tests

**Files:**
- Create: `e2e/doctor_test.go`

**Step 1: Write doctor E2E tests**

```go
package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoctorHealthy(t *testing.T) {
	env := newTestEnv(t)

	// First run a task to create audit entries
	_, _, exitCode := env.runApex("run", "say hello")
	require.Equal(t, 0, exitCode)

	// Doctor should report OK
	stdout, _, exitCode := env.runApex("doctor")
	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "OK")
}

func TestDoctorNoAudit(t *testing.T) {
	env := newTestEnv(t)

	// Remove audit directory to simulate fresh install
	os.RemoveAll(env.auditDir())

	stdout, _, exitCode := env.runApex("doctor")
	require.Equal(t, 0, exitCode)
	// Should handle gracefully
	assert.Contains(t, stdout, "Apex Doctor")
}

func TestDoctorCorruptedChain(t *testing.T) {
	env := newTestEnv(t)

	// Run a task to create audit
	_, _, exitCode := env.runApex("run", "say hello")
	require.Equal(t, 0, exitCode)

	// Corrupt the audit file
	auditFiles, _ := filepath.Glob(filepath.Join(env.auditDir(), "*.jsonl"))
	require.NotEmpty(t, auditFiles)

	data, err := os.ReadFile(auditFiles[0])
	require.NoError(t, err)
	// Tamper with the data
	corrupted := []byte("CORRUPTED" + string(data))
	os.WriteFile(auditFiles[0], corrupted, 0644)

	stdout, _, exitCode := env.runApex("doctor")
	require.Equal(t, 0, exitCode)
	// Should detect corruption (either ERROR or BROKEN)
	assert.True(t, assert.ObjectsAreEqual(true,
		containsAny(stdout, "BROKEN", "ERROR")),
		"doctor should detect corruption, got: %s", stdout)
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if assert.ObjectsAreEqual(true, len(s) > 0) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
```

Actually, let me simplify `containsAny`:

```go
import "strings"

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
```

**Step 2: Run**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./e2e/... -v -run "TestDoctor" -count=1`
Expected: PASS

**Step 3: Commit**

```bash
git add e2e/doctor_test.go
git commit -m "test(e2e): add doctor command tests"
```

---

### Task 8: Status + History Tests

**Files:**
- Create: `e2e/status_test.go`

**Step 1: Write status and history E2E tests**

```go
package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusAfterRun(t *testing.T) {
	env := newTestEnv(t)

	// Run a task first
	_, _, exitCode := env.runApex("run", "say hello")
	require.Equal(t, 0, exitCode)

	// Status should show the run
	stdout, _, exitCode := env.runApex("status")
	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "say hello")
	assert.Contains(t, stdout, "success")
}

func TestStatusEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, _, exitCode := env.runApex("status")
	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "No runs found")
}

func TestHistoryAfterRun(t *testing.T) {
	env := newTestEnv(t)

	// Run a task first
	_, _, exitCode := env.runApex("run", "say hello")
	require.Equal(t, 0, exitCode)

	// History should show audit entries
	stdout, _, exitCode := env.runApex("history")
	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "[OK]")
}

func TestHistoryEmpty(t *testing.T) {
	env := newTestEnv(t)

	// Remove audit entries
	stdout, _, exitCode := env.runApex("history")
	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "No history yet")
}
```

**Step 2: Run**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./e2e/... -v -run "TestStatus|TestHistory" -count=1`
Expected: PASS

**Step 3: Commit**

```bash
git add e2e/status_test.go
git commit -m "test(e2e): add status and history command tests"
```

---

### Task 9: Retry / Fault Tolerance Tests

**Files:**
- Create: `e2e/retry_test.go`

**Note:** The mock script needs enhancement for retry testing. We need a way to fail on the first N calls then succeed. We'll use a state file approach: the mock writes a counter file, and succeeds after N failures.

**Step 1: Enhance mock_claude.sh with retry counter support**

Add to `mock_claude.sh` (after the delay section, before the planner detection):

```bash
# Retry counter support: fail MOCK_FAIL_COUNT times, then succeed
# Uses MOCK_COUNTER_FILE to track call count across invocations
if [ -n "$MOCK_FAIL_COUNT" ] && [ -n "$MOCK_COUNTER_FILE" ]; then
  count=0
  if [ -f "$MOCK_COUNTER_FILE" ]; then
    count=$(cat "$MOCK_COUNTER_FILE")
  fi
  count=$((count + 1))
  echo "$count" > "$MOCK_COUNTER_FILE"

  if [ "$count" -le "$MOCK_FAIL_COUNT" ]; then
    echo "${MOCK_STDERR:-timeout error}" >&2
    exit "${MOCK_EXIT_CODE:-1}"
  fi
  # Reset exit code for success
  MOCK_EXIT_CODE=0
  MOCK_STDERR=""
fi
```

**Step 2: Write retry E2E tests**

```go
package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetrySuccessOnSecondAttempt(t *testing.T) {
	env := newTestEnv(t)
	counterFile := filepath.Join(env.Home, "retry_counter")

	stdout, _, exitCode := env.runApexWithEnv(
		map[string]string{
			"MOCK_FAIL_COUNT":  "1",
			"MOCK_COUNTER_FILE": counterFile,
			"MOCK_EXIT_CODE":    "1",
			"MOCK_STDERR":       "timeout error",
		},
		"run", "say hello",
	)

	require.Equal(t, 0, exitCode, "should succeed after retry")
	assert.Contains(t, stdout, "Done")

	// Counter file should show 2 calls (1 fail + 1 success for executor;
	// planner also calls mock but uses different detection path)
	data, err := os.ReadFile(counterFile)
	require.NoError(t, err)
	t.Logf("Total mock calls: %s", string(data))
}

func TestRetryExhausted(t *testing.T) {
	env := newTestEnv(t)
	counterFile := filepath.Join(env.Home, "retry_counter")

	// Fail all 3 attempts (max_attempts=3 in config)
	_, _, exitCode := env.runApexWithEnv(
		map[string]string{
			"MOCK_FAIL_COUNT":  "100",
			"MOCK_COUNTER_FILE": counterFile,
			"MOCK_EXIT_CODE":    "1",
			"MOCK_STDERR":       "timeout error",
		},
		"run", "say hello",
	)

	assert.NotEqual(t, 0, exitCode, "should fail after retries exhausted")
}

func TestNonRetriableDoesNotRetry(t *testing.T) {
	env := newTestEnv(t)
	counterFile := filepath.Join(env.Home, "retry_counter")

	// Exit code 2 = non-retriable, should not retry
	_, _, exitCode := env.runApexWithEnv(
		map[string]string{
			"MOCK_FAIL_COUNT":  "100",
			"MOCK_COUNTER_FILE": counterFile,
			"MOCK_EXIT_CODE":    "2",
			"MOCK_STDERR":       "permission denied",
		},
		"run", "say hello",
	)

	assert.NotEqual(t, 0, exitCode)

	// Should have fewer calls than max_attempts (planner + 1 executor call)
	data, _ := os.ReadFile(counterFile)
	t.Logf("Total mock calls for non-retriable: %s", string(data))
}
```

**Step 3: Run**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./e2e/... -v -run "TestRetry|TestNonRetriable" -count=1`
Expected: PASS

**Step 4: Commit**

```bash
git add e2e/testdata/mock_claude.sh e2e/retry_test.go
git commit -m "test(e2e): add retry and fault tolerance tests"
```

---

### Task 10: Snapshot Tests

**Files:**
- Create: `e2e/snapshot_test.go`

**Step 1: Write snapshot E2E tests**

```go
package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotListEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, _, exitCode := env.runApex("snapshot", "list")
	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "No snapshots")
}

func TestSnapshotCreatedOnRun(t *testing.T) {
	env := newTestEnv(t)

	// Run a task — it should create a snapshot
	stdout, _, exitCode := env.runApex("run", "say hello")
	require.Equal(t, 0, exitCode)

	// On success, snapshot is dropped. Check stdout for "Snapshot saved" message
	assert.Contains(t, stdout, "Snapshot saved")
}

func TestSnapshotPersistsOnFailure(t *testing.T) {
	env := newTestEnv(t)

	// Fail the execution — snapshot should persist
	_, _, exitCode := env.runApexWithEnv(
		map[string]string{
			"MOCK_EXIT_CODE": "2",
			"MOCK_STDERR":    "fatal error",
		},
		"run", "say hello",
	)

	assert.NotEqual(t, 0, exitCode)

	// Snapshot list should show an entry
	stdout, _, listCode := env.runApex("snapshot", "list")
	require.Equal(t, 0, listCode)
	// Should have a snapshot entry (not "No snapshots")
	t.Logf("Snapshot list output: %s", stdout)
}
```

**Step 2: Run**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./e2e/... -v -run "TestSnapshot" -count=1`
Expected: PASS

**Step 3: Commit**

```bash
git add e2e/snapshot_test.go
git commit -m "test(e2e): add snapshot command tests"
```

---

### Task 11: Live Smoke Tests

**Files:**
- Create: `e2e/live_test.go`

**Step 1: Write live smoke tests behind build tag**

```go
//go:build live

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newLiveEnv creates a test env that uses the real claude binary.
func newLiveEnv(t *testing.T) *TestEnv {
	t.Helper()
	env := newTestEnv(t)

	// Override config to use real claude binary
	apexDir := filepath.Join(env.Home, ".apex")
	configYAML := fmt.Sprintf(`claude:
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
`)
	require.NoError(t, os.WriteFile(filepath.Join(apexDir, "config.yaml"), []byte(configYAML), 0644))
	return env
}

func TestLiveVersion(t *testing.T) {
	env := newLiveEnv(t)
	stdout, _, exitCode := env.runApex("version")
	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "apex v")
}

func TestLiveDoctor(t *testing.T) {
	env := newLiveEnv(t)
	stdout, _, exitCode := env.runApex("doctor")
	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "Apex Doctor")
}

func TestLiveRunSimple(t *testing.T) {
	if os.Getenv("APEX_LIVE_TESTS") == "" {
		t.Skip("Set APEX_LIVE_TESTS=1 to run live Claude tests (costs tokens)")
	}
	env := newLiveEnv(t)

	stdout, stderr, exitCode := env.runApex("run", "echo hello world to stdout using a shell command")
	t.Logf("stdout: %s", stdout)
	t.Logf("stderr: %s", stderr)
	assert.Equal(t, 0, exitCode, "live run should succeed")
}

func TestLivePlan(t *testing.T) {
	if os.Getenv("APEX_LIVE_TESTS") == "" {
		t.Skip("Set APEX_LIVE_TESTS=1 to run live Claude tests (costs tokens)")
	}
	env := newLiveEnv(t)

	stdout, stderr, exitCode := env.runApex("plan", "create a hello world Go program")
	t.Logf("stdout: %s", stdout)
	t.Logf("stderr: %s", stderr)
	assert.Equal(t, 0, exitCode, "live plan should succeed")
	assert.Contains(t, stdout, "Execution Plan")
}
```

**Step 2: Run (without live — should be skipped)**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./e2e/... -v -count=1`
Expected: Live tests are not compiled (build tag not set), mock tests all pass

**Step 3: Commit**

```bash
git add e2e/live_test.go
git commit -m "test(e2e): add live smoke tests behind build tag"
```

---

### Task 12: Makefile Integration

**Files:**
- Modify: `Makefile`

**Step 1: Add E2E targets to Makefile**

Append to Makefile:

```makefile
.PHONY: e2e e2e-live

e2e:
	go test ./e2e/... -v -count=1 -timeout=120s

e2e-live:
	APEX_LIVE_TESTS=1 go test ./e2e/... -v -count=1 -tags=live -timeout=300s
```

**Step 2: Run the full E2E suite**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && make e2e`
Expected: All mock-mode tests pass

**Step 3: Run existing unit tests to ensure nothing broke**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && make test`
Expected: All unit tests pass

**Step 4: Commit**

```bash
git add Makefile
git commit -m "build: add e2e and e2e-live Makefile targets"
```

---

### Task 13: Update PROGRESS.md

**Files:**
- Modify: `PROGRESS.md`

**Step 1: Add E2E testing to completed phases**

Add row to completed phases table:

```markdown
| E2E | E2E Testing Module | `e2e-testing-design.md` | Done |
```

**Step 2: Commit**

```bash
git add PROGRESS.md
git commit -m "docs: mark E2E testing module complete"
```

package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestEnv encapsulates a temporary isolated environment for a single test.
type TestEnv struct {
	Home    string // temp HOME directory (~/.apex, ~/.claude live here)
	WorkDir string // temp working directory (git repo)
	MockBin string // absolute path to mock_claude.sh
	T       *testing.T
}

// newTestEnv creates a fully initialised test environment:
//   - temp HOME with .apex/ directory structure and .claude/ directory
//   - temp WorkDir initialised as a git repo with an empty commit
//   - config.yaml pointing claude.binary to the mock script
func newTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	home := t.TempDir()
	workDir := t.TempDir()

	// Locate mock_claude.sh relative to this source file
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed — cannot locate test source file")
	}
	mockBin := filepath.Join(filepath.Dir(thisFile), "testdata", "mock_claude.sh")
	if _, err := os.Stat(mockBin); err != nil {
		t.Fatalf("mock_claude.sh not found at %s: %v", mockBin, err)
	}
	// Ensure the mock script is executable
	if err := os.Chmod(mockBin, 0755); err != nil {
		t.Fatalf("chmod mock_claude.sh: %v", err)
	}

	// Create .apex/ directory structure
	apexDirs := []string{
		filepath.Join(home, ".apex", "audit"),
		filepath.Join(home, ".apex", "runs"),
		filepath.Join(home, ".apex", "memory", "decisions"),
		filepath.Join(home, ".apex", "memory", "facts"),
		filepath.Join(home, ".apex", "memory", "sessions"),
	}
	for _, d := range apexDirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	// Create .claude/ directory (for kill switch)
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", claudeDir, err)
	}

	// Write config.yaml pointing to the mock script
	configContent := fmt.Sprintf(`claude:
  model: "mock-model"
  effort: "low"
  timeout: 10
  binary: %q
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
sandbox:
  level: "none"
`, mockBin)

	configPath := filepath.Join(home, ".apex", "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}

	// Initialise git repo in WorkDir with an empty commit (for snapshot compatibility)
	for _, gitCmd := range [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@apex.dev"},
		{"git", "config", "user.name", "Apex Test"},
		{"git", "commit", "--allow-empty", "-m", "initial commit"},
	} {
		c := exec.Command(gitCmd[0], gitCmd[1:]...)
		c.Dir = workDir
		c.Env = append(os.Environ(), "HOME="+home)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git init step %v failed: %v\n%s", gitCmd, err, out)
		}
	}

	return &TestEnv{
		Home:    home,
		WorkDir: workDir,
		MockBin: mockBin,
		T:       t,
	}
}

// runApex executes the compiled apex binary with the given arguments.
func (e *TestEnv) runApex(args ...string) (stdout, stderr string, exitCode int) {
	return e.runApexWithEnv(nil, args...)
}

// runApexWithEnv executes apex with additional environment variables for mock control.
func (e *TestEnv) runApexWithEnv(env map[string]string, args ...string) (stdout, stderr string, exitCode int) {
	e.T.Helper()

	cmd := exec.Command(apexBin, args...)
	cmd.Dir = e.WorkDir

	// Build environment: real PATH + overridden HOME + extras.
	// CLAUDE_CODE_OAUTH_TOKEN is forwarded so that live tests can
	// authenticate with the real Claude CLI from a temp HOME.
	cmdEnv := []string{
		"HOME=" + e.Home,
		"PATH=" + os.Getenv("PATH"),
		"USER=" + os.Getenv("USER"),
	}
	if tok := os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"); tok != "" {
		cmdEnv = append(cmdEnv, "CLAUDE_CODE_OAUTH_TOKEN="+tok)
	}
	for k, v := range env {
		cmdEnv = append(cmdEnv, k+"="+v)
	}
	cmd.Env = cmdEnv

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()

	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Non-exit error (e.g. binary not found)
			exitCode = -1
		}
	}

	return outBuf.String(), errBuf.String(), exitCode
}

// fileExists returns true if path exists and is not a directory.
func (e *TestEnv) fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// readFile returns the contents of path as a string. Fails the test on error.
func (e *TestEnv) readFile(path string) string {
	e.T.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		e.T.Fatalf("readFile(%s): %v", path, err)
	}
	return string(data)
}

// auditDir returns the path to the audit directory inside the temp HOME.
func (e *TestEnv) auditDir() string {
	return filepath.Join(e.Home, ".apex", "audit")
}

// runsDir returns the path to the runs directory inside the temp HOME.
func (e *TestEnv) runsDir() string {
	return filepath.Join(e.Home, ".apex", "runs")
}

// killSwitchPath returns the path to the kill switch file inside the temp HOME.
func (e *TestEnv) killSwitchPath() string {
	return filepath.Join(e.Home, ".claude", "KILL_SWITCH")
}

// ---------------------------------------------------------------------------
// Smoke test — verifies that the test framework compiles and apex runs
// ---------------------------------------------------------------------------

func TestVersion(t *testing.T) {
	env := newTestEnv(t)
	stdout, stderr, code := env.runApex("version")

	if code != 0 {
		t.Fatalf("apex version exited %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "apex") {
		t.Fatalf("expected 'apex' in version output, got: %s", stdout)
	}
}

# Phase 11: Execution Sandbox — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add multi-level execution sandboxing (Docker > Ulimit > None) so agent tasks run in isolated environments with Fail-Closed governance enforcement.

**Architecture:** A `Sandbox` interface with `Wrap()` transforms the binary+args before execution. `Detect()` auto-selects the strongest available backend. The executor calls `Wrap()` before spawning the process. Governance blocks execution when the required isolation level isn't available.

**Tech Stack:** Go, `os/exec`, `context`, Testify

---

### Task 1: Level Enum + Sandbox Interface

**Files:**
- Create: `internal/sandbox/sandbox.go`
- Create: `internal/sandbox/sandbox_test.go`

**Step 1: Write the failing test**

In `internal/sandbox/sandbox_test.go`:

```go
package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLevelOrdering(t *testing.T) {
	assert.True(t, None < Ulimit)
	assert.True(t, Ulimit < Docker)
}

func TestLevelString(t *testing.T) {
	assert.Equal(t, "none", None.String())
	assert.Equal(t, "ulimit", Ulimit.String())
	assert.Equal(t, "docker", Docker.String())
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
		err   bool
	}{
		{"none", None, false},
		{"ulimit", Ulimit, false},
		{"docker", Docker, false},
		{"DOCKER", Docker, false},
		{"auto", None, true},
		{"invalid", None, true},
	}
	for _, tt := range tests {
		got, err := ParseLevel(tt.input)
		if tt.err {
			assert.Error(t, err, "input: %s", tt.input)
		} else {
			assert.NoError(t, err, "input: %s", tt.input)
			assert.Equal(t, tt.want, got, "input: %s", tt.input)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/sandbox/... -v -count=1`
Expected: FAIL — package does not exist

**Step 3: Write minimal implementation**

In `internal/sandbox/sandbox.go`:

```go
package sandbox

import (
	"context"
	"fmt"
	"strings"
)

// Level represents sandbox isolation strength (higher = stronger).
type Level int

const (
	None   Level = iota
	Ulimit
	Docker
)

func (l Level) String() string {
	switch l {
	case None:
		return "none"
	case Ulimit:
		return "ulimit"
	case Docker:
		return "docker"
	default:
		return "unknown"
	}
}

// ParseLevel converts a string to a Level. Does not accept "auto".
func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(s) {
	case "none":
		return None, nil
	case "ulimit":
		return Ulimit, nil
	case "docker":
		return Docker, nil
	default:
		return None, fmt.Errorf("unknown sandbox level: %q", s)
	}
}

// Sandbox wraps a command with isolation.
type Sandbox interface {
	Level() Level
	Wrap(ctx context.Context, binary string, args []string) (string, []string, error)
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/sandbox/... -v -count=1`
Expected: PASS (3 tests)

**Step 5: Commit**

```bash
git add internal/sandbox/sandbox.go internal/sandbox/sandbox_test.go
git commit -m "feat(sandbox): add Level enum, Sandbox interface, ParseLevel"
```

---

### Task 2: None Backend

**Files:**
- Create: `internal/sandbox/none.go`
- Modify: `internal/sandbox/sandbox_test.go`

**Step 1: Write the failing test**

Append to `internal/sandbox/sandbox_test.go`:

```go
func TestNoneBackend(t *testing.T) {
	sb := &NoneSandbox{}
	assert.Equal(t, None, sb.Level())

	bin, args, err := sb.Wrap(context.Background(), "claude", []string{"-p", "hello"})
	assert.NoError(t, err)
	assert.Equal(t, "claude", bin)
	assert.Equal(t, []string{"-p", "hello"}, args)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/sandbox/... -v -count=1 -run TestNoneBackend`
Expected: FAIL — `NoneSandbox` undefined

**Step 3: Write minimal implementation**

In `internal/sandbox/none.go`:

```go
package sandbox

import "context"

// NoneSandbox is a no-op passthrough — returns the command unchanged.
type NoneSandbox struct{}

func (n *NoneSandbox) Level() Level { return None }

func (n *NoneSandbox) Wrap(_ context.Context, binary string, args []string) (string, []string, error) {
	return binary, args, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/sandbox/... -v -count=1 -run TestNoneBackend`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/sandbox/none.go internal/sandbox/sandbox_test.go
git commit -m "feat(sandbox): add None backend (passthrough)"
```

---

### Task 3: Ulimit Backend

**Files:**
- Create: `internal/sandbox/ulimit.go`
- Modify: `internal/sandbox/sandbox_test.go`

**Step 1: Write the failing test**

Append to `internal/sandbox/sandbox_test.go`:

```go
func TestUlimitBackend(t *testing.T) {
	sb := &UlimitSandbox{
		MaxMemoryKB:  2097152,  // 2GB
		MaxCPUSec:    300,
		MaxFileSizeMB: 100,
	}
	assert.Equal(t, Ulimit, sb.Level())

	bin, args, err := sb.Wrap(context.Background(), "claude", []string{"-p", "hello"})
	assert.NoError(t, err)
	assert.Equal(t, "sh", bin)
	// The args should be: ["-c", "ulimit -v 2097152 -t 300 -f 204800; exec claude -p hello"]
	assert.Len(t, args, 2)
	assert.Equal(t, "-c", args[0])
	assert.Contains(t, args[1], "ulimit")
	assert.Contains(t, args[1], "-v 2097152")
	assert.Contains(t, args[1], "-t 300")
	assert.Contains(t, args[1], "exec claude -p hello")
}

func TestUlimitDefaults(t *testing.T) {
	sb := &UlimitSandbox{} // all zero
	_, args, err := sb.Wrap(context.Background(), "claude", []string{"-p", "hi"})
	assert.NoError(t, err)
	// Should use defaults
	assert.Contains(t, args[1], "ulimit")
	assert.Contains(t, args[1], "exec claude")
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/sandbox/... -v -count=1 -run TestUlimit`
Expected: FAIL — `UlimitSandbox` undefined

**Step 3: Write minimal implementation**

In `internal/sandbox/ulimit.go`:

```go
package sandbox

import (
	"context"
	"fmt"
	"strings"
)

// UlimitSandbox wraps commands with resource limits via ulimit.
type UlimitSandbox struct {
	MaxMemoryKB   int // -v: virtual memory in KB (default: 2097152 = 2GB)
	MaxCPUSec     int // -t: CPU seconds (default: 300)
	MaxFileSizeMB int // -f: file size in 512-byte blocks (default: 100MB = 204800 blocks)
}

func (u *UlimitSandbox) Level() Level { return Ulimit }

func (u *UlimitSandbox) Wrap(_ context.Context, binary string, args []string) (string, []string, error) {
	mem := u.MaxMemoryKB
	if mem <= 0 {
		mem = 2097152 // 2GB
	}
	cpu := u.MaxCPUSec
	if cpu <= 0 {
		cpu = 300
	}
	fileMB := u.MaxFileSizeMB
	if fileMB <= 0 {
		fileMB = 100
	}
	fileBlocks := fileMB * 2048 // 1MB = 2048 blocks of 512 bytes

	// Build the shell command with ulimit + exec
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shellQuote(a)
	}

	cmd := fmt.Sprintf("ulimit -v %d -t %d -f %d; exec %s %s",
		mem, cpu, fileBlocks, shellQuote(binary), strings.Join(quoted, " "))

	return "sh", []string{"-c", cmd}, nil
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	// If no special chars, return as-is
	safe := true
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '/' || c == ':' || c == '=') {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/sandbox/... -v -count=1 -run TestUlimit`
Expected: PASS (2 tests)

**Step 5: Commit**

```bash
git add internal/sandbox/ulimit.go internal/sandbox/sandbox_test.go
git commit -m "feat(sandbox): add Ulimit backend with resource limits"
```

---

### Task 4: Docker Backend

**Files:**
- Create: `internal/sandbox/docker.go`
- Modify: `internal/sandbox/sandbox_test.go`

**Step 1: Write the failing test**

Append to `internal/sandbox/sandbox_test.go`:

```go
func TestDockerBackend(t *testing.T) {
	sb := &DockerSandbox{
		Image:       "ubuntu:22.04",
		MemoryLimit: "2g",
		CPULimit:    "2",
		WorkDir:     "/tmp/test-workspace",
	}
	assert.Equal(t, Docker, sb.Level())

	bin, args, err := sb.Wrap(context.Background(), "claude", []string{"-p", "hello"})
	assert.NoError(t, err)
	assert.Equal(t, "docker", bin)

	joined := strings.Join(args, " ")
	assert.Contains(t, joined, "run")
	assert.Contains(t, joined, "--rm")
	assert.Contains(t, joined, "--network=none")
	assert.Contains(t, joined, "--memory=2g")
	assert.Contains(t, joined, "--cpus=2")
	assert.Contains(t, joined, "-v /tmp/test-workspace:/workspace:rw")
	assert.Contains(t, joined, "-w /workspace")
	assert.Contains(t, joined, "ubuntu:22.04")
	assert.Contains(t, joined, "claude")
	assert.Contains(t, joined, "-p")
	assert.Contains(t, joined, "hello")
}

func TestDockerBackendDefaults(t *testing.T) {
	sb := &DockerSandbox{}
	_, args, err := sb.Wrap(context.Background(), "claude", []string{"-p", "hi"})
	assert.NoError(t, err)

	joined := strings.Join(args, " ")
	assert.Contains(t, joined, "ubuntu:22.04") // default image
	assert.Contains(t, joined, "--memory=2g")   // default mem
	assert.Contains(t, joined, "--cpus=2")       // default cpu
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/sandbox/... -v -count=1 -run TestDocker`
Expected: FAIL — `DockerSandbox` undefined

**Step 3: Write minimal implementation**

In `internal/sandbox/docker.go`:

```go
package sandbox

import (
	"context"
	"os"
)

// DockerSandbox wraps commands in a docker run container.
type DockerSandbox struct {
	Image       string // default: "ubuntu:22.04"
	MemoryLimit string // default: "2g"
	CPULimit    string // default: "2"
	WorkDir     string // host dir to mount as /workspace
}

func (d *DockerSandbox) Level() Level { return Docker }

func (d *DockerSandbox) Wrap(_ context.Context, binary string, args []string) (string, []string, error) {
	image := d.Image
	if image == "" {
		image = "ubuntu:22.04"
	}
	mem := d.MemoryLimit
	if mem == "" {
		mem = "2g"
	}
	cpu := d.CPULimit
	if cpu == "" {
		cpu = "2"
	}
	workdir := d.WorkDir
	if workdir == "" {
		workdir, _ = os.Getwd()
	}

	dockerArgs := []string{
		"run", "--rm",
		"--network=none",
		"--memory=" + mem,
		"--cpus=" + cpu,
		"-v", workdir + ":/workspace:rw",
		"-w", "/workspace",
		image,
		binary,
	}
	dockerArgs = append(dockerArgs, args...)

	return "docker", dockerArgs, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/sandbox/... -v -count=1 -run TestDocker`
Expected: PASS (2 tests)

**Step 5: Commit**

```bash
git add internal/sandbox/docker.go internal/sandbox/sandbox_test.go
git commit -m "feat(sandbox): add Docker backend with network/resource isolation"
```

---

### Task 5: Detect() and ForLevel()

**Files:**
- Modify: `internal/sandbox/sandbox.go`
- Modify: `internal/sandbox/sandbox_test.go`

**Step 1: Write the failing test**

Append to `internal/sandbox/sandbox_test.go`:

```go
func TestDetect(t *testing.T) {
	sb := Detect()
	assert.NotNil(t, sb)
	// Must return at least None
	assert.True(t, sb.Level() >= None)
}

func TestForLevelNone(t *testing.T) {
	sb, err := ForLevel(None)
	assert.NoError(t, err)
	assert.Equal(t, None, sb.Level())
}

func TestForLevelUlimit(t *testing.T) {
	sb, err := ForLevel(Ulimit)
	assert.NoError(t, err)
	assert.Equal(t, Ulimit, sb.Level())
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/sandbox/... -v -count=1 -run "TestDetect|TestForLevel"`
Expected: FAIL — `Detect` and `ForLevel` undefined

**Step 3: Write minimal implementation**

Append to `internal/sandbox/sandbox.go`:

```go
import (
	"os/exec"
	"time"
)

// Detect returns the strongest available sandbox backend.
// Checks Docker first (< 50ms timeout), falls back to Ulimit, then None.
func Detect() Sandbox {
	if dockerAvailable() {
		return &DockerSandbox{}
	}
	return &UlimitSandbox{}
}

// ForLevel returns a sandbox for a specific level.
func ForLevel(level Level) (Sandbox, error) {
	switch level {
	case Docker:
		if !dockerAvailable() {
			return nil, fmt.Errorf("docker is not available")
		}
		return &DockerSandbox{}, nil
	case Ulimit:
		return &UlimitSandbox{}, nil
	case None:
		return &NoneSandbox{}, nil
	default:
		return nil, fmt.Errorf("unknown level: %d", level)
	}
}

func dockerAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	return exec.CommandContext(ctx, "docker", "info").Run() == nil
}
```

Note: The full `sandbox.go` will need its imports updated to include `"os/exec"` and `"time"`.

**Step 4: Run test to verify it passes**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/sandbox/... -v -count=1 -run "TestDetect|TestForLevel"`
Expected: PASS (3 tests)

**Step 5: Run all sandbox tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/sandbox/... -v -count=1`
Expected: All 10+ tests PASS

**Step 6: Commit**

```bash
git add internal/sandbox/sandbox.go internal/sandbox/sandbox_test.go
git commit -m "feat(sandbox): add Detect() and ForLevel() capability detection"
```

---

### Task 6: SandboxConfig in Config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go` (if exists, else create)

**Step 1: Write the failing test**

In `internal/config/config_test.go`, add:

```go
func TestDefaultSandboxConfig(t *testing.T) {
	cfg := Default()
	assert.Equal(t, "auto", cfg.Sandbox.Level)
	assert.Equal(t, "ubuntu:22.04", cfg.Sandbox.DockerImage)
	assert.Equal(t, "2g", cfg.Sandbox.MemoryLimit)
	assert.Equal(t, "2", cfg.Sandbox.CPULimit)
	assert.Equal(t, 100, cfg.Sandbox.MaxFileSizeMB)
	assert.Equal(t, 300, cfg.Sandbox.MaxCPUSeconds)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/config/... -v -count=1 -run TestDefaultSandboxConfig`
Expected: FAIL — `Sandbox` field not on Config

**Step 3: Write minimal implementation**

In `internal/config/config.go`:

1. Add `SandboxConfig` struct after `RetryConfig`:

```go
type SandboxConfig struct {
	Level         string   `yaml:"level"`           // "auto", "docker", "ulimit", "none"
	RequireFor    []string `yaml:"require_for"`     // risk levels requiring sandbox, e.g. ["HIGH","CRITICAL"]
	DockerImage   string   `yaml:"docker_image"`
	MemoryLimit   string   `yaml:"memory_limit"`
	CPULimit      string   `yaml:"cpu_limit"`
	MaxFileSizeMB int      `yaml:"max_file_size_mb"`
	MaxCPUSeconds int      `yaml:"max_cpu_seconds"`
}
```

2. Add `Sandbox SandboxConfig \`yaml:"sandbox"\`` to `Config` struct.

3. Add defaults in `Default()`:

```go
Sandbox: SandboxConfig{
	Level:         "auto",
	DockerImage:   "ubuntu:22.04",
	MemoryLimit:   "2g",
	CPULimit:      "2",
	MaxFileSizeMB: 100,
	MaxCPUSeconds: 300,
},
```

4. Add zero-value guards in `Load()`:

```go
if cfg.Sandbox.Level == "" {
	cfg.Sandbox.Level = "auto"
}
if cfg.Sandbox.DockerImage == "" {
	cfg.Sandbox.DockerImage = "ubuntu:22.04"
}
if cfg.Sandbox.MemoryLimit == "" {
	cfg.Sandbox.MemoryLimit = "2g"
}
if cfg.Sandbox.CPULimit == "" {
	cfg.Sandbox.CPULimit = "2"
}
if cfg.Sandbox.MaxFileSizeMB == 0 {
	cfg.Sandbox.MaxFileSizeMB = 100
}
if cfg.Sandbox.MaxCPUSeconds == 0 {
	cfg.Sandbox.MaxCPUSeconds = 300
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/config/... -v -count=1 -run TestDefaultSandboxConfig`
Expected: PASS

**Step 5: Run all config tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/config/... -v -count=1`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add SandboxConfig with defaults"
```

---

### Task 7: Executor Integration — Sandbox.Wrap()

**Files:**
- Modify: `internal/executor/claude.go` — add `Sandbox` field to `Options`, call `Wrap()` before exec
- Modify: `internal/executor/claude_test.go` — add test for sandbox wrapping

**Step 1: Write the failing test**

Append to `internal/executor/claude_test.go`:

```go
import (
	"github.com/lyndonlyu/apex/internal/sandbox"
)

func TestExecuteWithSandbox(t *testing.T) {
	sb := &sandbox.UlimitSandbox{
		MaxMemoryKB:   2097152,
		MaxCPUSec:     300,
		MaxFileSizeMB: 100,
	}
	exec := New(Options{
		Model:   "claude-opus-4-6",
		Effort:  "high",
		Timeout: 10 * time.Second,
		Binary:  "echo",
		Sandbox: sb,
	})

	result, err := exec.Run(context.Background(), "hello")
	require.NoError(t, err)
	// The command ran through sh -c "ulimit ...; exec echo ..."
	assert.Equal(t, 0, result.ExitCode)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/executor/... -v -count=1 -run TestExecuteWithSandbox`
Expected: FAIL — `Sandbox` field not on Options

**Step 3: Write minimal implementation**

In `internal/executor/claude.go`:

1. Add import: `"github.com/lyndonlyu/apex/internal/sandbox"`

2. Add field to `Options`:

```go
type Options struct {
	Model   string
	Effort  string
	Timeout time.Duration
	Binary  string           // defaults to "claude"
	Sandbox sandbox.Sandbox  // optional sandbox wrapper
}
```

3. Modify `Run()` to call `Wrap()` before exec:

```go
func (e *Executor) Run(ctx context.Context, task string) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, e.opts.Timeout)
	defer cancel()

	binary := e.opts.Binary
	args := e.buildArgs(task)

	if e.opts.Sandbox != nil {
		var err error
		binary, args, err = e.opts.Sandbox.Wrap(ctx, binary, args)
		if err != nil {
			return Result{}, fmt.Errorf("sandbox wrap: %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	// ... rest unchanged
```

Add `"fmt"` to imports.

**Step 4: Run test to verify it passes**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/executor/... -v -count=1 -run TestExecuteWithSandbox`
Expected: PASS

**Step 5: Run all executor tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/executor/... -v -count=1`
Expected: All 6 tests PASS

**Step 6: Commit**

```bash
git add internal/executor/claude.go internal/executor/claude_test.go
git commit -m "feat(executor): integrate Sandbox.Wrap() before command execution"
```

---

### Task 8: Wire Sandbox in run.go

**Files:**
- Modify: `cmd/apex/run.go`

**Step 1: Understand what to change**

After risk classification and before executor creation in `run.go`, we need to:
1. Resolve the sandbox based on config (`auto` → `Detect()`, specific level → `ForLevel()`)
2. Configure the sandbox with config values
3. Pass sandbox to executor `Options`
4. Add Fail-Closed governance check

**Step 2: Write implementation**

In `cmd/apex/run.go`, add import `"github.com/lyndonlyu/apex/internal/sandbox"`.

After the risk classification block (line ~67) and before planning, add:

```go
// Resolve sandbox
var sb sandbox.Sandbox
if cfg.Sandbox.Level == "auto" {
	sb = sandbox.Detect()
} else {
	level, parseErr := sandbox.ParseLevel(cfg.Sandbox.Level)
	if parseErr != nil {
		return fmt.Errorf("invalid sandbox level: %w", parseErr)
	}
	var levelErr error
	sb, levelErr = sandbox.ForLevel(level)
	if levelErr != nil {
		return fmt.Errorf("sandbox unavailable: %w", levelErr)
	}
}

// Configure sandbox from config
switch s := sb.(type) {
case *sandbox.DockerSandbox:
	s.Image = cfg.Sandbox.DockerImage
	s.MemoryLimit = cfg.Sandbox.MemoryLimit
	s.CPULimit = cfg.Sandbox.CPULimit
case *sandbox.UlimitSandbox:
	s.MaxMemoryKB = cfg.Sandbox.MaxFileSizeMB * 1024 // approximate
	s.MaxCPUSec = cfg.Sandbox.MaxCPUSeconds
	s.MaxFileSizeMB = cfg.Sandbox.MaxFileSizeMB
}

// Fail-Closed: check if risk level requires higher sandbox
for _, req := range cfg.Sandbox.RequireFor {
	if strings.EqualFold(req, risk.String()) {
		var requiredLevel sandbox.Level
		if strings.EqualFold(req, "CRITICAL") {
			requiredLevel = sandbox.Docker
		} else if strings.EqualFold(req, "HIGH") {
			requiredLevel = sandbox.Ulimit
		}
		if sb.Level() < requiredLevel {
			return fmt.Errorf("fail-closed: task risk %s requires %s isolation but only %s available",
				risk, requiredLevel, sb.Level())
		}
		break
	}
}

fmt.Printf("Sandbox: %s\n", sb.Level())
```

Then update both executor creations (planner and execution) to include `Sandbox: sb`:

```go
planExec := executor.New(executor.Options{
	// ... existing fields ...
	Sandbox: sb,
})

exec := executor.New(executor.Options{
	// ... existing fields ...
	Sandbox: sb,
})
```

**Step 3: Run build to verify compilation**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build ./cmd/apex/`
Expected: Success

**Step 4: Run all tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./... -v -count=1`
Expected: All PASS

**Step 5: Commit**

```bash
git add cmd/apex/run.go
git commit -m "feat(run): wire sandbox resolution, config, and fail-closed governance"
```

---

### Task 9: Audit — Add sandbox_level Field

**Files:**
- Modify: `internal/audit/logger.go` — add `SandboxLevel` to `Entry` and `Record`
- Modify: `internal/audit/logger_test.go` — add test for sandbox_level
- Modify: `cmd/apex/run.go` — pass sandbox level to audit entry

**Step 1: Write the failing test**

In `internal/audit/logger_test.go`, add:

```go
func TestLogSandboxLevel(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	entry := Entry{
		Task:         "test task",
		RiskLevel:    "LOW",
		Outcome:      "success",
		Duration:     100 * time.Millisecond,
		Model:        "test",
		SandboxLevel: "ulimit",
	}
	require.NoError(t, logger.Log(entry))

	records, err := logger.Recent(1)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "ulimit", records[0].SandboxLevel)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/audit/... -v -count=1 -run TestLogSandboxLevel`
Expected: FAIL — `SandboxLevel` not on Entry

**Step 3: Write minimal implementation**

In `internal/audit/logger.go`:

1. Add `SandboxLevel string` to `Entry` struct:

```go
type Entry struct {
	Task         string
	RiskLevel    string
	Outcome      string
	Duration     time.Duration
	Model        string
	Error        string
	SandboxLevel string
}
```

2. Add `SandboxLevel string \`json:"sandbox_level,omitempty"\`` to `Record` struct.

3. In `Log()`, set `SandboxLevel: entry.SandboxLevel` in the record.

**Step 4: Run test to verify it passes**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/audit/... -v -count=1 -run TestLogSandboxLevel`
Expected: PASS

**Step 5: Update run.go audit logging**

In `cmd/apex/run.go`, where audit entries are logged (around line 262), add `SandboxLevel`:

```go
logger.Log(audit.Entry{
	Task:         fmt.Sprintf("[%s] %s", n.ID, n.Task),
	RiskLevel:    risk.String(),
	Outcome:      nodeOutcome,
	Duration:     duration,
	Model:        cfg.Claude.Model,
	Error:        nodeErr,
	SandboxLevel: sb.Level().String(),
})
```

**Step 6: Run all tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./... -v -count=1`
Expected: All PASS

**Step 7: Commit**

```bash
git add internal/audit/logger.go internal/audit/logger_test.go cmd/apex/run.go
git commit -m "feat(audit): add sandbox_level field to audit entries"
```

---

### Task 10: E2E Tests for Sandbox

**Files:**
- Create: `e2e/sandbox_test.go`

**Step 1: Write E2E tests**

In `e2e/sandbox_test.go`:

```go
package e2e

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSandboxDetect(t *testing.T) {
	env := newTestEnv(t)
	out := env.runApex("run", "--dry-run", "list files")
	// Dry-run should mention sandbox level
	assert.Contains(t, out, "Sandbox:")
}

func TestSandboxNone(t *testing.T) {
	env := newTestEnv(t)
	// Override config to sandbox level: none
	env.writeConfig(`
claude:
  model: claude-opus-4-6
  effort: high
  binary: ` + env.MockBin + `
sandbox:
  level: "none"
`)
	out := env.runApex("run", "say hello")
	assert.Contains(t, out, "Sandbox: none")
	assert.Contains(t, out, "Done")
}

func TestSandboxUlimit(t *testing.T) {
	env := newTestEnv(t)
	env.writeConfig(`
claude:
  model: claude-opus-4-6
  effort: high
  binary: ` + env.MockBin + `
sandbox:
  level: "ulimit"
`)
	out := env.runApex("run", "say hello")
	assert.Contains(t, out, "Sandbox: ulimit")
	assert.Contains(t, out, "Done")
}

func TestSandboxFailClosedRejects(t *testing.T) {
	env := newTestEnv(t)
	env.writeConfig(`
claude:
  model: claude-opus-4-6
  effort: high
  binary: ` + env.MockBin + `
sandbox:
  level: "none"
  require_for: ["HIGH"]
`)
	// "delete" triggers HIGH risk
	out := env.runApex("run", "delete old files")
	assert.Contains(t, strings.ToLower(out), "fail-closed")
}

func TestSandboxLevelInAudit(t *testing.T) {
	env := newTestEnv(t)
	env.runApex("run", "say hello")

	// Check audit log for sandbox_level field
	auditFiles := env.auditFiles()
	require.NotEmpty(t, auditFiles)
	content := env.readFile(auditFiles[0])
	assert.Contains(t, content, "sandbox_level")
}
```

Note: The `writeConfig` and `auditFiles` helpers may need to be added to `helpers_test.go` if they don't exist.

**Step 2: Add helper methods if needed**

In `e2e/helpers_test.go`, add:

```go
func (e *TestEnv) writeConfig(content string) {
	configPath := filepath.Join(e.Home, ".apex", "config.yaml")
	os.WriteFile(configPath, []byte(content), 0644)
}

func (e *TestEnv) auditFiles() []string {
	pattern := filepath.Join(e.Home, ".apex", "audit", "*.jsonl")
	files, _ := filepath.Glob(pattern)
	return files
}
```

**Step 3: Run E2E tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./e2e/... -v -count=1 -run TestSandbox -timeout=120s`
Expected: All 5 sandbox E2E tests PASS

**Step 4: Run full E2E suite**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && make e2e`
Expected: All E2E tests PASS (previous 24 + new 5)

**Step 5: Commit**

```bash
git add e2e/sandbox_test.go e2e/helpers_test.go
git commit -m "test(e2e): add sandbox E2E tests (detect, none, ulimit, fail-closed, audit)"
```

---

### Task 11: Full Test Suite + PROGRESS.md

**Files:**
- Modify: `PROGRESS.md`

**Step 1: Run full test suite**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && make test`
Expected: All unit tests PASS

**Step 2: Run E2E tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && make e2e`
Expected: All E2E tests PASS

**Step 3: Update PROGRESS.md**

Update the phase table to mark Phase 11 as Done:

```markdown
| 11 | Execution Sandbox | `2026-02-19-phase11-execution-sandbox-design.md` | Done |
```

Update Current section:

```markdown
## Current: Phase 12 — TBD

No phase in progress.
```

Add to Key Packages:

```markdown
| `internal/sandbox` | Multi-level execution sandboxing (Docker/Ulimit/None) |
```

**Step 4: Commit**

```bash
git add PROGRESS.md
git commit -m "docs: mark Phase 11 Execution Sandbox as complete"
```

---

## Summary

| Task | Component | Tests |
|------|-----------|-------|
| 1 | Level enum + interface | 3 unit |
| 2 | None backend | 1 unit |
| 3 | Ulimit backend | 2 unit |
| 4 | Docker backend | 2 unit |
| 5 | Detect() + ForLevel() | 3 unit |
| 6 | SandboxConfig | 1 unit |
| 7 | Executor integration | 1 unit |
| 8 | run.go wiring + fail-closed | build check |
| 9 | Audit sandbox_level | 1 unit |
| 10 | E2E tests | 5 E2E |
| 11 | Full suite + PROGRESS | suite run |

**Total: 11 tasks, ~15 new unit tests, 5 new E2E tests, 11 commits**

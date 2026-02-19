# Phase 11: Execution Sandbox Design

**Date:** 2026-02-19 | **Phase:** 11 | **Status:** Approved

## Goal

Add multi-level execution sandboxing to Apex so that agent tasks run in isolated environments. The sandbox layer sits between the executor and the OS, wrapping commands with the strongest available isolation backend. A Capability Matrix detects available backends at startup. Governance integrates with sandbox levels for Fail-Closed enforcement.

## Architecture

### Sandbox Levels (strongest to weakest)

| Level | Isolation | Platform | Detection |
|-------|-----------|----------|-----------|
| `Docker` | Network + FS + Process | All (requires Docker) | `docker info` succeeds |
| `Ulimit` | Resource limits (CPU/mem/files) | All | Always available |
| `None` | No isolation | All | Fallback |

> bubblewrap (Linux-only) omitted for now — can be added later as a level between Docker and Ulimit.

### Package Layout

```
internal/sandbox/
├── sandbox.go         # Level enum, Sandbox interface, Detect()
├── docker.go          # Docker backend: wraps cmd in docker run
├── ulimit.go          # Ulimit backend: wraps cmd with ulimit prefixes
├── none.go            # No-op passthrough
└── sandbox_test.go    # Unit tests
```

### Core Interface

```go
type Level int

const (
    None   Level = iota
    Ulimit
    Docker
)

type Sandbox interface {
    Level() Level
    Wrap(ctx context.Context, binary string, args []string) (string, []string, error)
}

// Detect returns the strongest available sandbox backend.
// Checks Docker first (< 50ms timeout), falls back to Ulimit, then None.
func Detect() Sandbox

// ForLevel returns a sandbox for a specific level, or error if unavailable.
func ForLevel(level Level) (Sandbox, error)
```

### Docker Backend

Wraps the claude command inside `docker run`:

```
docker run --rm --network=none \
  -v <workdir>:/workspace:rw \
  -w /workspace \
  --memory=2g --cpus=2 \
  <image> <binary> <args...>
```

Key properties:
- `--network=none` — no network access
- `--memory` / `--cpus` — resource limits
- Read-only mounts for sensitive paths excluded
- Configurable image in config.yaml

### Ulimit Backend

Prefixes the command with resource limits:

```
ulimit -v <max_memory_kb> -t <max_cpu_seconds> -f <max_file_size_blocks>
```

On macOS, uses `ulimit` in a subshell wrapper. Lighter than Docker but no FS/network isolation.

### None Backend

Passthrough — returns the command unchanged. Used when no isolation is available and the task's risk level doesn't require it.

## Integration Points

### 1. Config (`internal/config/config.go`)

```go
type SandboxConfig struct {
    Level              string   `yaml:"level"`                // "auto", "docker", "ulimit", "none"
    RequireFor         []string `yaml:"require_for"`          // risk levels requiring isolation, e.g. ["HIGH", "CRITICAL"]
    DockerImage        string   `yaml:"docker_image"`         // default: "ubuntu:22.04"
    MemoryLimit        string   `yaml:"memory_limit"`         // default: "2g"
    CPULimit           string   `yaml:"cpu_limit"`            // default: "2"
    MaxFileSizeMB      int      `yaml:"max_file_size_mb"`     // default: 100 (ulimit)
    MaxCPUSeconds      int      `yaml:"max_cpu_seconds"`      // default: 300 (ulimit)
}
```

### 2. Executor (`internal/executor/claude.go`)

Before running the command, the executor calls `sandbox.Wrap()` to transform the binary+args:

```go
func (e *Executor) Run(ctx context.Context, task string) (Result, error) {
    binary, args := e.opts.Binary, e.buildArgs(task)

    if e.opts.Sandbox != nil {
        var err error
        binary, args, err = e.opts.Sandbox.Wrap(ctx, binary, args)
        if err != nil {
            return Result{}, fmt.Errorf("sandbox wrap: %w", err)
        }
    }

    cmd := exec.CommandContext(ctx, binary, args...)
    // ... rest unchanged
}
```

### 3. Governance — Fail-Closed

In `cmd/apex/run.go`, after risk classification:

```go
if sandbox.Level() < requiredLevel {
    return fmt.Errorf("fail-closed: task requires %s isolation but only %s available", required, sandbox.Level())
}
```

Logic:
- If risk is in `sandbox.require_for` list AND available sandbox level is below threshold → reject
- CRITICAL always requires Docker (if configured)
- HIGH requires at least Ulimit

### 4. Audit

Add `sandbox_level` field to audit entries so every execution records what isolation was used.

## E2E Tests

| Test | Scenario | Assert |
|------|----------|--------|
| `TestSandboxDetect` | Call `Detect()` | Returns a valid level |
| `TestRunWithUlimit` | Config `level: ulimit` | Task runs with resource limits |
| `TestRunWithNone` | Config `level: none` | Task runs without sandbox |
| `TestFailClosedRejects` | Config requires Docker, Docker unavailable | Exit non-zero, error mentions "fail-closed" |
| `TestSandboxLevelInAudit` | Run a task | Audit entry contains sandbox_level |

## Design Decisions

1. **Why not bubblewrap first?** We're on macOS. bubblewrap is Linux-only. Docker provides equivalent isolation cross-platform. bubblewrap can be added later as a Linux-optimized path.

2. **Why Sandbox.Wrap() returns new binary+args instead of modifying exec.Cmd?** Cleaner interface — the sandbox transforms the command, the executor handles exec.Cmd creation. Easier to test.

3. **Why `Detect()` with < 50ms timeout?** `docker info` can hang if Docker daemon is unresponsive. A fast timeout ensures startup isn't blocked.

4. **Why record sandbox_level in audit?** Traceability — knowing what isolation level was used for each task is essential for security review.

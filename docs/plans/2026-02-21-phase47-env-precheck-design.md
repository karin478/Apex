# Phase 47: Environment Precheck

> Design doc for Apex Agent CLI — startup safety checks for runtime environment validation.

## Problem

The system starts execution without verifying its runtime environment. Missing directories, unavailable databases, invalid configs, or missing binaries can cause failures deep into execution. There is no structured way to validate the environment before starting work.

## Solution

A `precheck` package that defines a `Check` interface with pluggable validation checks. A `Runner` executes all registered checks and reports pass/fail results. Built-in checks cover: data directory existence, config file validity, Claude binary availability, and database connectivity.

## Architecture

```
internal/precheck/
├── precheck.go       # Check interface, CheckResult, Runner, built-in checks
└── precheck_test.go  # 7 unit tests
```

## Key Types

### Check

```go
type Check interface {
    Name() string
    Run() CheckResult
}
```

### CheckResult

```go
type CheckResult struct {
    Name    string `json:"name"`
    Passed  bool   `json:"passed"`
    Message string `json:"message"`
}
```

### Runner

```go
type Runner struct {
    mu     sync.RWMutex
    checks []Check
}
```

### RunResult

```go
type RunResult struct {
    AllPassed bool          `json:"all_passed"`
    Results   []CheckResult `json:"results"`
    Duration  string        `json:"duration"` // e.g. "45ms"
}
```

## Built-in Checks

### DirCheck

```go
type DirCheck struct {
    Dir string
}
```
Verifies directory exists and is accessible. Name: "dir:{path}"

### FileCheck

```go
type FileCheck struct {
    Path string
    Desc string
}
```
Verifies file exists. Name: "file:{desc}"

### BinaryCheck

```go
type BinaryCheck struct {
    Binary string
}
```
Verifies binary is in PATH using `exec.LookPath`. Name: "binary:{name}"

### CustomCheck

```go
type CustomCheck struct {
    CheckName string
    Fn        func() CheckResult
}
```
Wraps an arbitrary function as a Check. Name: CheckName.

## Core Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `NewRunner` | `() *Runner` | Creates empty runner |
| `(*Runner) Add` | `(c Check)` | Adds a check |
| `(*Runner) Run` | `() RunResult` | Runs all checks, returns aggregate result |
| `(*Runner) Checks` | `() []string` | Returns check names |
| `DefaultRunner` | `(home string) *Runner` | Creates runner with standard checks for apex data dirs |

### DefaultRunner Checks

Given `home` (e.g., "/Users/lyndon"):
1. DirCheck for `{home}/.apex`
2. DirCheck for `{home}/.apex/audit`
3. DirCheck for `{home}/.apex/runs`
4. DirCheck for `{home}/.apex/memory`
5. FileCheck for `{home}/.apex/config.yaml` (desc: "config")

## Design Decisions

### Check Interface

Pluggable — users can add custom checks without modifying the package. Built-in checks cover the common cases.

### Runner Collects All Results

Even if one check fails, the runner continues and reports all results. This gives the user a complete picture rather than stopping at the first failure.

### Duration as String

Human-readable duration (e.g., "45ms", "1.2s") using `time.Duration.String()`.

### DefaultRunner Takes Home Path

Parameterized for testability. The CLI passes `os.Getenv("HOME")`, tests pass `t.TempDir()`.

## CLI Commands

### `apex precheck [--format json]`
Runs all environment checks and displays results.

## Unit Tests (7)

| Test | Description |
|------|-------------|
| `TestDirCheckPass` | Existing directory → passed=true |
| `TestDirCheckFail` | Non-existing directory → passed=false |
| `TestFileCheckPass` | Existing file → passed=true |
| `TestFileCheckFail` | Non-existing file → passed=false |
| `TestBinaryCheck` | "go" binary should be found; "nonexistent_xyz_abc" should fail |
| `TestRunnerAllPass` | All checks pass → AllPassed=true |
| `TestRunnerOneFail` | One check fails → AllPassed=false, all results reported |

## E2E Tests (2)

| Test | Description |
|------|-------------|
| `TestPrecheck` | CLI runs precheck, exit 0, stdout contains check results |
| `TestPrecheckJSON` | CLI runs precheck --format json, contains "all_passed" |

## Format Functions

| Function | Description |
|----------|-------------|
| `FormatRunResult(result RunResult) string` | Results display with PASS/FAIL indicators and duration |
| `FormatRunResultJSON(result RunResult) (string, error)` | JSON output |

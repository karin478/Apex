# Phase 37: Mode Selector

> Design doc for Apex Agent CLI — execution mode selection based on task complexity.

## Problem

The agent system treats all tasks uniformly: same token budget, same concurrency, same timeouts. But tasks vary wildly — a quick status check needs minimal resources, while a deep exploratory analysis needs generous token reserves and extended timeouts. Without mode-aware execution, simple tasks waste resources and complex tasks get starved.

## Solution

A `mode` package that defines 5 execution modes with distinct configurations. A `Selector` manages mode registration, manual selection, and complexity-based automatic selection.

## Architecture

```
internal/mode/
├── mode.go       # Mode enum, ModeConfig, Selector, SelectByComplexity
└── mode_test.go  # 7 unit tests
```

## Key Types

### Mode

```go
type Mode string

const (
    ModeNormal      Mode = "NORMAL"
    ModeUrgent      Mode = "URGENT"
    ModeExploratory Mode = "EXPLORATORY"
    ModeBatch       Mode = "BATCH"
    ModeLongRunning Mode = "LONG_RUNNING"
)
```

### ModeConfig

```go
type ModeConfig struct {
    Name           Mode          `json:"name"            yaml:"name"`
    TokenReserve   int           `json:"token_reserve"   yaml:"token_reserve"`
    Concurrency    int           `json:"concurrency"     yaml:"concurrency"`
    SkipValidation bool          `json:"skip_validation" yaml:"skip_validation"`
    Timeout        time.Duration `json:"timeout"         yaml:"timeout"`
}
```

### Selector

```go
type Selector struct {
    modes   map[Mode]ModeConfig
    current Mode
}
```

## Core Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `DefaultModes` | `() map[Mode]ModeConfig` | Returns the 5 built-in mode configs |
| `NewSelector` | `(modes map[Mode]ModeConfig) *Selector` | Creates Selector with given modes |
| `(*Selector) Select` | `(mode Mode) error` | Manually select a mode; error if unknown |
| `(*Selector) Current` | `() (Mode, ModeConfig)` | Returns current mode and its config |
| `(*Selector) List` | `() []ModeConfig` | Returns all registered modes sorted by name |
| `(*Selector) SelectByComplexity` | `(score int) Mode` | Auto-select based on complexity score |
| `(*Selector) Config` | `(mode Mode) (ModeConfig, error)` | Returns config for a specific mode |

## Default Mode Configurations

| Mode | Token Reserve | Concurrency | Skip Validation | Timeout |
|------|--------------|-------------|-----------------|---------|
| `NORMAL` | 4000 | 2 | false | 5m |
| `URGENT` | 2000 | 4 | true | 2m |
| `EXPLORATORY` | 8000 | 1 | false | 10m |
| `BATCH` | 4000 | 8 | false | 30m |
| `LONG_RUNNING` | 6000 | 2 | false | 60m |

## Complexity-Based Selection

`SelectByComplexity(score int) Mode` selects a mode based on integer score:

| Score Range | Selected Mode |
|-------------|---------------|
| < 30 | NORMAL |
| 30–60 | EXPLORATORY |
| > 60 | LONG_RUNNING |

The score is an opaque integer provided by callers (planner, agent framework, etc.). The selector has no opinion on how the score is computed.

## Design Decisions

### Mode as String Enum

Using `type Mode string` instead of `iota` ints for JSON/YAML serialization friendliness and readable log output.

### Selector Defaults to NORMAL

`NewSelector` sets `current = ModeNormal`. All operations before any explicit `Select()` call use NORMAL mode.

### Sorted List Output

`List()` returns modes sorted alphabetically by name for deterministic output in CLI and tests.

### Complexity Thresholds

Fixed thresholds (30, 60) are hardcoded rather than configurable, matching the principle of minimal complexity. They can be adjusted in code if real usage data warrants it.

## CLI Commands

### `apex mode list`
Lists all available modes with their configurations.

### `apex mode select <mode>`
Manually selects an execution mode. Validates that the mode exists.

### `apex mode config <mode>`
Shows detailed configuration for a specific mode.

## Unit Tests (7)

| Test | Description |
|------|-------------|
| `TestDefaultModes` | Returns 5 modes with correct names |
| `TestNewSelector` | Default current mode is NORMAL |
| `TestSelectorSelect` | Valid mode → success; unknown mode → error |
| `TestSelectorCurrent` | Returns current mode and matching config |
| `TestSelectorList` | Returns all modes sorted alphabetically |
| `TestSelectByComplexity` | Score < 30 → NORMAL; 30-60 → EXPLORATORY; > 60 → LONG_RUNNING |
| `TestSelectorConfig` | Known mode → config; unknown mode → error |

## E2E Tests (3)

| Test | Description |
|------|-------------|
| `TestModeList` | CLI invocation → mode list output |
| `TestModeSelect` | CLI invocation → mode selected confirmation |
| `TestModeConfig` | CLI invocation → mode config details |

## Format Functions

| Function | Description |
|----------|-------------|
| `FormatModeList(modes []ModeConfig) string` | Table: NAME / TOKEN_RESERVE / CONCURRENCY / SKIP_VALIDATION / TIMEOUT |
| `FormatModeConfig(config ModeConfig) string` | Detailed single-mode display |
| `FormatModeListJSON(modes []ModeConfig) (string, error)` | JSON output |

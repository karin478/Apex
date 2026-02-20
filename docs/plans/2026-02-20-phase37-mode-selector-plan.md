# Mode Selector Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an `internal/mode` package that defines 5 execution modes with distinct configurations and complexity-based automatic mode selection.

**Architecture:** Mode string enum, ModeConfig struct, Selector for mode management/selection/complexity-based auto-selection. Format functions + Cobra CLI.

**Tech Stack:** Go, `time`, `encoding/json`, `sort`, Testify, Cobra CLI

---

### Task 1: Mode Selector Core — Modes + Selector + SelectByComplexity (7 tests)

**Files:**
- Create: `internal/mode/mode.go`
- Create: `internal/mode/mode_test.go`

**Step 1: Write 7 failing tests**

```go
package mode

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultModes(t *testing.T) {
	modes := DefaultModes()
	assert.Len(t, modes, 5)
	assert.Contains(t, modes, ModeNormal)
	assert.Contains(t, modes, ModeUrgent)
	assert.Contains(t, modes, ModeExploratory)
	assert.Contains(t, modes, ModeBatch)
	assert.Contains(t, modes, ModeLongRunning)
}

func TestNewSelector(t *testing.T) {
	s := NewSelector(DefaultModes())
	current, _ := s.Current()
	assert.Equal(t, ModeNormal, current)
}

func TestSelectorSelect(t *testing.T) {
	s := NewSelector(DefaultModes())

	t.Run("valid mode", func(t *testing.T) {
		err := s.Select(ModeUrgent)
		require.NoError(t, err)
		current, _ := s.Current()
		assert.Equal(t, ModeUrgent, current)
	})

	t.Run("unknown mode", func(t *testing.T) {
		err := s.Select(Mode("INVALID"))
		assert.Error(t, err)
	})
}

func TestSelectorCurrent(t *testing.T) {
	s := NewSelector(DefaultModes())
	err := s.Select(ModeExploratory)
	require.NoError(t, err)

	current, config := s.Current()
	assert.Equal(t, ModeExploratory, current)
	assert.Equal(t, ModeExploratory, config.Name)
	assert.Equal(t, 8000, config.TokenReserve)
	assert.Equal(t, 1, config.Concurrency)
}

func TestSelectorList(t *testing.T) {
	s := NewSelector(DefaultModes())
	list := s.List()
	assert.Len(t, list, 5)
	// Verify sorted by name.
	for i := 1; i < len(list); i++ {
		assert.True(t, string(list[i-1].Name) <= string(list[i].Name),
			"expected sorted order, got %s before %s", list[i-1].Name, list[i].Name)
	}
}

func TestSelectByComplexity(t *testing.T) {
	s := NewSelector(DefaultModes())

	t.Run("low complexity", func(t *testing.T) {
		mode := s.SelectByComplexity(15)
		assert.Equal(t, ModeNormal, mode)
	})

	t.Run("medium complexity", func(t *testing.T) {
		mode := s.SelectByComplexity(45)
		assert.Equal(t, ModeExploratory, mode)
	})

	t.Run("high complexity", func(t *testing.T) {
		mode := s.SelectByComplexity(80)
		assert.Equal(t, ModeLongRunning, mode)
	})

	t.Run("boundary 30", func(t *testing.T) {
		mode := s.SelectByComplexity(30)
		assert.Equal(t, ModeExploratory, mode)
	})

	t.Run("boundary 60", func(t *testing.T) {
		mode := s.SelectByComplexity(60)
		assert.Equal(t, ModeExploratory, mode)
	})
}

func TestSelectorConfig(t *testing.T) {
	s := NewSelector(DefaultModes())

	t.Run("known mode", func(t *testing.T) {
		config, err := s.Config(ModeBatch)
		require.NoError(t, err)
		assert.Equal(t, ModeBatch, config.Name)
		assert.Equal(t, 8, config.Concurrency)
	})

	t.Run("unknown mode", func(t *testing.T) {
		_, err := s.Config(Mode("NOPE"))
		assert.Error(t, err)
	})
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/mode/ -v -count=1`
Expected: FAIL — functions not defined

**Step 3: Write minimal implementation**

```go
// Package mode provides execution mode selection based on task complexity.
package mode

import (
	"fmt"
	"sort"
	"time"
)

// Mode represents an execution mode.
type Mode string

const (
	ModeNormal      Mode = "NORMAL"
	ModeUrgent      Mode = "URGENT"
	ModeExploratory Mode = "EXPLORATORY"
	ModeBatch       Mode = "BATCH"
	ModeLongRunning Mode = "LONG_RUNNING"
)

// ModeConfig holds the configuration for an execution mode.
type ModeConfig struct {
	Name           Mode          `json:"name"            yaml:"name"`
	TokenReserve   int           `json:"token_reserve"   yaml:"token_reserve"`
	Concurrency    int           `json:"concurrency"     yaml:"concurrency"`
	SkipValidation bool          `json:"skip_validation" yaml:"skip_validation"`
	Timeout        time.Duration `json:"timeout"         yaml:"timeout"`
}

// DefaultModes returns the 5 built-in mode configurations.
func DefaultModes() map[Mode]ModeConfig {
	return map[Mode]ModeConfig{
		ModeNormal: {
			Name: ModeNormal, TokenReserve: 4000, Concurrency: 2,
			SkipValidation: false, Timeout: 5 * time.Minute,
		},
		ModeUrgent: {
			Name: ModeUrgent, TokenReserve: 2000, Concurrency: 4,
			SkipValidation: true, Timeout: 2 * time.Minute,
		},
		ModeExploratory: {
			Name: ModeExploratory, TokenReserve: 8000, Concurrency: 1,
			SkipValidation: false, Timeout: 10 * time.Minute,
		},
		ModeBatch: {
			Name: ModeBatch, TokenReserve: 4000, Concurrency: 8,
			SkipValidation: false, Timeout: 30 * time.Minute,
		},
		ModeLongRunning: {
			Name: ModeLongRunning, TokenReserve: 6000, Concurrency: 2,
			SkipValidation: false, Timeout: 60 * time.Minute,
		},
	}
}

// Selector manages mode registration and selection.
type Selector struct {
	modes   map[Mode]ModeConfig
	current Mode
}

// NewSelector creates a Selector with the given modes, defaulting to NORMAL.
func NewSelector(modes map[Mode]ModeConfig) *Selector {
	return &Selector{modes: modes, current: ModeNormal}
}

// Select manually selects a mode. Returns an error if the mode is unknown.
func (s *Selector) Select(mode Mode) error {
	if _, ok := s.modes[mode]; !ok {
		return fmt.Errorf("mode: unknown mode %q", mode)
	}
	s.current = mode
	return nil
}

// Current returns the current mode and its configuration.
func (s *Selector) Current() (Mode, ModeConfig) {
	return s.current, s.modes[s.current]
}

// List returns all registered modes sorted alphabetically by name.
func (s *Selector) List() []ModeConfig {
	configs := make([]ModeConfig, 0, len(s.modes))
	for _, cfg := range s.modes {
		configs = append(configs, cfg)
	}
	sort.Slice(configs, func(i, j int) bool {
		return string(configs[i].Name) < string(configs[j].Name)
	})
	return configs
}

// SelectByComplexity selects a mode based on the complexity score.
// Score < 30 → NORMAL, 30-60 → EXPLORATORY, > 60 → LONG_RUNNING.
// Also updates the current mode.
func (s *Selector) SelectByComplexity(score int) Mode {
	var mode Mode
	switch {
	case score > 60:
		mode = ModeLongRunning
	case score >= 30:
		mode = ModeExploratory
	default:
		mode = ModeNormal
	}
	s.current = mode
	return mode
}

// Config returns the configuration for a specific mode.
func (s *Selector) Config(mode Mode) (ModeConfig, error) {
	cfg, ok := s.modes[mode]
	if !ok {
		return ModeConfig{}, fmt.Errorf("mode: unknown mode %q", mode)
	}
	return cfg, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/mode/ -v -count=1 -race`
Expected: PASS (7 tests)

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/mode/mode.go internal/mode/mode_test.go
git commit -m "feat(mode): add execution mode selector with 5 modes and complexity-based selection

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: Format Functions + CLI Commands

**Files:**
- Create: `internal/mode/format.go`
- Create: `cmd/apex/mode.go`
- Modify: `cmd/apex/main.go:54` (add `rootCmd.AddCommand(modeCmd)`)

**Step 1: Write format.go**

```go
package mode

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatModeList formats a list of modes as a human-readable table.
func FormatModeList(modes []ModeConfig) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-15s %-14s %-13s %-16s %s\n",
		"NAME", "TOKEN_RESERVE", "CONCURRENCY", "SKIP_VALIDATION", "TIMEOUT")
	for _, m := range modes {
		fmt.Fprintf(&b, "%-15s %-14d %-13d %-16v %s\n",
			m.Name, m.TokenReserve, m.Concurrency, m.SkipValidation, m.Timeout)
	}
	return b.String()
}

// FormatModeConfig formats a single mode configuration for display.
func FormatModeConfig(config ModeConfig) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name:            %s\n", config.Name)
	fmt.Fprintf(&b, "Token Reserve:   %d\n", config.TokenReserve)
	fmt.Fprintf(&b, "Concurrency:     %d\n", config.Concurrency)
	fmt.Fprintf(&b, "Skip Validation: %v\n", config.SkipValidation)
	fmt.Fprintf(&b, "Timeout:         %s\n", config.Timeout)
	return b.String()
}

// FormatModeListJSON formats mode configs as indented JSON.
func FormatModeListJSON(modes []ModeConfig) (string, error) {
	data, err := json.MarshalIndent(modes, "", "  ")
	if err != nil {
		return "", fmt.Errorf("mode: json marshal: %w", err)
	}
	return string(data), nil
}
```

**Step 2: Write mode.go CLI**

```go
package main

import (
	"fmt"
	"strings"

	"github.com/lyndonlyu/apex/internal/mode"
	"github.com/spf13/cobra"
)

var modeFormat string

var modeCmd = &cobra.Command{
	Use:   "mode",
	Short: "Execution mode management",
}

var modeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available execution modes",
	RunE:  runModeList,
}

var modeSelectCmd = &cobra.Command{
	Use:   "select <mode>",
	Short: "Select an execution mode",
	Args:  cobra.ExactArgs(1),
	RunE:  runModeSelect,
}

var modeConfigCmd = &cobra.Command{
	Use:   "config <mode>",
	Short: "Show configuration for a specific mode",
	Args:  cobra.ExactArgs(1),
	RunE:  runModeConfig,
}

func init() {
	modeListCmd.Flags().StringVar(&modeFormat, "format", "", "Output format (json)")
	modeCmd.AddCommand(modeListCmd, modeSelectCmd, modeConfigCmd)
}

func runModeList(cmd *cobra.Command, args []string) error {
	s := mode.NewSelector(mode.DefaultModes())
	list := s.List()

	if modeFormat == "json" {
		out, err := mode.FormatModeListJSON(list)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(mode.FormatModeList(list))
	}
	return nil
}

func runModeSelect(cmd *cobra.Command, args []string) error {
	s := mode.NewSelector(mode.DefaultModes())
	m := mode.Mode(strings.ToUpper(args[0]))

	if err := s.Select(m); err != nil {
		return err
	}
	current, _ := s.Current()
	fmt.Printf("Mode selected: %s\n", current)
	return nil
}

func runModeConfig(cmd *cobra.Command, args []string) error {
	s := mode.NewSelector(mode.DefaultModes())
	m := mode.Mode(strings.ToUpper(args[0]))

	config, err := s.Config(m)
	if err != nil {
		return err
	}
	fmt.Print(mode.FormatModeConfig(config))
	return nil
}
```

**Step 3: Add command to main.go**

Add `rootCmd.AddCommand(modeCmd)` after the `pagingCmd` line in `cmd/apex/main.go`.

**Step 4: Run build + tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build ./cmd/apex/ && go test ./internal/mode/ -v -count=1`
Expected: BUILD OK, PASS

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/mode/format.go cmd/apex/mode.go cmd/apex/main.go
git commit -m "feat(mode): add format functions and CLI for mode list/select/config

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: E2E Tests (3 tests)

**Files:**
- Create: `e2e/mode_test.go`

**Step 1: Write E2E tests**

```go
package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModeList(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("mode", "list")

	assert.Equal(t, 0, exitCode,
		"apex mode list should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "NORMAL"),
		"stdout should contain NORMAL, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "URGENT"),
		"stdout should contain URGENT, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "EXPLORATORY"),
		"stdout should contain EXPLORATORY, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "BATCH"),
		"stdout should contain BATCH, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "LONG_RUNNING"),
		"stdout should contain LONG_RUNNING, got: %s", stdout)
}

func TestModeSelect(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("mode", "select", "URGENT")

	assert.Equal(t, 0, exitCode,
		"apex mode select URGENT should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "URGENT"),
		"stdout should contain URGENT, got: %s", stdout)
}

func TestModeConfig(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("mode", "config", "BATCH")

	assert.Equal(t, 0, exitCode,
		"apex mode config BATCH should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "BATCH"),
		"stdout should contain BATCH, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "8"),
		"stdout should contain concurrency 8, got: %s", stdout)
}
```

**Step 2: Build and run E2E tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./e2e/ -run TestMode -v -count=1`
Expected: PASS (3 tests)

**Step 3: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add e2e/mode_test.go
git commit -m "test(e2e): add E2E tests for mode list, select, and config

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Update PROGRESS.md

**Files:**
- Modify: `PROGRESS.md`

**Step 1: Update completed phases table**

Add row: `| 37 | Mode Selector | \`2026-02-20-phase37-mode-selector-design.md\` | Done |`

**Step 2: Update Current section**

Change "Phase 37 — TBD" → "Phase 38 — TBD"

**Step 3: Update test counts**

- Unit tests: 42 → 43 packages
- E2E tests: 108 → 111 tests

**Step 4: Add Key Package**

Add: `| \`internal/mode\` | Execution mode selector with 5 modes (NORMAL/URGENT/EXPLORATORY/BATCH/LONG_RUNNING) and complexity-based auto-selection |`

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add PROGRESS.md
git commit -m "docs: mark Phase 37 Mode Selector as complete

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

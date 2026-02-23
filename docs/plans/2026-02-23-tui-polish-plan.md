# TUI Polish Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Upgrade the Apex REPL TUI from functional-but-plain to visually polished, matching 2026 AI CLI standards with themed response framing, enhanced spinner, grouped help, status line, and a theme system.

**Architecture:** Add a `Theme` struct to `style.go` that holds all color constants; all existing style vars become derived from the active theme. Modify `spinner.go` to show elapsed time. Frame response output in `stream.go` with title + metadata footer. Add `group` field to `slashCmd` for /help grouping. Print a status line before each prompt in `interactive.go`.

**Tech Stack:** Go, lipgloss (theming + styling), glamour (markdown), chzyer/readline

---

### Task 1: Theme System in style.go

**Files:**
- Modify: `cmd/apex/style.go`

**Step 1: Write the new style.go with theme system**

Replace the entire `style.go` with a theme-based approach. Key changes:
- Define `Theme` struct with all color fields
- Create `darkTheme` and `lightTheme` presets
- `activeTheme` global variable, auto-detected via `lipgloss.HasDarkBackground()`
- All style variables become functions or are re-initialized when theme changes
- `SetTheme(name string)` function to switch themes
- Keep `separator()` and `renderRisk()` functions

```go
package main

import "github.com/charmbracelet/lipgloss"

// Theme defines all colors used in the TUI.
type Theme struct {
	Name    string
	Accent  lipgloss.Color
	Text    lipgloss.Color
	Dim     lipgloss.Color
	Meta    lipgloss.Color
	Success lipgloss.Color
	Error   lipgloss.Color
	Warning lipgloss.Color
	Info    lipgloss.Color
	Border  lipgloss.Color
}

var darkTheme = Theme{
	Name:    "dark",
	Accent:  lipgloss.Color("99"),
	Text:    lipgloss.Color("252"),
	Dim:     lipgloss.Color("242"),
	Meta:    lipgloss.Color("242"),
	Success: lipgloss.Color("10"),
	Error:   lipgloss.Color("9"),
	Warning: lipgloss.Color("11"),
	Info:    lipgloss.Color("245"),
	Border:  lipgloss.Color("237"),
}

var lightTheme = Theme{
	Name:    "light",
	Accent:  lipgloss.Color("55"),
	Text:    lipgloss.Color("235"),
	Dim:     lipgloss.Color("245"),
	Meta:    lipgloss.Color("245"),
	Success: lipgloss.Color("28"),
	Error:   lipgloss.Color("160"),
	Warning: lipgloss.Color("172"),
	Info:    lipgloss.Color("240"),
	Border:  lipgloss.Color("250"),
}

var activeTheme = darkTheme

func initTheme() {
	if !lipgloss.HasDarkBackground() {
		activeTheme = lightTheme
	}
}

// SetTheme switches the active theme by name. Returns false if name is unknown.
func SetTheme(name string) bool {
	switch name {
	case "dark":
		activeTheme = darkTheme
	case "light":
		activeTheme = lightTheme
	default:
		return false
	}
	refreshStyles()
	return true
}

// --- Derived styles (refreshed on theme change) ---

var (
	styleBannerTitle lipgloss.Style
	styleBannerInfo  lipgloss.Style
	stylePrompt      lipgloss.Style
	styleSuccess     lipgloss.Style
	styleError       lipgloss.Style
	styleSpinner     lipgloss.Style
	styleInfo        lipgloss.Style
	styleDim         lipgloss.Style
	styleMeta        lipgloss.Style
	styleStepBorder  lipgloss.Style
	styleStepName    lipgloss.Style
	styleRespTitle   lipgloss.Style
	styleRespRule    lipgloss.Style
)

func init() {
	initTheme()
	refreshStyles()
}

func refreshStyles() {
	t := activeTheme
	styleBannerTitle = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	styleBannerInfo = lipgloss.NewStyle().Foreground(t.Dim)
	stylePrompt = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	styleSuccess = lipgloss.NewStyle().Foreground(t.Success)
	styleError = lipgloss.NewStyle().Foreground(t.Error)
	styleSpinner = lipgloss.NewStyle().Foreground(t.Accent)
	styleInfo = lipgloss.NewStyle().Foreground(t.Info)
	styleDim = lipgloss.NewStyle().Foreground(t.Dim)
	styleMeta = lipgloss.NewStyle().Foreground(t.Meta)
	styleStepBorder = lipgloss.NewStyle().Foreground(t.Border)
	styleStepName = lipgloss.NewStyle().Foreground(t.Text)
	styleRespTitle = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	styleRespRule = lipgloss.NewStyle().Foreground(t.Border)
}

var styleRisk = map[string]lipgloss.Style{
	"LOW":      lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
	"MEDIUM":   lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
	"HIGH":     lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	"CRITICAL": lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1")),
}

func separator() string {
	return styleDim.Render("  ─────")
}

func renderRisk(level string) string {
	if s, ok := styleRisk[level]; ok {
		return s.Render(level)
	}
	return level
}

// responseHeader renders "  ◆ Response ────────────────"
func responseHeader() string {
	title := styleRespTitle.Render("◆ Response")
	rule := styleRespRule.Render(" ─────────────────────────────")
	return "  " + title + rule
}

// errorHeader renders "  ✗ Error ──────────────────"
func errorHeader() string {
	title := styleError.Render("✗ Error")
	rule := styleRespRule.Render(" ────────────────────────────────")
	return "  " + title + rule
}
```

**Step 2: Build and verify**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build ./cmd/apex/`
Expected: Build succeeds

**Step 3: Run existing tests**

Run: `go test ./cmd/apex/ -v -run "TestRenderRisk"`
Expected: PASS

**Step 4: Commit**

```bash
git add cmd/apex/style.go
git commit -m "feat(tui): add theme system with dark/light presets"
```

---

### Task 2: Enhanced Spinner with Elapsed Time

**Files:**
- Modify: `cmd/apex/spinner.go`
- Modify: `cmd/apex/stream.go`

**Step 1: Modify Spinner to show elapsed time and detail**

Update the `Spinner` struct to track start time and display elapsed seconds:

```go
package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

var brailleFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type Spinner struct {
	mu      sync.Mutex
	message string
	detail  string
	start   time.Time
	stop    chan struct{}
	done    chan struct{}
}

func NewSpinner(message string) *Spinner {
	s := &Spinner{
		message: message,
		start:   time.Now(),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	go s.run()
	return s
}

func NewSpinnerWithDetail(message, detail string) *Spinner {
	s := &Spinner{
		message: message,
		detail:  detail,
		start:   time.Now(),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	go s.run()
	return s
}

func (s *Spinner) Update(message string) {
	s.mu.Lock()
	s.message = message
	s.mu.Unlock()
}

func (s *Spinner) Stop() {
	close(s.stop)
	<-s.done
}

func (s *Spinner) run() {
	defer close(s.done)
	tick := time.NewTicker(80 * time.Millisecond)
	defer tick.Stop()

	frame := 0
	for {
		select {
		case <-s.stop:
			fmt.Printf("\r%s\r", strings.Repeat(" ", 100))
			return
		case <-tick.C:
			s.mu.Lock()
			msg := s.message
			detail := s.detail
			s.mu.Unlock()
			elapsed := time.Since(s.start).Seconds()
			suffix := ""
			if detail != "" {
				suffix = styleDim.Render(fmt.Sprintf(" (%s · %.1fs)", detail, elapsed))
			} else {
				suffix = styleDim.Render(fmt.Sprintf(" (%.1fs)", elapsed))
			}
			fmt.Printf("\r  %s %s%s", styleSpinner.Render(brailleFrames[frame]), styleDim.Render(msg), suffix)
			frame = (frame + 1) % len(brailleFrames)
		}
	}
}
```

**Step 2: Update stream.go to pass model name to spinners**

In `runSimpleTask`:
```go
spin := NewSpinnerWithDetail("Thinking...", cfg.Claude.Model)
```

In `runComplexTask`:
```go
spin := NewSpinnerWithDetail("Planning...", cfg.Planner.Model)
```

In `displayStepProgress` step spinners:
```go
spin := NewSpinner(fmt.Sprintf("Running step %d/%d...", stepIndex[n.ID], total))
```
(Step spinners keep no detail — that's fine, elapsed time is shown.)

**Step 3: Build and run tests**

Run: `go build ./cmd/apex/ && go test ./cmd/apex/ -v -run "TestSession|TestRender|TestCmd"`
Expected: All pass

**Step 4: Commit**

```bash
git add cmd/apex/spinner.go cmd/apex/stream.go
git commit -m "feat(tui): spinner with elapsed time and model detail"
```

---

### Task 3: Response Framing in stream.go

**Files:**
- Modify: `cmd/apex/stream.go`

**Step 1: Add response framing to runSimpleTask**

Replace the output section in `runSimpleTask` (after `spin.Stop()`):

```go
// Error case:
if err != nil {
    fmt.Println(errorHeader())
    fmt.Println(styleError.Render(fmt.Sprintf("  %s", err.Error())))
    fmt.Println()
    fmt.Println(separator())
    fmt.Println(styleMeta.Render(fmt.Sprintf("  ✗ %.1fs · %s · %s", duration.Seconds(), cfg.Claude.Model, renderRisk(riskLevel))))
    return result.Output, err
}

// Success case:
fmt.Println(responseHeader())
fmt.Println()
fmt.Println(renderMarkdown(result.Output))
fmt.Println()
fmt.Println(separator())
fmt.Println(styleMeta.Render(fmt.Sprintf("  ✓ %.1fs · %s · %s", duration.Seconds(), cfg.Claude.Model, renderRisk(riskLevel))))
```

**Step 2: Add response framing to runComplexTask footer**

The complex task footer already has a separator + metadata line. Update to use the same pattern:

```go
// After execution, before the footer:
fmt.Println()
fmt.Println(separator())
if execErr != nil {
    fmt.Println(styleMeta.Render(fmt.Sprintf("  ✗ %d/%d steps · %.1fs · %s · %s",
        completed, total, duration.Seconds(), cfg.Claude.Model, renderRisk(riskLevel))))
    return d.Summary(), execErr
}
fmt.Println(styleMeta.Render(fmt.Sprintf("  ✓ %d/%d steps · %.1fs · %s · %s",
    completed, total, duration.Seconds(), cfg.Claude.Model, renderRisk(riskLevel))))
```

**Step 3: Build and test**

Run: `go build ./cmd/apex/ && go test ./cmd/apex/`
Expected: All pass

**Step 4: Commit**

```bash
git add cmd/apex/stream.go
git commit -m "feat(tui): response framing with header and metadata footer"
```

---

### Task 4: Grouped /help + /theme Command

**Files:**
- Modify: `cmd/apex/slash.go`

**Step 1: Add `group` field to `slashCmd` and update all registrations**

Add `group string` to the struct:
```go
type slashCmd struct {
    name    string
    aliases []string
    group   string
    desc    string
    handler func(s *session, args string, rl *readline.Instance) bool
}
```

Update all registrations in `init()` to include the group. Use these groups:
- `"Session"` — help, new, compact, copy, export, clear
- `"Config"` — model, permissions, mode, config, theme
- `"Context"` — mention, context, diff
- `"Memory"` — memory, kg
- `"Execution"` — plan, review, trace, metrics
- `"System"` — status, history, doctor, snapshot, plugin, gc
- `"Utility"` — version, quit

Add a new `/theme` command after `config`:
```go
{name: "theme", group: "Config", desc: "Switch theme (dark/light)", handler: cmdTheme},
```

**Step 2: Rewrite `cmdHelp` to group by category**

```go
func cmdHelp(s *session, args string, rl *readline.Instance) bool {
    fmt.Println()
    lastGroup := ""
    for _, c := range slashCommands {
        if c.group != lastGroup {
            if lastGroup != "" {
                fmt.Println()
            }
            fmt.Println(styleDim.Render("  " + c.group))
            lastGroup = c.group
        }
        fmt.Printf("    %-16s %s\n", stylePrompt.Render("/"+c.name), c.desc)
    }
    fmt.Println()
    fmt.Println(styleDim.Render("  Tip: !<cmd> runs a shell command · Tab for autocomplete"))
    fmt.Println()
    return false
}
```

**Step 3: Add `cmdTheme` handler**

```go
func cmdTheme(s *session, args string, rl *readline.Instance) bool {
    name := strings.TrimSpace(args)
    if name == "" {
        fmt.Printf("  Current theme: %s\n", activeTheme.Name)
        fmt.Println(styleDim.Render("  Available: dark, light"))
        fmt.Println()
        return false
    }
    if !SetTheme(name) {
        fmt.Println(styleError.Render("  Unknown theme: " + name))
        fmt.Println(styleDim.Render("  Available: dark, light"))
        fmt.Println()
        return false
    }
    fmt.Println(styleSuccess.Render("  Theme set to " + name))
    fmt.Println()
    return false
}
```

**Step 4: Update `buildCompleter` — it already generates from registry, so `/theme` will be auto-included.**

**Step 5: Build and test**

Run: `go build ./cmd/apex/ && go test ./cmd/apex/ -v -run "TestFind|TestBuild"`
Expected: All pass. Verify that `findCommand("theme")` would work (existing test structure covers this).

**Step 6: Commit**

```bash
git add cmd/apex/slash.go
git commit -m "feat(tui): grouped /help display and /theme command"
```

---

### Task 5: Status Line + Simplified Banner

**Files:**
- Modify: `cmd/apex/interactive.go`

**Step 1: Simplify `printBanner`**

```go
func printBanner(cfg *config.Config) {
    cwd, _ := os.Getwd()
    fmt.Println()
    fmt.Println(styleBannerTitle.Render("  ◆ Apex v0.1.0"))
    fmt.Println(styleBannerInfo.Render(fmt.Sprintf("  %s · %s · %s", cfg.Claude.Model, cfg.Sandbox.Level, cwd)))
    fmt.Println()
    fmt.Println(styleDim.Render("  /help for commands · /quit to exit · Tab for autocomplete"))
    fmt.Println()
}
```

Remove the `styleBannerBox` variable from `style.go` (it's no longer used — it was removed in Task 1 already since the new style.go doesn't include it).

**Step 2: Add `printStatusLine` function**

```go
func (s *session) printStatusLine() {
    ctx := s.context()
    ctxLen := len(ctx)
    var ctxStr string
    if ctxLen > 1000 {
        ctxStr = fmt.Sprintf("%.1fk chars", float64(ctxLen)/1000)
    } else {
        ctxStr = fmt.Sprintf("%d chars", ctxLen)
    }
    line := fmt.Sprintf("  %s · context: %s · %d turns · %s",
        s.cfg.Claude.Model, ctxStr, len(s.turns), s.cfg.Sandbox.Level)
    fmt.Println(styleDim.Render(line))
}
```

**Step 3: Call `printStatusLine` before the prompt in the REPL loop**

In `startInteractive`, just before `rl.Readline()`, add:

```go
    for {
        if len(s.turns) > 0 {
            s.printStatusLine()
        }
        line, err := rl.Readline()
```

Only show after at least one turn (not on first prompt — banner already shows model info).

**Step 4: Build and test**

Run: `go build ./cmd/apex/ && go test ./cmd/apex/`
Expected: All pass

**Step 5: Commit**

```bash
git add cmd/apex/interactive.go
git commit -m "feat(tui): status line before prompt and simplified banner"
```

---

### Task 6: Full Test Suite + Manual Smoke Test

**Step 1: Run all tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./...`
Expected: All packages pass

**Step 2: Build binary**

Run: `go build -o /tmp/apex_dev ./cmd/apex/`
Expected: Build OK

**Step 3: Commit any fixups if needed**

Only if tests revealed issues.

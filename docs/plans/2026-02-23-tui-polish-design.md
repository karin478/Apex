# TUI Polish Design

Date: 2026-02-23

## Background

Apex REPL TUI is functional but visually plain compared to 2026 state-of-the-art AI CLI tools (Claude Code, Cursor CLI, Gemini CLI, Crush). Key gaps: no response framing, basic spinner, flat /help list, no status context, boxed banner, no theming.

## Design: 6 Modules

### Module A: Response Format Upgrade

Wrap responses in a visual frame with title + metadata footer:

```
  ◆ Response ─────────────────────────────

  [glamour markdown rendered content]

  ─────
  ✓ 2.3s · claude-opus-4-6 · LOW · 1.2k tokens
```

- Title line: `◆ Response` in theme accent color + dim horizontal rule
- Content: glamour-rendered markdown (unchanged)
- Footer: status icon + duration + model + risk + token count
- Error responses: `✗ Error` with red title line
- Token count: parse from executor result (add `Tokens int` to `executor.Result`)

### Module B: Enhanced Spinner

Show model name and live elapsed timer:

```
  ⠹ Thinking... (claude-opus-4-6 · 3.2s)
```

- Add elapsed time counter (updates every tick alongside frame)
- Show model name in parentheses
- Planning: `⠹ Planning... (claude-opus-4-6 · 1.5s)`

Implementation: modify `Spinner` to accept and display a `detail` string, pass `time.Since(start)` from the caller.

### Module C: /help Grouped Display

Group commands by category with dim section headers:

```
  Session
    /new          Start fresh session
    /compact      Compress context
    ...

  Config
    /model        View/switch model
    ...
```

Implementation: add `group string` field to `slashCmd` struct, `cmdHelp` iterates and prints group headers when group changes.

### Module D: Status Line Before Prompt

Print a dim status line before each prompt:

```
  claude-opus-4-6 · context: 2.1k chars · 3 turns · ulimit
  ❯
```

Implementation: in the REPL loop, before `rl.Readline()`, print a `styleDim` status line using session state. No persistent status bar needed (avoids bubbletea dependency).

### Module E: Banner Simplification

Remove the lipgloss box border, go minimal:

```
  ◆ Apex v0.1.0
  claude-opus-4-6 · ulimit · /Users/lyndonlyu/project

  /help for commands · /quit to exit · Tab for autocomplete
```

- Title: bold + accent color, no border box
- Info: dim gray
- Hint: dim gray with key actions

### Module F: Theme System

Define `Theme` struct with all color constants. Two presets: dark (default), light.

```go
type Theme struct {
    Accent    lipgloss.Color
    Text      lipgloss.Color
    Dim       lipgloss.Color
    Success   lipgloss.Color
    Error     lipgloss.Color
    Warning   lipgloss.Color
    // ...
}
```

- `/theme [dark|light]` command to switch
- Auto-detect with `lipgloss.HasDarkBackground()`
- All existing style vars become functions: `accentStyle()` reads from current theme

## Files Affected

- `cmd/apex/style.go` — theme system, all style definitions become theme-aware
- `cmd/apex/interactive.go` — status line before prompt, simplified banner
- `cmd/apex/stream.go` — response framing, enhanced spinner detail, token display
- `cmd/apex/spinner.go` — add elapsed time + detail string
- `cmd/apex/slash.go` — grouped /help, /theme command, group field on slashCmd
- `internal/executor/claude.go` — add Tokens field to Result (parse from claude output)

## Not Doing (YAGNI)

- Full-screen bubbletea rewrite
- Animated ASCII banner
- Split-pane diff view
- Mouse support
- Image rendering

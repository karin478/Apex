# CLI UX Upgrade — Claude Code-like Experience

**Date:** 2026-02-23
**Goal:** Transform the current bare-bones REPL into a polished Claude Code-like CLI experience.

**Approach:** Pure ANSI/lipgloss on the existing bufio.Scanner loop. No bubbletea TUI framework — keeps the architecture simple and avoids a full rewrite.

---

## 1. Banner & Prompt

Startup shows a lipgloss rounded box with project info:

```
╭─────────────────────────────────────────╮
│  ◆ Apex v0.1.0                          │
│  claude-opus-4-6 · ulimit · /Users/hank │
╰─────────────────────────────────────────╯
  /help for commands · /quit to exit

❯
```

- Prompt: `❯` in purple bold (lipgloss Color "99")
- User input: default terminal color (no styling)
- After each response: blank line then `❯` again

## 2. Spinner / Thinking Animation

After user sends input, immediately show a spinner on the same line:

```
❯ hi
⠋ Thinking...
```

Implementation:
- Goroutine ticks every 80ms cycling through braille frames: `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`
- Uses `\r` (carriage return) to overwrite the spinner line in-place
- When task completes, clear the spinner line with `\r` + spaces + `\r`
- Spinner text: "Thinking..." for simple tasks, "Planning..." then "Running step N/M..." for complex

## 3. Response Display

Simple task response:

```
❯ hi

  Hello! I'm Apex. How can I help you today?

  ─────
  2.3s · claude-opus-4-6 · LOW

❯
```

- Response body: glamour markdown rendering (already implemented)
- Separator: thin `─────` line in dim gray
- Metadata line: duration · model · risk level, all in dim gray
- Blank lines before and after for breathing room

## 4. Complex Task Progress

Multi-step DAG tasks show per-step progress:

```
❯ refactor auth then update tests

  Planning... 3 steps

  ┌ [1/3] Refactor auth module
  │ ⠋ Running...
  └ ✓ Done (12.1s)

  ┌ [2/3] Update unit tests
  │ ⠋ Running...
  └ ✓ Done (6.4s)

  ┌ [3/3] Update integration tests
  │ ⠋ Running...
  └ ✗ Failed (3.2s)

  ─────
  ✗ 2/3 steps completed · 21.7s · MEDIUM

❯
```

- Box-drawing chars (┌│└) for step framing
- Spinner on active step, ✓/✗ on completed
- Step name from DAG node task description
- Final summary line with pass/fail count

## 5. Metadata Footer (replaces status bar)

No persistent bottom status bar (would need bubbletea). Instead, every response ends with a dim metadata line after a separator:

```
  ─────
  2.3s · claude-opus-4-6 · LOW
```

For complex tasks:
```
  ─────
  ✓ 3/3 steps · 26.7s · claude-opus-4-6 · MEDIUM
```

## Files to Modify

| File | Change |
|------|--------|
| `cmd/apex/style.go` | New styles: box border, purple prompt, separator, step frame |
| `cmd/apex/interactive.go` | New banner, purple `❯` prompt, response formatting |
| `cmd/apex/stream.go` | Spinner goroutine, step progress display, metadata footer |
| `cmd/apex/spinner.go` | New file — reusable spinner component |

## Dependencies

No new dependencies. Uses existing lipgloss + glamour.

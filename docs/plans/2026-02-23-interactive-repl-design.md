# Interactive REPL Mode Design

**Date**: 2026-02-23
**Approach**: Hybrid — Simple REPL + Colored Output
**Dependencies**: go-prompt, lipgloss

---

## Overview

Add an interactive chat-style REPL to Apex, invoked by running `apex` with no arguments. Users type tasks in natural language, see real-time streaming output with colored formatting, and can issue slash commands for system operations. Session context accumulates across turns.

---

## Entry Point

```bash
# Interactive mode (no args)
$ apex

# One-shot mode (unchanged)
$ apex run "refactor auth"
```

When `apex` is invoked with no subcommand and no arguments, it enters interactive REPL mode. All existing commands (`apex run`, `apex status`, etc.) remain unchanged.

---

## Session Model

Each REPL session maintains a list of turns (task + result summary). This context is injected into subsequent planner prompts so the LLM can resolve references like "them", "the previous result", etc.

```go
type Session struct {
    Turns []Turn
}

type Turn struct {
    Task    string
    Summary string // truncated result for context budget
}
```

On exit, the session is saved to memory via the existing memory store.

---

## User Interface

```
apex v0.1.0 · claude-sonnet-4 · ulimit
Type a task, /help for commands, /quit to exit

apex> analyze the error handling patterns
[LOW] Planning... 1 step
● Analyzing error handling... ✓ (8.2s)

→ Found 3 patterns: sentinel errors in...

apex> now refactor them to use fmt.Errorf
[MEDIUM] Confirm? (y/n): y
Planning... 2 steps
● [1/2] Refactoring handlers... ✓ (12.1s)
● [2/2] Updating tests... ✓ (6.4s)
✓ Done (18.5s, 2 steps)

apex> /status
Recent runs: ...

apex> /quit
Session saved.
```

---

## Slash Commands

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/status` | Show recent run history |
| `/history` | Show task execution history |
| `/doctor` | Run system integrity check |
| `/clear` | Clear screen |
| `/config` | Show current config summary |
| `/quit` | Exit session (also `/exit`) |

Input not starting with `/` is treated as a task and goes through the full pipeline: risk classification → planning → DAG execution → audit.

---

## Dependencies

| Library | Purpose | Size |
|---------|---------|------|
| `github.com/c-bata/go-prompt` | Readline with autocomplete, history, key bindings | Lightweight |
| `github.com/charmbracelet/lipgloss` | Terminal styling (colors, borders, badges) | Lightweight |

No TUI framework (bubbletea) needed. These two libraries provide readline input and colored output without taking over the terminal.

---

## Streaming Output

The executor currently buffers full output before returning. For interactive mode, we add an optional streaming callback:

```go
type Options struct {
    // ... existing fields ...
    OnOutput func(chunk string) // nil = buffer mode (default)
}
```

Implementation: replace `cmd.Stdout = &buf` with `io.TeeReader` + line scanner goroutine when `OnOutput` is set. When nil, behavior is identical to current implementation (full backward compatibility).

---

## File Structure

New files:

```
cmd/apex/
├── interactive.go     # REPL loop, input handling, slash command dispatch
├── style.go           # lipgloss style definitions
└── stream.go          # streaming executor wrapper for REPL

internal/executor/
└── claude.go          # add OnOutput field to Options (backward-compatible)
```

---

## What Does NOT Change

- `apex run "task"` one-shot mode
- All internal packages (except executor.Options gains one optional field)
- Configuration file format
- Audit, governance, DAG, memory, snapshot logic
- Test framework and existing tests
- CLI flag structure for all existing commands

---

## Design Principles

1. **Additive only** — No existing behavior changes. REPL is a new entry point.
2. **Minimal dependencies** — Two small libraries, no framework.
3. **Session context** — Previous turns inform subsequent tasks naturally.
4. **Streaming** — Real-time output in REPL, buffered in one-shot mode.
5. **Slash commands** — System operations without LLM overhead.

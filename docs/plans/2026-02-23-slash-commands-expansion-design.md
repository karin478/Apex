# Slash Commands Expansion Design

Date: 2026-02-23

## Background

Apex REPL currently has only 7 slash commands (`/help`, `/status`, `/history`, `/doctor`, `/clear`, `/config`, `/quit`). Claude Code has ~40 and Codex CLI has ~30. Meanwhile Apex has 40+ cobra subcommands and 59 internal packages with functionality that is not exposed in the interactive REPL.

## Design: 22 New Slash Commands

### Group A: Session Control (4 new)

| Command | Description | Implementation |
|---------|-------------|----------------|
| `/new` | Reset session, start fresh without exiting | Clear `session.turns`, reprint banner |
| `/compact` | Compress context to free token budget | Summarize old turns into 1 line each, keep last 2 full |
| `/copy` | Copy last response to clipboard | `pbcopy` on macOS, `xclip` on Linux |
| `/export` | Export session transcript to file | Write turns as markdown to `~/.apex/exports/` |

### Group B: Runtime Config (3 new)

| Command | Description | Implementation |
|---------|-------------|----------------|
| `/model [name]` | View or switch Claude model mid-session | Modify `session.cfg.Claude.Model`, show current if no arg |
| `/permissions` | View or switch permission mode | Modify `session.cfg.Claude.PermissionMode` |
| `/mode [name]` | View or switch execution mode | Bridge to `internal/mode` |

### Group C: Context & Files (3 new)

| Command | Description | Implementation |
|---------|-------------|----------------|
| `/mention <path>` | Attach file content to next task context | Read file, store in `session.attachments` |
| `/context` | Show current context size and turn count | Display turns count, char count, attachment count |
| `/diff` | Show git diff of working directory | Run `git diff --stat` + `git diff` |

### Group D: Memory & Knowledge (3 new)

| Command | Description | Implementation |
|---------|-------------|----------------|
| `/memory [query]` | Search memory or show recent entries | Bridge to `internal/memory` + `internal/search` |
| `/memory clear` | Clear all session context and memory | Reset turns + clear staged memory |
| `/kg [query]` | Query knowledge graph | Bridge to `internal/kg` |

### Group E: Execution Tools (4 new)

| Command | Description | Implementation |
|---------|-------------|----------------|
| `/plan <task>` | Plan-only mode: show steps without executing | Bridge to `internal/planner` |
| `/review` | Review current git changes | Bridge to cobra `review` command |
| `/trace` | Show last execution trace | Bridge to cobra `trace` command |
| `/metrics` | Show execution metrics | Bridge to cobra `metrics` command |

### Group F: System Management (3 new)

| Command | Description | Implementation |
|---------|-------------|----------------|
| `/snapshot` | List or restore execution snapshots | Bridge to cobra `snapshot` commands |
| `/plugin` | List loaded plugins and status | Bridge to cobra `plugin list` |
| `/gc` | Clean up old runs/audit/snapshots | Bridge to cobra `gc` command |

### Group G: Utility (2 new)

| Command | Description | Implementation |
|---------|-------------|----------------|
| `/version` | Show version info | Print version string |
| `!<cmd>` | Execute shell command directly | `exec.Command("sh", "-c", cmd)` |

### Keep Existing (8)

`/help`, `/status`, `/history`, `/doctor`, `/clear`, `/config`, `/quit`, `/exit`

## Architecture

### Command Registry Pattern

Replace the monolithic `handleSlash` switch with a command registry:

```go
type slashCmd struct {
    name    string
    aliases []string
    desc    string
    handler func(s *session, args string, rl *readline.Instance) bool
}

var slashCommands []slashCmd
```

Benefits:
- `/help` auto-generates from registry (no duplicate list)
- Autocomplete auto-generates from registry
- Easy to add new commands without touching routing logic

### session struct additions

```go
type session struct {
    cfg         *config.Config
    turns       []turn
    lastOutput  string       // for /copy
    attachments []string     // for /mention
    home        string       // cached home dir
}
```

### File Structure

All new command handlers go in a single new file `cmd/apex/slash.go` to avoid bloating `interactive.go`. The interactive.go only keeps the REPL loop, session struct, banner, and save logic.

## Autocomplete

The readline PrefixCompleter will be auto-generated from the command registry. Commands with subcommands (like `/memory clear`) will have nested completers.

## Total Command Count After

- Existing: 8
- New: 22
- Shell escape: `!` (special syntax)
- **Total: 30 commands** (on par with Codex CLI)

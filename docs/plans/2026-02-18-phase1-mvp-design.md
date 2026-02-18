# Apex Agent — Phase 1 MVP Design

**Date**: 2026-02-18
**Status**: Approved
**Author**: Lyndon + Claude Opus 4.6

## Overview

Phase 1 MVP implements the minimum viable core loop of Apex Agent: a CLI tool that takes a user task, classifies risk, executes via Claude Code CLI, persists memory to the filesystem, and logs an audit trail.

## Decisions Made

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go | POSIX syscall support, single binary, SQLite ecosystem |
| Project structure | Single module + `internal/` packages | Simple for Phase 1, easy to refactor later |
| Claude invocation | `claude` CLI (`-p` mode) | Leverages existing install, no extra API key |
| Config directory | `~/.apex/` | Avoids collision with `~/.claude/` |
| Development method | Strict TDD (all phases) | Red → Green → Refactor |
| Timeout policy | Generous defaults to ensure task completion | 600s default, 1800s for long tasks |

## Architecture

```
apex (CLI entry)
  │
  ├── apex run "task description"
  │     ├── 1. Config: load ~/.apex/config.yaml
  │     ├── 2. Governance: keyword-based risk classification
  │     ├── 3. MEDIUM → terminal confirm (y/n)
  │     │   HIGH/CRITICAL → reject with message
  │     ├── 4. Executor: claude -p "..." --model claude-opus-4-6 --effort high
  │     ├── 5. Memory: persist result + decisions to filesystem
  │     └── 6. Audit: append JSONL log entry
  │
  ├── apex memory search "keyword"
  │     └── grep-based search across memory files
  │
  └── apex history
        └── display recent audit log entries
```

## Data Flow

```
User command → Config load → Governance(risk classify)
  → [MEDIUM needs confirm] → Executor(claude CLI)
  → Memory(file write) → Audit(JSONL append)
```

## Component Design

### 1. CLI Entry (`cmd/apex/`)

- Built with `cobra` (standard Go CLI framework)
- Subcommands: `run`, `memory search`, `history`, `version`
- Global flags: `--config`, `--verbose`, `--dry-run`

### 2. Governance (`internal/governance/`)

```
Input:  task description string
Output: RiskLevel (LOW / MEDIUM / HIGH / CRITICAL)

Classification: keyword matching (Phase 1, no LLM)

HIGH/CRITICAL keywords: delete, drop, deploy, production, migrate, rm -rf,
                        密钥, 生产, 删除, 部署
MEDIUM keywords: write, modify, install, update, config, create,
                 修改, 安装, 配置, 创建
Default: LOW

Actions by level:
  LOW      → auto-execute
  MEDIUM   → terminal prompt "⚠ [MEDIUM risk] Proceed? (y/n)"
  HIGH     → reject: "❌ HIGH risk task. Break it into smaller steps."
  CRITICAL → reject: "🚫 CRITICAL risk. Not supported in Phase 1."
```

### 3. Executor (`internal/executor/`)

```
Invocation:
  claude -p "<system_prompt>\n\n<task>" \
    --model claude-opus-4-6 \
    --effort high \
    --output-format json

Timeout: config.claude.timeout (default 600s)
Long task timeout: config.claude.long_task_timeout (default 1800s)

Error handling:
  - Non-zero exit → log to audit as failure
  - Timeout → kill process, log as timeout
  - Capture both stdout and stderr
```

### 4. Memory (`internal/memory/`)

```
~/.apex/memory/
├── decisions/     # {timestamp}-{slug}.md
├── facts/         # {timestamp}-{slug}.md
└── sessions/      # {session_id}.jsonl

Write: after each successful run, extract key info and persist
Search: file name + content grep (simple keyword matching)
Format: Markdown files with YAML frontmatter
```

Example memory file:
```markdown
---
type: decision
created: 2026-02-18T18:30:00Z
task: "refactor auth module"
confidence: 0.9
---

# Auth Module Refactoring Decision

Chose JWT over session-based auth because...
```

### 5. Audit (`internal/audit/`)

```
Path: ~/.apex/audit/{date}.jsonl
Mode: append-only

Record schema:
{
  "timestamp": "2026-02-18T18:30:00Z",
  "action_id": "uuid",
  "task": "task description",
  "risk_level": "LOW",
  "outcome": "success|failure|timeout|rejected",
  "duration_ms": 1234,
  "model": "claude-opus-4-6",
  "error": null
}
```

### 6. Config (`internal/config/`)

```yaml
# ~/.apex/config.yaml
claude:
  model: claude-opus-4-6
  effort: high
  timeout: 600
  long_task_timeout: 1800
governance:
  auto_approve: [LOW]
  confirm: [MEDIUM]
  reject: [HIGH, CRITICAL]
memory:
  dir: ~/.apex/memory
audit:
  dir: ~/.apex/audit
```

## Filesystem Layout

```
~/.apex/
├── config.yaml
├── memory/
│   ├── decisions/
│   ├── facts/
│   └── sessions/
└── audit/
    └── {date}.jsonl
```

## Project Structure

```
ai_agent_cli_project/
├── cmd/
│   └── apex/
│       └── main.go
├── internal/
│   ├── governance/
│   │   ├── risk.go
│   │   └── risk_test.go
│   ├── executor/
│   │   ├── claude.go
│   │   └── claude_test.go
│   ├── memory/
│   │   ├── store.go
│   │   └── store_test.go
│   ├── audit/
│   │   ├── logger.go
│   │   └── logger_test.go
│   └── config/
│       ├── config.go
│       └── config_test.go
├── docs/
│   └── plans/
├── go.mod
├── go.sum
├── Makefile
└── architecture-design-v11_0.md
```

## Non-Functional Requirements (Phase 1)

| Requirement | Target |
|-------------|--------|
| Build time | < 10s |
| Binary size | < 20MB |
| Cold start | < 100ms (excluding claude CLI) |
| Config load | < 10ms |
| Memory search (1k files) | < 500ms |
| Audit append | < 5ms |

## Out of Scope (Phase 1)

- DAG orchestration / multi-agent
- SQLite / runtime.db
- Snapshot / rollback
- Sandbox / isolation
- Vector search / embeddings
- Hash chain audit
- Kill Switch
- TUI dashboard
- Plugin system

# Phase 23: System Dashboard — Design Document

> Date: 2026-02-20
> Status: Approved
> Architecture Ref: v11.0 §2.9 Observability / Live Dashboard

## 1. Goal

Provide `apex dashboard` command that generates a system status overview, showing recent runs, health status, metrics summary, policy changes, and audit integrity.

## 2. New Package: `internal/dashboard`

### 2.1 Types

```go
type Section struct {
    Title   string
    Content string
}

type Dashboard struct {
    baseDir string
}
```

### 2.2 Functions

```go
func New(baseDir string) *Dashboard
func (d *Dashboard) Generate() ([]Section, error)
func FormatTerminal(sections []Section) string
func FormatMarkdown(sections []Section) string
```

### 2.3 Sections

| Section | Source Package | Content |
|---------|--------------|---------|
| System Health | `internal/health` | Current level + check details |
| Recent Runs | `internal/manifest` | Last 5 runs (ID/task/outcome/duration) |
| Metrics Summary | `internal/metrics` | Total runs, success rate, avg duration |
| Policy Changes | `internal/audit` | Recent policy change entries |
| Audit Integrity | `internal/audit` | Chain verification status |

## 3. CLI: `apex dashboard`

```
apex dashboard              # terminal output (default)
apex dashboard --format md  # markdown output
```

## 4. Non-Goals

- No real-time refresh / watch mode
- No interactive TUI (curses/bubbletea)
- No remote data fetching

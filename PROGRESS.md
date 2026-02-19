# Apex Agent CLI - Progress Tracker

> This file persists project context across sessions. Update after each phase completion.

## Project

**Name:** Apex Agent - Claude Code 长期记忆自治代理系统
**Architecture:** v11.0 (`architecture-design-v11_0.md`)
**Tech Stack:** Go, Cobra CLI, Testify
**Repo:** `/Users/lyndonlyu/Downloads/ai_agent_cli_project/`

## Completed Phases

| Phase | Name | Design Doc | Status |
|-------|------|-----------|--------|
| 1 | MVP (Executor + Pool) | `phase1-mvp-design.md` | Done |
| 2 | DAG Orchestration | `phase2-dag-design.md` | Done |
| 3 | Semantic Search (Embedding + VectorDB) | `phase3-semantic-search-design.md` | Done |
| 4 | Context Builder + Compression | `phase4-context-builder-design.md` | Done |
| 5 | Observability (Audit + Manifest) | `phase5-observability-design.md` | Done |
| 6 | Kill Switch | `phase6-kill-switch-design.md` | Done |
| 7 | Snapshot & Rollback | `phase7-snapshot-rollback-design.md` | Done |
| 8 | Human-in-the-Loop Approval | `phase8-approval-design.md` | Done |
| 9 | Dry-Run Mode | `phase9-dry-run-design.md` | Done |
| 10 | Fault Tolerance & Retry | `phase10-fault-tolerance-design.md` | Done |
| E2E | E2E Testing Module | `e2e-testing-design.md` | Done |
| 11 | Execution Sandbox | `2026-02-19-phase11-execution-sandbox-design.md` | Done |

## Current: Phase 12 — TBD

No phase in progress.

## Testing

| Suite | Command | Coverage |
|-------|---------|----------|
| Unit tests | `make test` | 21 packages |
| E2E tests (mock) | `make e2e` | 29 tests, all CLI commands |
| E2E tests (live) | `make e2e-live` | 4 smoke tests with real Claude |

## Key Packages

| Package | Purpose |
|---------|---------|
| `internal/executor` | Claude CLI wrapper |
| `internal/pool` | Agent pool + concurrency |
| `internal/dag` | DAG orchestration + state machine |
| `internal/planner` | Task decomposition |
| `internal/embedding` | OpenAI embedding client |
| `internal/vectordb` | Local vector similarity search |
| `internal/context` | Context builder + token compression |
| `internal/search` | Semantic search engine |
| `internal/audit` | Structured audit logging |
| `internal/manifest` | Execution manifest tracking |
| `internal/killswitch` | Emergency stop via file signal |
| `internal/snapshot` | Git-stash-based rollback |
| `internal/governance` | Risk classification |
| `internal/approval` | Human-in-the-loop approval gate |
| `internal/cost` | Token-to-cost estimation |
| `internal/retry` | Error classification + exponential backoff retry |
| `internal/config` | YAML config loader |
| `internal/memory` | File-based memory store |
| `internal/sandbox` | Multi-level execution sandboxing (Docker/Ulimit/None) |

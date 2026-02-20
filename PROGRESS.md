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
| 12 | Daily Anchor | `2026-02-19-phase12-daily-anchor-design.md` | Done |
| 13 | Adversarial Review | `2026-02-19-phase13-adversarial-review-design.md` | Done |
| 14 | Plugin System | `2026-02-19-phase14-plugin-system-design.md` | Done |
| 15 | Redaction Pipeline | `2026-02-19-phase15-redaction-pipeline-design.md` | Done |
| 16 | System Health Level | `2026-02-19-phase16-system-health-level-design.md` | Done |
| 17 | Causal Chain / Tracing | `2026-02-20-phase17-causal-chain-tracing-design.md` | Done |
| 18 | Metrics Export | `2026-02-20-phase18-metrics-export-design.md` | Done |
| 19 | Maintenance GC | `2026-02-20-phase19-maintenance-gc-design.md` | Done |
| 20 | Hypothesis Board | `2026-02-20-phase20-hypothesis-board-design.md` | Done |

## Current: Phase 21 — TBD

No phase in progress.

## Testing

| Suite | Command | Coverage |
|-------|---------|----------|
| Unit tests | `make test` | 29 packages |
| E2E tests (mock) | `make e2e` | 60 tests, all CLI commands |
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
| `internal/audit` | Structured audit logging + daily anchor verification |
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
| `internal/reasoning` | Adversarial review debate protocol + protocol registry |
| `internal/plugin` | Plugin management framework with directory scanning + SHA-256 verification |
| `internal/redact` | Redaction pipeline with built-in secret patterns + IP filtering |
| `internal/health` | System health level state machine (GREEN/YELLOW/RED/CRITICAL) |
| `internal/trace` | Causal chain tracing with TraceContext + parent-child linkage |
| `internal/metrics` | Metrics collection and export (runs, DAG, health, audit) |
| `internal/gc` | Garbage collection for old runs, audit logs, and snapshots |
| `internal/hypothesis` | Hypothesis board with propose/challenge/confirm/reject lifecycle |

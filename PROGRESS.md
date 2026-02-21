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
| 21 | Run Manifest Diffing | `2026-02-20-phase21-run-diff-design.md` | Done |
| 22 | Policy Change Audit | `2026-02-20-phase22-policy-change-audit-design.md` | Done |
| 23 | System Dashboard | `2026-02-20-phase23-dashboard-design.md` | Done |
| 24 | Content-Addressed Artifact Storage | `2026-02-20-phase24-artifact-storage-design.md` | Done |
| 25 | Knowledge Graph | `2026-02-20-phase25-knowledge-graph-design.md` | Done |
| 26 | Aggregation Pipeline | `2026-02-20-phase26-aggregation-pipeline-design.md` | Done |
| 27 | Memory Import/Export | `2026-02-20-phase27-memory-import-export-design.md` | Done |
| 28 | Artifact Lineage | `2026-02-20-phase28-artifact-lineage-design.md` | Done |
| 29 | Rate Limit Groups | `2026-02-20-phase29-rate-limit-groups-design.md` | Done |
| 30 | Memory Auto-Cleanup | `2026-02-20-phase30-memory-auto-cleanup-design.md` | Done |
| 31 | Connector Framework | `2026-02-20-phase31-connector-framework-design.md` | Done |
| 32 | Async Event Runtime | `2026-02-20-phase32-async-event-runtime-design.md` | Done |
| 33 | Schema Migration | `2026-02-20-phase33-schema-migration-design.md` | Done |
| 34 | External Data Puller | `2026-02-20-phase34-external-data-puller-design.md` | Done |
| 35 | Credential Injector | `2026-02-20-phase35-credential-injector-design.md` | Done |
| 36 | Context Paging Tool | `2026-02-20-phase36-context-paging-design.md` | Done |
| 37 | Mode Selector | `2026-02-20-phase37-mode-selector-design.md` | Done |
| 38 | Progress Tracker | `2026-02-20-phase38-progress-tracker-design.md` | Done |
| 39 | Configuration Profile Manager | `2026-02-20-phase39-config-profile-design.md` | Done |
| 40 | Notification System | `2026-02-20-phase40-notification-system-design.md` | Done |
| 41 | Fail-Closed Gate | `2026-02-21-phase41-fail-closed-gate-design.md` | Done |
| 42 | Runtime State DB | `2026-02-21-phase42-runtime-state-db-design.md` | Done |
| 43 | DAG State Machine Extension | `2026-02-21-phase43-dag-state-extension-design.md` | Done |
| 44 | Resource QoS | `2026-02-21-phase44-resource-qos-design.md` | Done |
| 45 | Task Template System | `2026-02-21-phase45-task-template-design.md` | Done |
| 46 | Run History Analytics | `2026-02-21-phase46-run-analytics-design.md` | Done |
| 47 | Environment Precheck | `2026-02-21-phase47-env-precheck-design.md` | Done |
| 48 | Data Reliability Foundation | `2026-02-21-phase48-data-reliability-design.md` | Done |
| 49 | Correctness & Verification Foundation | `2026-02-21-phase49-correctness-verification-design.md` | Done |

## Current: Phase 49 — Correctness & Verification Foundation ✅

Completed 2026-02-21. Four components forming the correctness verification layer:

1. **DAG State Completion** (`internal/dag/states.go`) — 7 new lifecycle states (Ready/Retrying/Resuming/Replanning/Invalidated/Escalated/NeedsHuman) + 8 transition methods (8 new tests, 32 total DAG tests)
2. **Rollback Quality** (`internal/dag/rollback.go`) — RollbackQuality grading (FULL/PARTIAL/STRUCTURAL/NONE) + RollbackResult type (3 tests)
3. **Invariant Framework** (`internal/invariant/`) — 9 correctness checkers (I1-I9): WAL-DB consistency, artifact reference, hanging actions, idempotency, trace completeness, audit hash chain, anchor consistency, dual-DB, lock ordering (7 tests)
4. **Memory Staged Commit** (`internal/staging/`) — 6-state pipeline (PENDING→VERIFIED/UNVERIFIED/REJECTED/EXPIRED→COMMITTED) + NLI keyword stub for conflict detection (10 tests)
5. **Doctor integration** — invariant I1-I9 checks in `apex doctor`
6. **run.go integration** — staging pipeline replaces direct memory save + auto-rollback on failure with quality grading
7. **Manifest extension** — `rollback_quality` field in run manifest
8. **E2E tests** — 3 new integration tests (invariant doctor, staging memory, rollback on failure)

## Testing

| Suite | Command | Coverage |
|-------|---------|----------|
| Unit tests | `make test` | 57 packages, 555 tests |
| E2E tests (mock) | `make e2e` | 142 tests, all CLI commands |
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
| `internal/audit` | Structured audit logging + daily anchor verification + policy change tracking |
| `internal/manifest` | Execution manifest tracking + run diffing |
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
| `internal/dashboard` | System status dashboard aggregating health, runs, metrics, audit |
| `internal/artifact` | Content-addressed artifact storage with SHA-256 dedup and orphan GC |
| `internal/kg` | Knowledge graph with entity-relationship storage, BFS traversal, and JSON persistence |
| `internal/aggregator` | Aggregation pipeline with summarize, merge, and reduce strategies |
| `internal/memport` | Memory import/export with JSON serialization and merge strategies |
| `internal/ratelimit` | Token bucket rate limiter with named groups for shared rate limiting |
| `internal/memclean` | Rule-based memory auto-cleanup with capacity threshold, stale detection, and exempt categories |
| `internal/connector` | Tool connector framework with YAML spec loading, 4-state circuit breaker, and registry |
| `internal/event` | Async event runtime with priority queue (URGENT/NORMAL/LONG_RUNNING) and handler router |
| `internal/migration` | Schema migration with PRAGMA user_version tracking, sequential registry, and pre-migration backup |
| `internal/datapuller` | External data puller with YAML spec loading, HTTP pull with auth, and JSON path transform |
| `internal/credinjector` | Zero-trust credential injection with placeholder refs, vault loading, inject/scrub, and error path protection |
| `internal/paging` | On-demand artifact content paging with line extraction, token estimation, and per-task budget enforcement |
| `internal/mode` | Execution mode selector with 5 modes (NORMAL/URGENT/EXPLORATORY/BATCH/LONG_RUNNING) and complexity-based auto-selection |
| `internal/progress` | Structured task progress tracking with Start/Update/Complete/Fail lifecycle and percent clamping |
| `internal/profile` | Named configuration profiles with registry, YAML loading, and environment switching (dev/staging/prod) |
| `internal/notify` | Event-driven notification with Channel interface, rule-based routing, and multi-channel dispatch |
| `internal/failclose` | Fail-closed safety gate with pluggable conditions, health/killswitch checks, and MustPass enforcement |
| `internal/statedb` | Centralized SQLite WAL runtime state DB with key-value state store and run record persistence |
| `internal/qos` | Priority-based resource QoS with slot reservation, 4-step allocation, and URGENT borrowing |
| `internal/template` | Reusable DAG templates with YAML loading, {{.VarName}} substitution, and Registry |
| `internal/analytics` | Run history analytics with summary, duration stats (P50/P90), and failure pattern detection |
| `internal/precheck` | Environment precheck with pluggable Check interface, DirCheck/FileCheck/BinaryCheck, and Runner |
| `internal/filelock` | Layered flock-based file locks with ordering enforcement (global→workspace), metadata tracking, and stale lock detection |
| `internal/writerq` | Single-writer DB queue serializing SQLite writes through one goroutine with batch transactions, panic recovery, and kill switch |
| `internal/outbox` | Action outbox with 7-step WAL protocol (STARTED→COMPLETED/FAILED), append-only JSONL with fsync, and startup reconciliation |
| `internal/invariant` | Correctness verification framework with 9 checkers (I1-I9) covering WAL-DB consistency, artifact refs, hanging actions, idempotency, trace completeness, audit hash chain, anchors, dual-DB, and lock ordering |
| `internal/staging` | Memory staged commit pipeline with 6-state lifecycle (PENDING→VERIFIED/UNVERIFIED/REJECTED/EXPIRED→COMMITTED) and keyword-based NLI conflict detection stub |

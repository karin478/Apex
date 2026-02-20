# Phase 32: Async Event Runtime — Implementation Plan

**Design:** `2026-02-20-phase32-async-event-runtime-design.md`

## Task 1: Event Core — Queue + Router

**Files:** `internal/event/event.go`, `internal/event/event_test.go`

**Tests (7):**
1. `TestNewEvent` — Verify ID, Type, Priority, Payload, CreatedAt
2. `TestQueuePushPop` — Push mixed priorities, Pop in priority order
3. `TestQueueEmpty` — Empty Pop returns false
4. `TestQueueStats` — Stats reflect counts per bucket
5. `TestRouterRegisterDispatch` — Register + dispatch verifies handler called
6. `TestRouterMultipleHandlers` — All handlers for type invoked
7. `TestRouterUnknownType` — Unknown type dispatch returns nil

**TDD workflow:** Write all 7 tests first, then implement.

## Task 2: Format + CLI — `apex event queue/types`

**Files:** `internal/event/format.go`, `internal/event/format_test.go`, `cmd/apex/event.go`

**Format functions:**
- `FormatQueueStats(stats QueueStats) string` — table: Priority, Count
- `FormatTypes(types []string) string` — list of registered types
- `FormatQueueStatsJSON(stats QueueStats) string`

**CLI:**
- `apex event queue [--format json]` — display queue statistics
- `apex event types` — list registered event types
- Register eventCmd in main.go

## Task 3: E2E Tests

**File:** `e2e/event_test.go`

**Tests (3):**
1. `TestEventQueueStats` — shows queue stats
2. `TestEventTypesEmpty` — no types shows empty message
3. `TestEventQueueRuns` — exits code 0

## Task 4: PROGRESS.md Update

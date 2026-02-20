# Phase 29: Rate Limit Groups — Implementation Plan

**Date:** 2026-02-20
**Design:** `2026-02-20-phase29-rate-limit-groups-design.md`
**Method:** Subagent-Driven Development

## Tasks

### Task 1: Rate Limiter Core (Limiter + Group)

**Files:** `internal/ratelimit/ratelimit.go`, `internal/ratelimit/ratelimit_test.go`
**Tests (7):**
1. `TestLimiterAllow` — Burst=3, consume 3 tokens, 4th rejected
2. `TestLimiterRefill` — After time.Sleep, tokens refill
3. `TestLimiterWait` — Blocks briefly then succeeds
4. `TestLimiterWaitCancel` — Context cancel returns context.Canceled
5. `TestGroupAdd` — Add limiter, verify in status
6. `TestGroupAllow` — Routes to correct limiter by name
7. `TestGroupNotFound` — Unknown group name returns error

**Spec:**
- LimiterStatus: Name, Rate, Burst, Available (json tags)
- Limiter: name, rate, burst, tokens, lastRefill, sync.Mutex
- NewLimiter(name, rate, burst) — tokens initialized to burst
- Allow() — refill, consume 1 if available, return bool
- Wait(ctx) — loop: try Allow, if not sleep min(1/rate, 100ms), check ctx
- Status() — refill, return LimiterStatus snapshot
- refill() — lazy token calculation
- Group: limiters map + sync.RWMutex
- NewGroup() — empty map
- Add(name, rate, burst) — creates and stores Limiter
- Allow(name) — (bool, error), error if not found
- Wait(name, ctx) — error if not found
- Status() — collect all LimiterStatus
- Remove(name) — delete from map

### Task 2: Format + CLI Command

**Files:** `internal/ratelimit/format.go`, `internal/ratelimit/format_test.go`, `cmd/apex/ratelimit.go`, update `cmd/apex/main.go`
**Tests (2):**
1. `TestFormatStatus` — Table with NAME/RATE/BURST/AVAILABLE columns
2. `TestFormatStatusJSON` — JSON array output

**Spec:**
- FormatStatus(statuses []LimiterStatus) string — tabular output
- FormatStatusJSON(statuses []LimiterStatus) string — JSON indent
- CLI: `apex ratelimit status [--format json]`
- Note: In current implementation, status shows empty since no groups are registered at runtime. Future phases will register groups via config.
- Register `ratelimitCmd` in rootCmd

### Task 3: E2E Tests

**Files:** `e2e/ratelimit_test.go`
**Tests (3):**
1. `TestRateLimitStatusEmpty` — No groups, shows "No rate limit groups"
2. `TestRateLimitStatusHelp` — Help shows available subcommand
3. `TestRateLimitStatusRuns` — Command exits cleanly

### Task 4: PROGRESS.md Update

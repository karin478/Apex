# Phase 29: Rate Limit Groups — Design Document

**Date:** 2026-02-20
**Status:** Approved (recommendation credit 7/10)

## Overview

Implement a token bucket rate limiter (`internal/ratelimit`) with named groups for shared rate limiting across multiple consumers. Supports both non-blocking `Allow()` and blocking `Wait()` patterns.

## Architecture

### Token Bucket Algorithm

Each limiter maintains a bucket of tokens that refills at a constant rate. Each request consumes one token. If the bucket is empty, the request is rejected (Allow) or blocks (Wait).

- Tokens refill at `rate` tokens/second
- Bucket capacity (burst) limits max tokens
- Lazy refill: tokens calculated on demand, not via timer

### Core Types

```go
type LimiterStatus struct {
    Name      string  `json:"name"`
    Rate      float64 `json:"rate"`       // tokens per second
    Burst     int     `json:"burst"`      // max bucket size
    Available float64 `json:"available"`  // current tokens
}

type Limiter struct {
    name       string
    rate       float64
    burst      int
    tokens     float64
    lastRefill time.Time
    mu         sync.Mutex
}

type Group struct {
    limiters map[string]*Limiter
    mu       sync.RWMutex
}
```

### Operations

```go
// Limiter
func NewLimiter(name string, rate float64, burst int) *Limiter
func (l *Limiter) Allow() bool                          // Non-blocking: consume 1 token or reject
func (l *Limiter) Wait(ctx context.Context) error       // Blocking: wait until token available or ctx done
func (l *Limiter) Status() LimiterStatus                // Current state snapshot
func (l *Limiter) refill()                              // Lazy token refill (private)

// Group
func NewGroup() *Group
func (g *Group) Add(name string, rate float64, burst int)
func (g *Group) Allow(name string) (bool, error)        // error if group not found
func (g *Group) Wait(name string, ctx context.Context) error
func (g *Group) Status() []LimiterStatus                // All limiters
func (g *Group) Remove(name string)
```

### Token Refill Logic

```
elapsed = now - lastRefill
newTokens = elapsed.Seconds() * rate
tokens = min(tokens + newTokens, burst)
lastRefill = now
```

### CLI Command

```
apex ratelimit status    Show all rate limit groups and their current state
```

Output format:
```
Rate Limit Groups:

  NAME            RATE       BURST    AVAILABLE
  k8s_internal    30.0/s     30       28.0
  external_api    60.0/s     60       60.0
```

## Testing

| Test | Description |
|------|-------------|
| TestLimiterAllow | Burst tokens consumed, then rejected |
| TestLimiterRefill | After waiting, tokens refill |
| TestLimiterWait | Blocks until token available |
| TestLimiterWaitCancel | Context cancel returns error |
| TestGroupAdd | Add and retrieve limiter |
| TestGroupAllow | Allow routes to correct limiter |
| TestGroupNotFound | Unknown group returns error |
| TestFormatStatus | Human-readable table output |
| TestFormatStatusJSON | JSON output |
| E2E: TestRateLimitStatusEmpty | No groups shows clean message |
| E2E: TestRateLimitStatusHelp | Help text shows subcommand |
| E2E: TestRateLimitStatusRuns | Command exits cleanly |

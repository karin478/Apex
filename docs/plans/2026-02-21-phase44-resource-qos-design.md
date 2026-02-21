# Phase 44: Resource QoS (Quality of Service)

> Design doc for Apex Agent CLI — resource reservation and priority-based concurrency slot management.

## Problem

The agent pool uses a flat concurrency limit (e.g., max 8 agents). When the system is saturated, URGENT tasks compete equally with BATCH tasks for slots. There is no mechanism to reserve capacity for high-priority work or preempt lower-priority tasks. The architecture requires resource QoS to guarantee responsiveness for critical operations.

## Solution

A `qos` package that manages concurrency slots with priority-based reservation. Each priority level can reserve a minimum number of slots, and allocation follows a strict priority ordering. The system ensures URGENT tasks always have capacity available.

## Architecture

```
internal/qos/
├── qos.go       # SlotPool, Reservation, Allocate/Release
└── qos_test.go  # 7 unit tests
```

## Key Types

### Reservation

```go
type Reservation struct {
    Priority string `json:"priority"`   // URGENT / HIGH / NORMAL / LOW
    Reserved int    `json:"reserved"`   // minimum guaranteed slots
}
```

### SlotPool

```go
type SlotPool struct {
    mu           sync.Mutex
    total        int
    used         int
    reservations map[string]Reservation
    allocated    map[string]int // current allocation per priority
}
```

## Priority Levels

| Priority | Value | Description |
|----------|-------|-------------|
| URGENT | 0 | Safety-critical, always gets slots |
| HIGH | 1 | Important but not safety-critical |
| NORMAL | 2 | Default priority |
| LOW | 3 | Background tasks, best-effort |

## Core Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `PriorityValue` | `(priority string) int` | URGENT=0, HIGH=1, NORMAL=2, LOW=3, unknown=99 |
| `NewSlotPool` | `(total int) *SlotPool` | Creates pool with given total capacity |
| `(*SlotPool) AddReservation` | `(r Reservation) error` | Adds reservation; error if total reserved exceeds capacity |
| `(*SlotPool) Allocate` | `(priority string) bool` | Try to allocate a slot; returns false if no capacity |
| `(*SlotPool) Release` | `(priority string)` | Release one slot for the given priority |
| `(*SlotPool) Available` | `() int` | Returns number of unallocated slots |
| `(*SlotPool) Usage` | `() PoolUsage` | Returns snapshot of current usage |
| `(*SlotPool) Total` | `() int` | Returns total capacity |
| `(*SlotPool) Reservations` | `() []Reservation` | Returns all reservations sorted by priority value |

### PoolUsage

```go
type PoolUsage struct {
    Total     int            `json:"total"`
    Used      int            `json:"used"`
    Available int            `json:"available"`
    ByPriority map[string]int `json:"by_priority"` // allocated per priority
}
```

## Allocation Logic

`Allocate(priority)` succeeds if:
1. There are free slots (`used < total`), AND
2. Either:
   a. The priority has reserved slots and its allocated count is below its reservation, OR
   b. The unreserved pool (total - sum of all reservations) has free capacity, OR
   c. The priority has a higher priority value (lower number) than any priority with unused reserved slots (preemption of reserved-but-unused capacity)

Simplified: URGENT can always use any free slot. Lower priorities can only use unreserved slots or their own reserved slots.

## Design Decisions

### Priority as String

Consistent with project patterns (mode, health level, event level all use strings with value-mapping functions).

### Reservation Not Enforcement

Reservations guarantee minimum capacity, not maximum. An URGENT task can use slots beyond its reservation if available.

### No Preemption of Running Tasks

Allocate only checks available slots — it never preempts a running task. This avoids complexity and ensures task completion.

### Mutex Protection

`sync.Mutex` (not RWMutex) since most operations modify state.

## CLI Commands

### `apex qos status [--format json]`
Shows pool usage: total, used, available, per-priority breakdown.

### `apex qos reservations`
Lists all reservations.

## Unit Tests (7)

| Test | Description |
|------|-------------|
| `TestPriorityValue` | URGENT=0, HIGH=1, NORMAL=2, LOW=3, unknown=99 |
| `TestNewSlotPool` | Creates pool with correct total and 0 used |
| `TestAddReservation` | Reservations accumulate; error when exceeding total |
| `TestAllocateRelease` | Allocate increments used, Release decrements |
| `TestAllocateRespectReservation` | URGENT reserved slots cannot be taken by LOW |
| `TestAllocateUrgentAlways` | URGENT can use any free slot even when others are reserved |
| `TestUsage` | Usage snapshot reflects current state correctly |

## E2E Tests (2)

| Test | Description |
|------|-------------|
| `TestQoSStatus` | CLI shows pool usage info |
| `TestQoSReservations` | CLI lists reservations |

## Format Functions

| Function | Description |
|----------|-------------|
| `FormatUsage(usage PoolUsage) string` | Pool usage display |
| `FormatReservations(reservations []Reservation) string` | Table: PRIORITY / RESERVED |
| `FormatUsageJSON(usage PoolUsage) (string, error)` | JSON output |

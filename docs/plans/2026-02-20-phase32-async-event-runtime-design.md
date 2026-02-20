# Phase 32: Async Event Runtime — Design Document

**Date:** 2026-02-20
**Status:** Approved (recommendation credit 1/6)

## Overview

Implement an async event runtime (`internal/event`) with priority-based event queue, event router with handler registration, and CLI commands. Based on Architecture v11.0 §1.1 Execution Layer.

## Architecture

### Priority Levels (from Architecture)

- URGENT (0): Reserved 20% token budget, highest dispatch priority
- NORMAL (1): Standard operations
- LONG_RUNNING (2): Background tasks, lowest priority

### Core Types

```go
type Priority int

const (
    PriorityURGENT       Priority = 0
    PriorityNORMAL       Priority = 1
    PriorityLONG_RUNNING Priority = 2
)

type Event struct {
    ID        string    `json:"id"`
    Type      string    `json:"type"`
    Priority  Priority  `json:"priority"`
    Payload   string    `json:"payload"`
    CreatedAt time.Time `json:"created_at"`
}

type Handler func(Event) error

type Router struct {
    handlers map[string][]Handler
    mu       sync.RWMutex
}

type Queue struct {
    urgent      []Event
    normal      []Event
    longRunning []Event
    mu          sync.Mutex
    notify      chan struct{}
}

type QueueStats struct {
    Urgent      int `json:"urgent"`
    Normal      int `json:"normal"`
    LongRunning int `json:"long_running"`
    Total       int `json:"total"`
}
```

### Operations

```go
func NewEvent(eventType string, priority Priority, payload string) Event
func NewQueue() *Queue
func (q *Queue) Push(e Event)
func (q *Queue) Pop() (Event, bool)
func (q *Queue) Len() int
func (q *Queue) Stats() QueueStats
func NewRouter() *Router
func (r *Router) Register(eventType string, h Handler)
func (r *Router) Dispatch(e Event) error
func (r *Router) Types() []string
```

- **NewEvent**: Generates UUID-style ID, sets CreatedAt to now.
- **Queue**: Three internal slices (urgent/normal/longRunning). Push adds to appropriate bucket and signals notify channel. Pop dequeues in strict priority order (urgent first). Thread-safe with sync.Mutex.
- **Router**: Maps event types to handler slices. Dispatch calls all handlers for the event type. Thread-safe with sync.RWMutex. Unknown types are no-op (return nil).

### CLI Command

```
apex event queue     # Show queue statistics
apex event types     # List registered event types
```

## Testing

| Test | Description |
|------|-------------|
| TestNewEvent | Verify ID, Type, Priority, Payload, CreatedAt fields |
| TestQueuePushPop | Push mixed priorities, Pop returns in priority order |
| TestQueueEmpty | Empty queue Pop returns false |
| TestQueueStats | Stats reflect counts per priority bucket |
| TestRouterRegisterDispatch | Register handler, dispatch event, verify called |
| TestRouterMultipleHandlers | Multiple handlers for same type all invoked |
| TestRouterUnknownType | Dispatch unknown type returns nil |
| E2E: TestEventQueueStats | apex event queue shows stats |
| E2E: TestEventTypesEmpty | apex event types shows empty |
| E2E: TestEventQueueRuns | Command exits cleanly |

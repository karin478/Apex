# Phase 40: Notification System

> Design doc for Apex Agent CLI — event-driven notification with multi-channel support.

## Problem

When long-running tasks complete, fail, or require attention, users have no way to be notified unless they're actively watching the terminal. The system needs a mechanism to push notifications through configurable channels when significant events occur.

## Solution

A `notify` package with a `Channel` interface for pluggable notification backends, a `Rule` struct for event-to-channel routing, and a `Dispatcher` that evaluates events against rules and sends notifications through matched channels.

## Architecture

```
internal/notify/
├── notify.go       # Channel interface, Rule, Event, Dispatcher
└── notify_test.go  # 7 unit tests
```

## Key Types

### Event

```go
type Event struct {
    Type    string `json:"type"`
    TaskID  string `json:"task_id"`
    Message string `json:"message"`
    Level   string `json:"level"` // INFO / WARN / ERROR
}
```

### Channel

```go
type Channel interface {
    Name() string
    Send(event Event) error
}
```

Built-in channels:
- `StdoutChannel` — prints to stdout (default, always available)
- `FileChannel` — appends to a log file

### Rule

```go
type Rule struct {
    EventType string `json:"event_type" yaml:"event_type"` // event type to match, "*" for all
    MinLevel  string `json:"min_level"   yaml:"min_level"`  // minimum level: INFO/WARN/ERROR
    Channel   string `json:"channel"     yaml:"channel"`    // channel name to route to
}
```

### Dispatcher

```go
type Dispatcher struct {
    mu       sync.RWMutex
    channels map[string]Channel
    rules    []Rule
}
```

## Core Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `NewDispatcher` | `() *Dispatcher` | Creates empty dispatcher |
| `(*Dispatcher) RegisterChannel` | `(ch Channel) error` | Registers a channel; error if name empty |
| `(*Dispatcher) AddRule` | `(rule Rule)` | Adds a routing rule |
| `(*Dispatcher) Dispatch` | `(event Event) []error` | Evaluates rules, sends to matched channels, returns errors |
| `(*Dispatcher) Channels` | `() []string` | Returns registered channel names sorted |
| `(*Dispatcher) Rules` | `() []Rule` | Returns all rules |
| `LevelValue` | `(level string) int` | INFO=0, WARN=1, ERROR=2, unknown=-1 |
| `MatchRule` | `(rule Rule, event Event) bool` | True if event type matches and level >= min_level |
| `NewStdoutChannel` | `() *StdoutChannel` | Creates stdout channel |
| `NewFileChannel` | `(path string) *FileChannel` | Creates file channel |

## Rule Matching

`MatchRule(rule, event)` returns true when:
1. `rule.EventType == "*"` OR `rule.EventType == event.Type`
2. AND `LevelValue(event.Level) >= LevelValue(rule.MinLevel)`

## Design Decisions

### Channel Interface

Keeps notification backends pluggable. StdoutChannel and FileChannel are built-in; webhook/Slack can be added later without changing the core.

### Rules as Flat List

Rules are evaluated sequentially. An event can match multiple rules and be sent to multiple channels. Simple and predictable.

### Dispatch Returns []error

Non-fatal: if one channel fails, others still get the notification. Caller decides how to handle partial failures.

### Level as String

Using string levels (INFO/WARN/ERROR) with a `LevelValue` helper for comparison. Consistent with the string enum pattern used throughout the project.

### Mutex Protection

`sync.RWMutex` on Dispatcher, consistent with project patterns.

## CLI Commands

### `apex notify list`
Lists registered channels and rules.

### `apex notify send <type> <message> [--level INFO]`
Sends a test notification event.

### `apex notify channels`
Lists registered notification channels.

## Unit Tests (7)

| Test | Description |
|------|-------------|
| `TestLevelValue` | INFO=0, WARN=1, ERROR=2, unknown=-1 |
| `TestMatchRule` | Wildcard match, type match, level filter, no match |
| `TestNewDispatcher` | Empty dispatcher has no channels or rules |
| `TestDispatcherRegisterChannel` | Register succeeds; empty name returns error |
| `TestDispatcherAddRule` | Rules accumulate correctly |
| `TestDispatcherDispatch` | Event routed to correct channels based on rules |
| `TestDispatcherDispatchPartialFailure` | One channel fails, others still receive |

## E2E Tests (3)

| Test | Description |
|------|-------------|
| `TestNotifyChannels` | CLI lists default channels |
| `TestNotifySend` | CLI sends test notification |
| `TestNotifyList` | CLI lists rules and channels |

## Format Functions

| Function | Description |
|----------|-------------|
| `FormatChannelList(names []string) string` | Table of channel names |
| `FormatRuleList(rules []Rule) string` | Table: EVENT_TYPE / MIN_LEVEL / CHANNEL |
| `FormatRuleListJSON(rules []Rule) (string, error)` | JSON output |

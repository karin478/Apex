# Notification System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an `internal/notify` package with Channel interface, Rule-based event routing, and Dispatcher for multi-channel notifications.

**Architecture:** Event struct, Channel interface (Stdout/File), Rule matching, Dispatcher with mutex-protected channel registry + rule list. Format functions + Cobra CLI.

**Tech Stack:** Go, `sync`, `os`, `fmt`, `sort`, `encoding/json`, Testify, Cobra CLI

---

### Task 1: Notification System Core — Channel + Rule + Dispatcher (7 tests)

**Files:**
- Create: `internal/notify/notify.go`
- Create: `internal/notify/notify_test.go`

**Step 1: Write 7 failing tests**

```go
package notify

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLevelValue(t *testing.T) {
	assert.Equal(t, 0, LevelValue("INFO"))
	assert.Equal(t, 1, LevelValue("WARN"))
	assert.Equal(t, 2, LevelValue("ERROR"))
	assert.Equal(t, -1, LevelValue("UNKNOWN"))
}

func TestMatchRule(t *testing.T) {
	t.Run("wildcard match", func(t *testing.T) {
		rule := Rule{EventType: "*", MinLevel: "INFO", Channel: "stdout"}
		event := Event{Type: "run.completed", Level: "INFO"}
		assert.True(t, MatchRule(rule, event))
	})

	t.Run("type match", func(t *testing.T) {
		rule := Rule{EventType: "run.completed", MinLevel: "INFO", Channel: "stdout"}
		event := Event{Type: "run.completed", Level: "WARN"}
		assert.True(t, MatchRule(rule, event))
	})

	t.Run("level filter", func(t *testing.T) {
		rule := Rule{EventType: "*", MinLevel: "ERROR", Channel: "stdout"}
		event := Event{Type: "run.completed", Level: "INFO"}
		assert.False(t, MatchRule(rule, event))
	})

	t.Run("type mismatch", func(t *testing.T) {
		rule := Rule{EventType: "health.red", MinLevel: "INFO", Channel: "stdout"}
		event := Event{Type: "run.completed", Level: "ERROR"}
		assert.False(t, MatchRule(rule, event))
	})
}

func TestNewDispatcher(t *testing.T) {
	d := NewDispatcher()
	assert.Empty(t, d.Channels())
	assert.Empty(t, d.Rules())
}

func TestDispatcherRegisterChannel(t *testing.T) {
	d := NewDispatcher()

	t.Run("valid channel", func(t *testing.T) {
		err := d.RegisterChannel(&mockChannel{name: "test"})
		require.NoError(t, err)
		assert.Contains(t, d.Channels(), "test")
	})

	t.Run("empty name", func(t *testing.T) {
		err := d.RegisterChannel(&mockChannel{name: ""})
		assert.Error(t, err)
	})
}

func TestDispatcherAddRule(t *testing.T) {
	d := NewDispatcher()
	d.AddRule(Rule{EventType: "*", MinLevel: "INFO", Channel: "stdout"})
	d.AddRule(Rule{EventType: "run.completed", MinLevel: "WARN", Channel: "file"})
	assert.Len(t, d.Rules(), 2)
}

func TestDispatcherDispatch(t *testing.T) {
	d := NewDispatcher()
	ch1 := &mockChannel{name: "ch1"}
	ch2 := &mockChannel{name: "ch2"}
	err := d.RegisterChannel(ch1)
	require.NoError(t, err)
	err = d.RegisterChannel(ch2)
	require.NoError(t, err)

	d.AddRule(Rule{EventType: "run.completed", MinLevel: "INFO", Channel: "ch1"})
	d.AddRule(Rule{EventType: "*", MinLevel: "ERROR", Channel: "ch2"})

	event := Event{Type: "run.completed", Level: "INFO", TaskID: "t1", Message: "done"}
	errs := d.Dispatch(event)
	assert.Empty(t, errs)

	// ch1 should have received the event (type match).
	assert.Len(t, ch1.received, 1)
	assert.Equal(t, "run.completed", ch1.received[0].Type)

	// ch2 should NOT have received (level too low for ERROR rule).
	assert.Empty(t, ch2.received)
}

func TestDispatcherDispatchPartialFailure(t *testing.T) {
	d := NewDispatcher()
	good := &mockChannel{name: "good"}
	bad := &mockChannel{name: "bad", failOnSend: true}
	err := d.RegisterChannel(good)
	require.NoError(t, err)
	err = d.RegisterChannel(bad)
	require.NoError(t, err)

	d.AddRule(Rule{EventType: "*", MinLevel: "INFO", Channel: "good"})
	d.AddRule(Rule{EventType: "*", MinLevel: "INFO", Channel: "bad"})

	event := Event{Type: "test", Level: "INFO", Message: "hello"}
	errs := d.Dispatch(event)

	// One error from bad channel.
	assert.Len(t, errs, 1)
	// Good channel still received the event.
	assert.Len(t, good.received, 1)
}

// mockChannel is a test double for Channel.
type mockChannel struct {
	name       string
	received   []Event
	failOnSend bool
}

func (m *mockChannel) Name() string { return m.name }

func (m *mockChannel) Send(event Event) error {
	if m.failOnSend {
		return fmt.Errorf("mock: send failed")
	}
	m.received = append(m.received, event)
	return nil
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/notify/ -v -count=1`
Expected: FAIL — functions not defined

**Step 3: Write minimal implementation**

```go
// Package notify provides event-driven notifications with multi-channel support.
package notify

import (
	"fmt"
	"os"
	"sort"
	"sync"
)

// Event represents a notification event.
type Event struct {
	Type    string `json:"type"`
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
	Level   string `json:"level"` // INFO / WARN / ERROR
}

// Channel is the interface for notification backends.
type Channel interface {
	Name() string
	Send(event Event) error
}

// Rule defines how an event is routed to a channel.
type Rule struct {
	EventType string `json:"event_type" yaml:"event_type"`
	MinLevel  string `json:"min_level"  yaml:"min_level"`
	Channel   string `json:"channel"    yaml:"channel"`
}

// Dispatcher evaluates events against rules and sends to matched channels.
type Dispatcher struct {
	mu       sync.RWMutex
	channels map[string]Channel
	rules    []Rule
}

// NewDispatcher creates an empty Dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{channels: make(map[string]Channel)}
}

// RegisterChannel registers a notification channel. Error if name is empty.
func (d *Dispatcher) RegisterChannel(ch Channel) error {
	if ch.Name() == "" {
		return fmt.Errorf("notify: channel name cannot be empty")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.channels[ch.Name()] = ch
	return nil
}

// AddRule adds a routing rule.
func (d *Dispatcher) AddRule(rule Rule) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.rules = append(d.rules, rule)
}

// Dispatch evaluates an event against all rules and sends to matched channels.
// Returns errors from channels that fail (other channels still receive).
func (d *Dispatcher) Dispatch(event Event) []error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var errs []error
	sent := make(map[string]bool)

	for _, rule := range d.rules {
		if !MatchRule(rule, event) {
			continue
		}
		if sent[rule.Channel] {
			continue
		}
		ch, ok := d.channels[rule.Channel]
		if !ok {
			continue
		}
		if err := ch.Send(event); err != nil {
			errs = append(errs, fmt.Errorf("notify: channel %s: %w", rule.Channel, err))
		}
		sent[rule.Channel] = true
	}
	return errs
}

// Channels returns registered channel names sorted.
func (d *Dispatcher) Channels() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	names := make([]string, 0, len(d.channels))
	for name := range d.channels {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Rules returns all rules.
func (d *Dispatcher) Rules() []Rule {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]Rule, len(d.rules))
	copy(result, d.rules)
	return result
}

// LevelValue converts a level string to an integer for comparison.
func LevelValue(level string) int {
	switch level {
	case "INFO":
		return 0
	case "WARN":
		return 1
	case "ERROR":
		return 2
	default:
		return -1
	}
}

// MatchRule returns true if the event matches the rule.
func MatchRule(rule Rule, event Event) bool {
	if rule.EventType != "*" && rule.EventType != event.Type {
		return false
	}
	return LevelValue(event.Level) >= LevelValue(rule.MinLevel)
}

// StdoutChannel sends notifications to stdout.
type StdoutChannel struct{}

// NewStdoutChannel creates a stdout channel.
func NewStdoutChannel() *StdoutChannel { return &StdoutChannel{} }

// Name returns "stdout".
func (c *StdoutChannel) Name() string { return "stdout" }

// Send prints the event to stdout.
func (c *StdoutChannel) Send(event Event) error {
	fmt.Fprintf(os.Stdout, "[%s] %s: %s\n", event.Level, event.Type, event.Message)
	return nil
}

// FileChannel appends notifications to a file.
type FileChannel struct {
	path string
}

// NewFileChannel creates a file channel.
func NewFileChannel(path string) *FileChannel { return &FileChannel{path: path} }

// Name returns "file".
func (c *FileChannel) Name() string { return "file" }

// Send appends the event to the file.
func (c *FileChannel) Send(event Event) error {
	f, err := os.OpenFile(c.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("notify: open file: %w", err)
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "[%s] %s: %s\n", event.Level, event.Type, event.Message)
	return err
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/notify/ -v -count=1 -race`
Expected: PASS (7 tests)

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/notify/notify.go internal/notify/notify_test.go
git commit -m "feat(notify): add notification dispatcher with Channel interface, rule matching, and multi-channel routing

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: Format Functions + CLI Commands

**Files:**
- Create: `internal/notify/format.go`
- Create: `cmd/apex/notify.go`
- Modify: `cmd/apex/main.go` (add `rootCmd.AddCommand(notifyCmd)` after the `profileCmd` line)

**Step 1: Write format.go**

```go
package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatChannelList formats channel names as a list.
func FormatChannelList(names []string) string {
	if len(names) == 0 {
		return "No channels registered.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-20s\n", "CHANNEL")
	for _, name := range names {
		fmt.Fprintf(&b, "%-20s\n", name)
	}
	return b.String()
}

// FormatRuleList formats rules as a human-readable table.
func FormatRuleList(rules []Rule) string {
	if len(rules) == 0 {
		return "No rules configured.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-20s %-10s %s\n", "EVENT_TYPE", "MIN_LEVEL", "CHANNEL")
	for _, r := range rules {
		fmt.Fprintf(&b, "%-20s %-10s %s\n", r.EventType, r.MinLevel, r.Channel)
	}
	return b.String()
}

// FormatRuleListJSON formats rules as indented JSON.
func FormatRuleListJSON(rules []Rule) (string, error) {
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return "", fmt.Errorf("notify: json marshal: %w", err)
	}
	return string(data), nil
}
```

**Step 2: Write cmd/apex/notify.go CLI**

```go
package main

import (
	"fmt"

	"github.com/lyndonlyu/apex/internal/notify"
	"github.com/spf13/cobra"
)

var notifyLevel string

// notifyDispatcher is a package-level dispatcher with default channels and rules.
var notifyDispatcher = func() *notify.Dispatcher {
	d := notify.NewDispatcher()
	_ = d.RegisterChannel(notify.NewStdoutChannel())
	d.AddRule(notify.Rule{EventType: "*", MinLevel: "INFO", Channel: "stdout"})
	return d
}()

var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Notification management",
}

var notifyChannelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "List registered notification channels",
	RunE:  runNotifyChannels,
}

var notifyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List notification rules",
	RunE:  runNotifyList,
}

var notifySendCmd = &cobra.Command{
	Use:   "send <type> <message>",
	Short: "Send a test notification",
	Args:  cobra.ExactArgs(2),
	RunE:  runNotifySend,
}

func init() {
	notifySendCmd.Flags().StringVar(&notifyLevel, "level", "INFO", "Event level (INFO/WARN/ERROR)")
	notifyCmd.AddCommand(notifyChannelsCmd, notifyListCmd, notifySendCmd)
}

func runNotifyChannels(cmd *cobra.Command, args []string) error {
	fmt.Print(notify.FormatChannelList(notifyDispatcher.Channels()))
	return nil
}

func runNotifyList(cmd *cobra.Command, args []string) error {
	fmt.Print(notify.FormatRuleList(notifyDispatcher.Rules()))
	return nil
}

func runNotifySend(cmd *cobra.Command, args []string) error {
	event := notify.Event{
		Type:    args[0],
		Message: args[1],
		Level:   notifyLevel,
	}
	errs := notifyDispatcher.Dispatch(event)
	for _, err := range errs {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %v\n", err)
	}
	return nil
}
```

**Step 3: Add command to main.go**

Add `rootCmd.AddCommand(notifyCmd)` after the `profileCmd` line in `cmd/apex/main.go`.

**Step 4: Run build + tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build ./cmd/apex/ && go test ./internal/notify/ -v -count=1`
Expected: BUILD OK, PASS

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/notify/format.go cmd/apex/notify.go cmd/apex/main.go
git commit -m "feat(notify): add format functions and CLI for notify channels/list/send

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: E2E Tests (3 tests)

**Files:**
- Create: `e2e/notify_test.go`

**Step 1: Write E2E tests**

```go
package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotifyChannels(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("notify", "channels")

	assert.Equal(t, 0, exitCode,
		"apex notify channels should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "stdout"),
		"stdout should contain 'stdout' channel, got: %s", stdout)
}

func TestNotifySend(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("notify", "send", "test.event", "hello world")

	assert.Equal(t, 0, exitCode,
		"apex notify send should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "test.event"),
		"stdout should contain event type, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "hello world"),
		"stdout should contain message, got: %s", stdout)
}

func TestNotifyList(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("notify", "list")

	assert.Equal(t, 0, exitCode,
		"apex notify list should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "EVENT_TYPE"),
		"stdout should contain EVENT_TYPE header, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "stdout"),
		"stdout should contain stdout channel in rules, got: %s", stdout)
}
```

**Step 2: Build and run E2E tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build -o bin/apex ./cmd/apex/ && go test ./e2e/ -run TestNotify -v -count=1`
Expected: PASS (3 tests)

**Step 3: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add e2e/notify_test.go
git commit -m "test(e2e): add E2E tests for notify channels, send, and list

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Update PROGRESS.md

**Files:**
- Modify: `PROGRESS.md`

**Step 1: Update completed phases table**

Add row: `| 40 | Notification System | \`2026-02-20-phase40-notification-system-design.md\` | Done |`

**Step 2: Update Current section**

Change "Phase 40 — TBD" → "Phase 41 — TBD"

**Step 3: Update test counts**

- Unit tests: 45 → 46 packages
- E2E tests: 117 → 120 tests

**Step 4: Add Key Package**

Add: `| \`internal/notify\` | Event-driven notification with Channel interface, rule-based routing, and multi-channel dispatch |`

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add PROGRESS.md
git commit -m "docs: mark Phase 40 Notification System as complete

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

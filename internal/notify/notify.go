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

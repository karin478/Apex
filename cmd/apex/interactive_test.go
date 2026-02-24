package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lyndonlyu/apex/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestSessionContext(t *testing.T) {
	s := &session{}
	assert.Empty(t, s.context())

	s.turns = append(s.turns, turn{task: "analyze code", summary: "found 3 issues"})
	ctx := s.context()
	assert.Contains(t, ctx, "analyze code")
	assert.Contains(t, ctx, "found 3 issues")
}

func TestSessionContextTruncation(t *testing.T) {
	s := &session{}
	// Add 7 turns, only last 5 should appear in context
	for i := 0; i < 7; i++ {
		s.turns = append(s.turns, turn{
			task:    fmt.Sprintf("task %d", i),
			summary: fmt.Sprintf("result %d", i),
		})
	}
	ctx := s.context()
	assert.NotContains(t, ctx, "task 0")
	assert.NotContains(t, ctx, "task 1")
	assert.Contains(t, ctx, "task 2")
	assert.Contains(t, ctx, "task 6")
}

func TestSessionContextSummaryTruncation(t *testing.T) {
	s := &session{}
	longSummary := strings.Repeat("x", 1000)
	s.turns = append(s.turns, turn{task: "test", summary: longSummary})
	ctx := s.context()
	assert.True(t, len(ctx) < 600, "context should be truncated")
	assert.Contains(t, ctx, "...")
}

func TestRenderRisk(t *testing.T) {
	// Verify no panics and returns non-empty for all levels
	assert.NotEmpty(t, renderRisk("LOW"))
	assert.NotEmpty(t, renderRisk("MEDIUM"))
	assert.NotEmpty(t, renderRisk("HIGH"))
	assert.NotEmpty(t, renderRisk("CRITICAL"))
	assert.NotEmpty(t, renderRisk("UNKNOWN"))
}

// --- Unit tests for registry functions ---

func TestFindCommand(t *testing.T) {
	cmd := findCommand("help")
	if cmd == nil || cmd.name != "help" {
		t.Error("should find /help")
	}
	cmd = findCommand("exit")
	if cmd == nil || cmd.name != "quit" {
		t.Error("should find /exit as alias of /quit")
	}
	cmd = findCommand("EXIT") // case insensitive
	if cmd == nil || cmd.name != "quit" {
		t.Error("should find /EXIT case-insensitively")
	}
	cmd = findCommand("nonexistent")
	if cmd != nil {
		t.Error("should return nil for unknown command")
	}
}

func TestBuildCompleter(t *testing.T) {
	c := buildCompleter()
	if c == nil {
		t.Error("should return non-nil completer")
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello", 3) != "hel..." {
		t.Error("should truncate")
	}
	if truncate("hi", 10) != "hi" {
		t.Error("should not truncate short strings")
	}
	// Test CJK (multi-byte) characters
	if truncate("你好世界测试", 2) != "你好..." {
		t.Error("should truncate by runes, not bytes")
	}
}

// --- Unit tests for handler logic ---

func TestCmdNew(t *testing.T) {
	cfg := &config.Config{}
	cfg.Claude.Model = "test"
	cfg.Sandbox.Level = "none"
	s := &session{
		cfg:         cfg,
		turns:       []turn{{task: "x", summary: "y"}},
		lastOutput:  "hello",
		attachments: []string{"file.txt"},
	}
	cmdNew(s, "", nil)
	if len(s.turns) != 0 || s.lastOutput != "" || len(s.attachments) != 0 {
		t.Error("session not reset")
	}
}

func TestCmdCompact(t *testing.T) {
	s := &session{
		cfg:   &config.Config{},
		turns: make([]turn, 5),
	}
	for i := range s.turns {
		s.turns[i] = turn{task: fmt.Sprintf("task %d", i), summary: strings.Repeat("x", 200)}
	}
	cmdCompact(s, "", nil)
	// First 3 turns should be truncated
	for i := 0; i < 3; i++ {
		if len([]rune(s.turns[i].summary)) > 84 { // 80 + "..."
			t.Errorf("turn %d not compacted: len=%d runes", i, len([]rune(s.turns[i].summary)))
		}
	}
	// Last 2 should be untouched
	if len(s.turns[3].summary) != 200 {
		t.Error("turn 3 should be full")
	}
}

func TestCmdCompactMinimal(t *testing.T) {
	s := &session{
		cfg:   &config.Config{},
		turns: []turn{{task: "a", summary: "b"}},
	}
	cmdCompact(s, "", nil) // should not panic with <= 2 turns
}

func TestCmdContext(t *testing.T) {
	s := &session{
		cfg:         &config.Config{},
		turns:       []turn{{task: "a", summary: "b"}, {task: "c", summary: "d"}},
		attachments: []string{"f1.go", "f2.go"},
	}
	cmdContext(s, "", nil) // should not panic
}

func TestCmdMention(t *testing.T) {
	s := &session{cfg: &config.Config{}}
	cmdMention(s, "/tmp/nonexistent_file_12345.txt", nil)
	if len(s.attachments) != 0 {
		t.Error("should not attach nonexistent file")
	}
}

func TestCmdModel(t *testing.T) {
	cfg := &config.Config{}
	cfg.Claude.Model = "claude-opus-4-6"
	cfg.Claude.Effort = "high"
	s := &session{cfg: cfg}

	// Switch model by alias
	cmdModel(s, "sonnet", nil)
	if s.cfg.Claude.Model != "claude-sonnet-4-5" {
		t.Errorf("model not switched to sonnet, got %s", s.cfg.Claude.Model)
	}
	if s.cfg.Claude.Effort != "high" {
		t.Error("effort should remain unchanged")
	}

	// Switch model by alias + effort
	cmdModel(s, "haiku low", nil)
	if s.cfg.Claude.Model != "claude-haiku-4-5" || s.cfg.Claude.Effort != "low" {
		t.Errorf("model+effort not switched, got %s %s", s.cfg.Claude.Model, s.cfg.Claude.Effort)
	}

	// Switch model by number
	cmdModel(s, "1", nil)
	if s.cfg.Claude.Model != "claude-opus-4-6" {
		t.Errorf("model not switched by number, got %s", s.cfg.Claude.Model)
	}

	// Switch model by full id
	cmdModel(s, "claude-sonnet-4-5", nil)
	if s.cfg.Claude.Model != "claude-sonnet-4-5" {
		t.Errorf("model not switched by full id, got %s", s.cfg.Claude.Model)
	}

	// Unknown model should not change current
	cmdModel(s, "unknown-model", nil)
	if s.cfg.Claude.Model != "claude-sonnet-4-5" {
		t.Error("unknown model should not change current model")
	}

	// Effort subcommand
	cmdModel(s, "effort medium", nil)
	if s.cfg.Claude.Effort != "medium" {
		t.Errorf("effort not set, got %s", s.cfg.Claude.Effort)
	}
}

func TestCmdVersion(t *testing.T) {
	s := &session{cfg: &config.Config{}}
	cmdVersion(s, "", nil) // should not panic
}

// --- Integration test for shell escape ---

func TestRunShellCommand(t *testing.T) {
	// Ensure it doesn't panic on a simple command
	runShellCommand("echo hello")
}

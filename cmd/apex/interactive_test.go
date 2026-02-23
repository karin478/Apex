package main

import (
	"fmt"
	"strings"
	"testing"

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

package reasoning

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndLoadReview(t *testing.T) {
	dir := t.TempDir()

	result := &ReviewResult{
		ID:        "test-id-123",
		Proposal:  "Use Redis for caching",
		CreatedAt: "2026-02-19T12:00:00Z",
		Steps: []Step{
			{Role: "advocate", Prompt: "p1", Output: "o1"},
			{Role: "critic", Prompt: "p2", Output: "o2"},
			{Role: "advocate", Prompt: "p3", Output: "o3"},
			{Role: "judge", Prompt: "p4", Output: "o4"},
		},
		Verdict: Verdict{
			Decision: "approve",
			Summary:  "Good proposal",
			Risks:    []string{"risk1"},
			Actions:  []string{"action1"},
		},
		DurationMs: 5000,
	}

	err := SaveReview(dir, result)
	require.NoError(t, err)

	// File should exist with correct name
	path := filepath.Join(dir, "test-id-123.json")
	assert.FileExists(t, path)

	// Permissions should be 0600
	info, _ := os.Stat(path)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	// Load and verify round-trip
	loaded, err := LoadReview(dir, "test-id-123")
	require.NoError(t, err)
	assert.Equal(t, result.Proposal, loaded.Proposal)
	assert.Equal(t, result.Verdict.Decision, loaded.Verdict.Decision)
	assert.Len(t, loaded.Steps, 4)
	assert.Equal(t, "advocate", loaded.Steps[0].Role)
}

func TestLoadReviewNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadReview(dir, "nonexistent")
	assert.Error(t, err)
}

func TestParseVerdictCleanJSON(t *testing.T) {
	input := `{"decision":"approve","summary":"Good idea","risks":["risk1"],"suggested_actions":["action1"]}`
	v, err := parseVerdict(input)
	require.NoError(t, err)
	assert.Equal(t, "approve", v.Decision)
	assert.Equal(t, "Good idea", v.Summary)
	assert.Equal(t, []string{"risk1"}, v.Risks)
	assert.Equal(t, []string{"action1"}, v.Actions)
}

func TestParseVerdictMarkdownWrapped(t *testing.T) {
	input := "Here is my verdict:\n```json\n{\"decision\":\"reject\",\"summary\":\"Too risky\",\"risks\":[\"r1\",\"r2\"],\"suggested_actions\":[\"a1\"]}\n```\nEnd of review."
	v, err := parseVerdict(input)
	require.NoError(t, err)
	assert.Equal(t, "reject", v.Decision)
	assert.Equal(t, []string{"r1", "r2"}, v.Risks)
}

func TestParseVerdictMalformed(t *testing.T) {
	input := "I think this is a good idea but I can't decide."
	v, err := parseVerdict(input)
	require.NoError(t, err)
	// Fallback: decision="revise", summary=full text
	assert.Equal(t, "revise", v.Decision)
	assert.Contains(t, v.Summary, "good idea")
}

// mockRunner implements reasoning.Runner for testing.
type mockRunner struct {
	responses []string
	calls     []string
	callIndex int
}

func (m *mockRunner) RunTask(ctx context.Context, task string) (string, error) {
	m.calls = append(m.calls, task)
	if m.callIndex < len(m.responses) {
		resp := m.responses[m.callIndex]
		m.callIndex++
		return resp, nil
	}
	return "fallback response", nil
}

func TestRunReview(t *testing.T) {
	mock := &mockRunner{
		responses: []string{
			"Redis is great for caching because...",
			"However, Redis has these risks...",
			"Let me address those concerns...",
			`{"decision":"approve","summary":"Redis is viable","risks":["single point of failure"],"suggested_actions":["add sentinel"]}`,
		},
	}

	result, err := RunReview(context.Background(), mock, "Use Redis for caching")
	require.NoError(t, err)

	assert.Equal(t, "Use Redis for caching", result.Proposal)
	assert.NotEmpty(t, result.ID)
	assert.NotEmpty(t, result.CreatedAt)
	require.Len(t, result.Steps, 4)

	// Verify roles
	assert.Equal(t, "advocate", result.Steps[0].Role)
	assert.Equal(t, "critic", result.Steps[1].Role)
	assert.Equal(t, "advocate", result.Steps[2].Role)
	assert.Equal(t, "judge", result.Steps[3].Role)

	// Verify outputs
	assert.Contains(t, result.Steps[0].Output, "Redis is great")
	assert.Contains(t, result.Steps[1].Output, "risks")

	// Verify verdict parsed
	assert.Equal(t, "approve", result.Verdict.Decision)
	assert.Equal(t, "Redis is viable", result.Verdict.Summary)
	assert.Equal(t, []string{"single point of failure"}, result.Verdict.Risks)

	// Verify 4 calls were made to the runner
	assert.Len(t, mock.calls, 4)

	// Verify context threading: advocate prompt includes proposal
	assert.Contains(t, mock.calls[0], "Use Redis for caching")
	// Critic prompt includes advocate output
	assert.Contains(t, mock.calls[1], "Redis is great")
	// Advocate response includes critic output
	assert.Contains(t, mock.calls[2], "risks")
	// Judge prompt includes all previous outputs
	assert.Contains(t, mock.calls[3], "Redis is great")
	assert.Contains(t, mock.calls[3], "risks")
	assert.Contains(t, mock.calls[3], "address those concerns")
}

func TestRunReviewWithProgress(t *testing.T) {
	mock := &mockRunner{
		responses: []string{
			"advocate output",
			"critic output",
			"response output",
			`{"decision":"revise","summary":"needs work","risks":[],"suggested_actions":[]}`,
		},
	}

	var progressSteps []int
	progress := func(step int, dur time.Duration) {
		progressSteps = append(progressSteps, step)
	}

	result, err := RunReviewWithProgress(context.Background(), mock, "test proposal", progress)
	require.NoError(t, err)
	assert.Len(t, result.Steps, 4)
	assert.Equal(t, []int{0, 1, 2, 3}, progressSteps)
	assert.Equal(t, "revise", result.Verdict.Decision)
}

type errorRunner struct {
	callCount int
	failAt    int
}

func (e *errorRunner) RunTask(ctx context.Context, task string) (string, error) {
	e.callCount++
	if e.callCount > e.failAt {
		return "", fmt.Errorf("executor failed")
	}
	return "output", nil
}

func TestRunReviewExecutorError(t *testing.T) {
	errRunner := &errorRunner{failAt: 1}
	_, err := RunReview(context.Background(), errRunner, "test proposal")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "critic")
}

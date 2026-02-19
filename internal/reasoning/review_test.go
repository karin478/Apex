package reasoning

import (
	"os"
	"path/filepath"
	"testing"

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

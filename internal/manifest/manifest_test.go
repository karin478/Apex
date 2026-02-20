package manifest

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	m := &Manifest{
		RunID:      "test-001",
		Task:       "build something",
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Model:      "claude-opus-4-6",
		Effort:     "high",
		RiskLevel:  "LOW",
		NodeCount:  2,
		DurationMs: 5000,
		Outcome:    "success",
		Nodes: []NodeResult{
			{ID: "step-1", Task: "do A", Status: "completed"},
			{ID: "step-2", Task: "do B", Status: "completed"},
		},
	}
	require.NoError(t, store.Save(m))
	assert.FileExists(t, filepath.Join(dir, "test-001", "manifest.json"))

	loaded, err := store.Load("test-001")
	require.NoError(t, err)
	assert.Equal(t, m.RunID, loaded.RunID)
	assert.Equal(t, m.Task, loaded.Task)
	assert.Len(t, loaded.Nodes, 2)
}

func TestLoadNotFound(t *testing.T) {
	store := NewStore(t.TempDir())
	_, err := store.Load("nonexistent")
	assert.Error(t, err)
}

func TestRecent(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	for i := 0; i < 5; i++ {
		ts := time.Now().Add(time.Duration(i) * time.Second).UTC().Format(time.RFC3339)
		store.Save(&Manifest{
			RunID:     fmt.Sprintf("run-%d", i),
			Task:      fmt.Sprintf("task %d", i),
			Timestamp: ts,
			Outcome:   "success",
		})
	}

	recent, err := store.Recent(3)
	require.NoError(t, err)
	assert.Len(t, recent, 3)
	assert.Equal(t, "run-4", recent[0].RunID)
}

func TestRecentEmpty(t *testing.T) {
	store := NewStore(t.TempDir())
	recent, err := store.Recent(5)
	require.NoError(t, err)
	assert.Empty(t, recent)
}

func TestManifestTraceID(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	m := &Manifest{
		RunID:      "trace-run-001",
		Task:       "traced task",
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Model:      "claude-opus-4-6",
		Effort:     "high",
		RiskLevel:  "LOW",
		NodeCount:  1,
		DurationMs: 1200,
		Outcome:    "success",
		TraceID:    "trace-abc-123",
		Nodes: []NodeResult{
			{ID: "step-1", Task: "do traced work", Status: "completed"},
		},
	}
	require.NoError(t, store.Save(m))

	loaded, err := store.Load("trace-run-001")
	require.NoError(t, err)
	assert.Equal(t, "trace-abc-123", loaded.TraceID)
}

func TestNodeResultActionID(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	m := &Manifest{
		RunID:      "action-run-001",
		Task:       "action task",
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Model:      "claude-opus-4-6",
		Effort:     "medium",
		RiskLevel:  "MEDIUM",
		NodeCount:  2,
		DurationMs: 3000,
		Outcome:    "success",
		Nodes: []NodeResult{
			{ID: "step-1", Task: "first action", Status: "completed", ActionID: "act-001"},
			{ID: "step-2", Task: "second action", Status: "completed", ActionID: "act-002"},
		},
	}
	require.NoError(t, store.Save(m))

	loaded, err := store.Load("action-run-001")
	require.NoError(t, err)
	require.Len(t, loaded.Nodes, 2)
	assert.Equal(t, "act-001", loaded.Nodes[0].ActionID)
	assert.Equal(t, "act-002", loaded.Nodes[1].ActionID)
}

package hypothesis

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBoard(t *testing.T) {
	b := NewBoard("session-1")
	assert.Equal(t, "session-1", b.SessionID)
	assert.Empty(t, b.Hypotheses)
}

func TestPropose(t *testing.T) {
	b := NewBoard("s1")
	h := b.Propose("The bug is in the auth module")
	assert.NotEmpty(t, h.ID)
	assert.Equal(t, Proposed, h.Status)
	assert.Equal(t, "The bug is in the auth module", h.Statement)
	assert.Len(t, b.Hypotheses, 1)
}

func TestChallenge(t *testing.T) {
	b := NewBoard("s1")
	h := b.Propose("Memory leak in pool")
	err := b.Challenge(h.ID, Evidence{Type: "log_line", Content: "no leak detected", Confidence: 0.6})
	require.NoError(t, err)

	updated, _ := b.Get(h.ID)
	assert.Equal(t, Challenged, updated.Status)
	assert.Len(t, updated.Evidence, 1)
}

func TestConfirm(t *testing.T) {
	b := NewBoard("s1")
	h := b.Propose("Race condition in DAG")
	err := b.Confirm(h.ID, Evidence{Type: "observation", Content: "reproduced under load", Confidence: 0.9})
	require.NoError(t, err)

	updated, _ := b.Get(h.ID)
	assert.Equal(t, Confirmed, updated.Status)
	assert.Len(t, updated.Evidence, 1)
}

func TestReject(t *testing.T) {
	b := NewBoard("s1")
	h := b.Propose("Disk full")
	err := b.Reject(h.ID, "df shows 60% free")
	require.NoError(t, err)

	updated, _ := b.Get(h.ID)
	assert.Equal(t, Rejected, updated.Status)
}

func TestGetNotFound(t *testing.T) {
	b := NewBoard("s1")
	_, err := b.Get("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestScore(t *testing.T) {
	h := &Hypothesis{
		Evidence: []Evidence{
			{Confidence: 0.8},
			{Confidence: 0.6},
		},
	}
	assert.InDelta(t, 0.7, Score(h), 0.001)
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "board.json")

	b := NewBoard("test-session")
	b.Propose("hypothesis A")
	b.Propose("hypothesis B")

	require.NoError(t, b.Save(path))

	loaded, err := LoadBoard(path)
	require.NoError(t, err)
	assert.Equal(t, "test-session", loaded.SessionID)
	assert.Len(t, loaded.Hypotheses, 2)
}

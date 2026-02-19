package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnchorWriteAndLoad(t *testing.T) {
	dir := t.TempDir()
	a := Anchor{
		Date:        "2026-02-19",
		ChainHash:   "abc123",
		RecordCount: 5,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	err := WriteAnchor(dir, a)
	require.NoError(t, err)
	path := filepath.Join(dir, "anchors.jsonl")
	assert.FileExists(t, path)
	anchors, err := LoadAnchors(dir)
	require.NoError(t, err)
	require.Len(t, anchors, 1)
	assert.Equal(t, "2026-02-19", anchors[0].Date)
	assert.Equal(t, "abc123", anchors[0].ChainHash)
	assert.Equal(t, 5, anchors[0].RecordCount)
}

func TestAnchorUpdateSameDay(t *testing.T) {
	dir := t.TempDir()
	WriteAnchor(dir, Anchor{Date: "2026-02-19", ChainHash: "hash1", RecordCount: 3, CreatedAt: "t1"})
	WriteAnchor(dir, Anchor{Date: "2026-02-19", ChainHash: "hash2", RecordCount: 5, CreatedAt: "t2"})
	anchors, err := LoadAnchors(dir)
	require.NoError(t, err)
	require.Len(t, anchors, 1, "should replace, not append")
	assert.Equal(t, "hash2", anchors[0].ChainHash)
	assert.Equal(t, 5, anchors[0].RecordCount)
}

func TestAnchorMultipleDays(t *testing.T) {
	dir := t.TempDir()
	WriteAnchor(dir, Anchor{Date: "2026-02-18", ChainHash: "hash18", RecordCount: 2, CreatedAt: "t1"})
	WriteAnchor(dir, Anchor{Date: "2026-02-19", ChainHash: "hash19", RecordCount: 3, CreatedAt: "t2"})
	anchors, err := LoadAnchors(dir)
	require.NoError(t, err)
	require.Len(t, anchors, 2)
	assert.Equal(t, "2026-02-18", anchors[0].Date)
	assert.Equal(t, "2026-02-19", anchors[1].Date)
}

func TestLoadAnchorsEmpty(t *testing.T) {
	dir := t.TempDir()
	anchors, err := LoadAnchors(dir)
	require.NoError(t, err)
	assert.Empty(t, anchors)
}

func TestAnchorFilePermissions(t *testing.T) {
	dir := t.TempDir()
	WriteAnchor(dir, Anchor{Date: "2026-02-19", ChainHash: "hash", RecordCount: 1, CreatedAt: "t1"})
	path := filepath.Join(dir, "anchors.jsonl")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

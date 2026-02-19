package audit

import (
	"os"
	"os/exec"
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

func TestMaybeCreateAnchor(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)
	logger.Log(Entry{Task: "task-0", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	logger.Log(Entry{Task: "task-1", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	created, err := MaybeCreateAnchor(logger, "")
	require.NoError(t, err)
	assert.True(t, created)
	anchors, _ := LoadAnchors(dir)
	require.Len(t, anchors, 1)
	today := time.Now().Format("2006-01-02")
	assert.Equal(t, today, anchors[0].Date)
	assert.Equal(t, 2, anchors[0].RecordCount)
	hash, _, _ := logger.LastHashForDate(today)
	assert.Equal(t, hash, anchors[0].ChainHash)
}

func TestMaybeCreateAnchorSkipsIfUnchanged(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)
	logger.Log(Entry{Task: "task-0", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	created, _ := MaybeCreateAnchor(logger, "")
	assert.True(t, created)
	created, _ = MaybeCreateAnchor(logger, "")
	assert.False(t, created, "should skip when chain_hash unchanged")
}

func TestMaybeCreateAnchorUpdatesOnNewEntries(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)
	logger.Log(Entry{Task: "task-0", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	MaybeCreateAnchor(logger, "")
	logger.Log(Entry{Task: "task-1", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	created, _ := MaybeCreateAnchor(logger, "")
	assert.True(t, created, "should update when new entries exist")
	anchors, _ := LoadAnchors(dir)
	require.Len(t, anchors, 1, "should replace, not duplicate")
	assert.Equal(t, 2, anchors[0].RecordCount)
}

func TestMaybeCreateAnchorNoEntries(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)
	created, err := MaybeCreateAnchor(logger, "")
	require.NoError(t, err)
	assert.False(t, created)
}

func TestCreateGitTag(t *testing.T) {
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = dir
		out, err := c.CombinedOutput()
		require.NoError(t, err, "git cmd %v failed: %s", args, out)
	}
	ok := createGitTag(dir, "apex-audit-anchor-2026-02-19", "abc123", 5)
	assert.True(t, ok)
	c := exec.Command("git", "tag", "-l", "apex-audit-anchor-2026-02-19")
	c.Dir = dir
	out, err := c.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "apex-audit-anchor-2026-02-19")
}

func TestCreateGitTagNotGitRepo(t *testing.T) {
	dir := t.TempDir()
	ok := createGitTag(dir, "test-tag", "hash", 1)
	assert.False(t, ok)
}

func TestCreateGitTagForceUpdate(t *testing.T) {
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = dir
		c.CombinedOutput()
	}
	createGitTag(dir, "apex-audit-anchor-2026-02-19", "hash1", 3)
	ok := createGitTag(dir, "apex-audit-anchor-2026-02-19", "hash2", 5)
	assert.True(t, ok)
	c := exec.Command("git", "tag", "-l", "-n1", "apex-audit-anchor-2026-02-19")
	c.Dir = dir
	out, _ := c.Output()
	assert.Contains(t, string(out), "hash2")
}

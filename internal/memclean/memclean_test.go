package memclean

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, 0.8, cfg.CapacityThreshold)
	assert.Equal(t, 1000, cfg.MaxEntries)
	assert.Equal(t, 0.3, cfg.ConfidenceMin)
	assert.Equal(t, 30, cfg.StaleAfterDays)
	assert.Equal(t, []string{"decisions", "preferences"}, cfg.ExemptCategories)
}

// seedMemDir creates a temp directory with .md and .jsonl files across categories.
func seedMemDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// decisions/arch.md
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "decisions"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "decisions", "arch.md"),
		[]byte("# Architecture Decision\nChose microservices.\n"),
		0644,
	))

	// facts/golang.md
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "facts"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "facts", "golang.md"),
		[]byte("# Go Version\nProject uses Go 1.25.\n"),
		0644,
	))

	// sessions/2026-01.jsonl
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sessions"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "sessions", "2026-01.jsonl"),
		[]byte(`{"task":"init","result":"ok"}`+"\n"),
		0644,
	))

	return dir
}

func TestScan(t *testing.T) {
	dir := seedMemDir(t)

	entries, err := Scan(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 3)

	// Build a map of category -> entry for easier assertion.
	byCategory := map[string]MemoryEntry{}
	for _, e := range entries {
		byCategory[e.Category] = e
	}

	// decisions/arch.md
	dec, ok := byCategory["decisions"]
	require.True(t, ok, "should have decisions entry")
	assert.Equal(t, filepath.Join("decisions", "arch.md"), dec.Path)
	assert.Equal(t, "decisions", dec.Category)
	assert.Greater(t, dec.Size, int64(0))
	assert.False(t, dec.ModTime.IsZero())
	assert.Equal(t, 0.5, dec.Confidence)

	// facts/golang.md
	fact, ok := byCategory["facts"]
	require.True(t, ok, "should have facts entry")
	assert.Equal(t, filepath.Join("facts", "golang.md"), fact.Path)
	assert.Equal(t, "facts", fact.Category)

	// sessions/2026-01.jsonl
	sess, ok := byCategory["sessions"]
	require.True(t, ok, "should have sessions entry")
	assert.Equal(t, filepath.Join("sessions", "2026-01.jsonl"), sess.Path)
	assert.Equal(t, "sessions", sess.Category)
}

func TestEvaluateUnderThreshold(t *testing.T) {
	// 3 entries with MaxEntries=1000 → 3 < 800 → nothing to remove.
	entries := []MemoryEntry{
		{Path: "facts/a.md", Category: "facts", Confidence: 0.1, ModTime: time.Now().AddDate(0, 0, -60)},
		{Path: "facts/b.md", Category: "facts", Confidence: 0.1, ModTime: time.Now().AddDate(0, 0, -60)},
		{Path: "facts/c.md", Category: "facts", Confidence: 0.1, ModTime: time.Now().AddDate(0, 0, -60)},
	}
	cfg := DefaultConfig()

	toRemove, toKeep := Evaluate(entries, cfg, time.Now())

	assert.Empty(t, toRemove)
	assert.Len(t, toKeep, 3)
}

func TestEvaluateOverThreshold(t *testing.T) {
	now := time.Now()
	staleTime := now.AddDate(0, 0, -60) // 60 days ago, well beyond 30-day threshold
	recentTime := now.AddDate(0, 0, -5) // 5 days ago, recent

	// Create a config with low MaxEntries so we exceed the threshold easily.
	cfg := CleanupConfig{
		CapacityThreshold: 0.8,
		MaxEntries:        5, // threshold = 5 * 0.8 = 4
		ConfidenceMin:     0.3,
		StaleAfterDays:    30,
		ExemptCategories:  []string{"decisions", "preferences"},
	}

	entries := []MemoryEntry{
		{Path: "facts/stale-low.md", Category: "facts", Confidence: 0.1, ModTime: staleTime},      // stale + low confidence → remove
		{Path: "facts/stale-high.md", Category: "facts", Confidence: 0.8, ModTime: staleTime},      // stale but high confidence → keep
		{Path: "facts/recent-low.md", Category: "facts", Confidence: 0.1, ModTime: recentTime},     // low confidence but recent → keep
		{Path: "facts/recent-high.md", Category: "facts", Confidence: 0.9, ModTime: recentTime},    // recent + high confidence → keep
		{Path: "sessions/old.jsonl", Category: "sessions", Confidence: 0.2, ModTime: staleTime},    // stale + low confidence → remove
	}

	toRemove, toKeep := Evaluate(entries, cfg, now)

	assert.Len(t, toRemove, 2)
	assert.Len(t, toKeep, 3)

	removedPaths := map[string]bool{}
	for _, e := range toRemove {
		removedPaths[e.Path] = true
	}
	assert.True(t, removedPaths["facts/stale-low.md"])
	assert.True(t, removedPaths["sessions/old.jsonl"])
}

func TestEvaluateExempt(t *testing.T) {
	now := time.Now()
	staleTime := now.AddDate(0, 0, -60)

	cfg := CleanupConfig{
		CapacityThreshold: 0.8,
		MaxEntries:        5,
		ConfidenceMin:     0.3,
		StaleAfterDays:    30,
		ExemptCategories:  []string{"decisions", "preferences"},
	}

	entries := []MemoryEntry{
		{Path: "decisions/old.md", Category: "decisions", Confidence: 0.1, ModTime: staleTime},       // exempt: decisions
		{Path: "preferences/theme.md", Category: "preferences", Confidence: 0.1, ModTime: staleTime}, // exempt: preferences
		{Path: "facts/stale.md", Category: "facts", Confidence: 0.1, ModTime: staleTime},             // not exempt → remove
		{Path: "facts/keep.md", Category: "facts", Confidence: 0.9, ModTime: now},                    // keep (high confidence, recent)
		{Path: "sessions/old.jsonl", Category: "sessions", Confidence: 0.2, ModTime: staleTime},      // not exempt → remove
	}

	toRemove, toKeep := Evaluate(entries, cfg, now)

	// Only facts/stale.md and sessions/old.jsonl should be removed.
	assert.Len(t, toRemove, 2)
	assert.Len(t, toKeep, 3)

	// Verify exempt categories are never in toRemove.
	for _, e := range toRemove {
		assert.NotEqual(t, "decisions", e.Category)
		assert.NotEqual(t, "preferences", e.Category)
	}

	// Verify exempt entries are in toKeep.
	keptPaths := map[string]bool{}
	for _, e := range toKeep {
		keptPaths[e.Path] = true
	}
	assert.True(t, keptPaths["decisions/old.md"])
	assert.True(t, keptPaths["preferences/theme.md"])
}

func TestExecute(t *testing.T) {
	dir := seedMemDir(t)

	// Verify files exist before execution.
	_, err := os.Stat(filepath.Join(dir, "facts", "golang.md"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "sessions", "2026-01.jsonl"))
	require.NoError(t, err)

	toRemove := []MemoryEntry{
		{Path: filepath.Join("facts", "golang.md")},
		{Path: filepath.Join("sessions", "2026-01.jsonl")},
	}

	result, removed, err := Execute(dir, toRemove)
	require.NoError(t, err)

	assert.Equal(t, 2, removed)
	assert.Equal(t, 2, result.Removed)

	// Files should be gone.
	_, err = os.Stat(filepath.Join(dir, "facts", "golang.md"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(dir, "sessions", "2026-01.jsonl"))
	assert.True(t, os.IsNotExist(err))

	// decisions/arch.md should still exist.
	_, err = os.Stat(filepath.Join(dir, "decisions", "arch.md"))
	assert.NoError(t, err)
}

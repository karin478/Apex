package dashboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDir creates a temporary base directory with the standard
// subdirectory layout expected by the dashboard subsystems.
func setupTestDir(t *testing.T) string {
	t.Helper()
	base := t.TempDir()

	dirs := []string{
		"audit",
		"runs",
		"memory/decisions",
		"memory/facts",
		"memory/sessions",
	}
	for _, d := range dirs {
		require.NoError(t, os.MkdirAll(filepath.Join(base, d), 0o755))
	}
	return base
}

func TestGenerateEmpty(t *testing.T) {
	base := setupTestDir(t)
	d := New(base)

	sections, err := d.Generate()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(sections), 3, "should produce at least 3 sections")
}

func TestGenerateHealthSection(t *testing.T) {
	base := setupTestDir(t)
	d := New(base)

	sections, err := d.Generate()
	require.NoError(t, err)
	require.NotEmpty(t, sections)

	assert.Equal(t, "System Health", sections[0].Title)
	assert.Contains(t, sections[0].Content, "Level:")
}

func TestGenerateRunsEmpty(t *testing.T) {
	base := setupTestDir(t)
	d := New(base)

	sections, err := d.Generate()
	require.NoError(t, err)

	// Find the runs section (index 1).
	require.True(t, len(sections) >= 2, "should have at least 2 sections")
	runsSection := sections[1]
	assert.Equal(t, "Recent Runs", runsSection.Title)
	assert.Contains(t, runsSection.Content, "No runs recorded")
}

func TestGenerateWithManifest(t *testing.T) {
	base := setupTestDir(t)

	// Write a fake manifest JSON.
	m := map[string]interface{}{
		"run_id":      "run-001",
		"task":        "deploy-api",
		"timestamp":   "2025-06-15T10:00:00Z",
		"model":       "claude-opus-4-6",
		"effort":      "high",
		"risk_level":  "medium",
		"node_count":  3,
		"duration_ms": 1500,
		"outcome":     "success",
		"nodes": []map[string]interface{}{
			{"id": "n1", "task": "build", "status": "success"},
			{"id": "n2", "task": "test", "status": "success"},
			{"id": "n3", "task": "deploy", "status": "success"},
		},
	}
	data, err := json.MarshalIndent(m, "", "  ")
	require.NoError(t, err)

	runDir := filepath.Join(base, "runs", "run-001")
	require.NoError(t, os.MkdirAll(runDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(runDir, "manifest.json"), data, 0o644))

	d := New(base)
	sections, err := d.Generate()
	require.NoError(t, err)

	// Verify runs section contains the run ID and task.
	require.True(t, len(sections) >= 2, "should have at least 2 sections")
	runsSection := sections[1]
	assert.Equal(t, "Recent Runs", runsSection.Title)
	assert.Contains(t, runsSection.Content, "run-001")
	assert.Contains(t, runsSection.Content, "deploy-api")

	// Verify metrics section shows total runs count.
	require.True(t, len(sections) >= 3, "should have at least 3 sections")
	metricsSection := sections[2]
	assert.Equal(t, "Metrics Summary", metricsSection.Title)
	assert.Contains(t, metricsSection.Content, "Total runs: 1")
}

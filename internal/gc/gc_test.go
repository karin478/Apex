package gc

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeManifest(t *testing.T, runsDir, runID string, ts time.Time) {
	t.Helper()
	runDir := filepath.Join(runsDir, runID)
	require.NoError(t, os.MkdirAll(runDir, 0755))
	manifest := `{"run_id":"` + runID + `","timestamp":"` + ts.Format(time.RFC3339) + `","outcome":"success"}`
	require.NoError(t, os.WriteFile(filepath.Join(runDir, "manifest.json"), []byte(manifest), 0644))
}

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	assert.Equal(t, 30, p.MaxAgeDays)
	assert.Equal(t, 100, p.MaxRuns)
	assert.Equal(t, 90, p.MaxAuditDays)
	assert.False(t, p.DryRun)
}

func TestRunCleanupByAge(t *testing.T) {
	dir := t.TempDir()
	runsDir := filepath.Join(dir, "runs")
	os.MkdirAll(filepath.Join(dir, "audit"), 0755)

	now := time.Now()
	writeManifest(t, runsDir, "recent", now)
	writeManifest(t, runsDir, "old-1", now.AddDate(0, 0, -60))
	writeManifest(t, runsDir, "old-2", now.AddDate(0, 0, -90))

	policy := Policy{MaxAgeDays: 30, MaxRuns: 100, MaxAuditDays: 90}
	result, err := Run(dir, policy)
	require.NoError(t, err)
	assert.Equal(t, 2, result.RunsRemoved)
	assert.DirExists(t, filepath.Join(runsDir, "recent"))
	assert.NoDirExists(t, filepath.Join(runsDir, "old-1"))
	assert.NoDirExists(t, filepath.Join(runsDir, "old-2"))
}

func TestRunCleanupByCount(t *testing.T) {
	dir := t.TempDir()
	runsDir := filepath.Join(dir, "runs")
	os.MkdirAll(filepath.Join(dir, "audit"), 0755)

	now := time.Now()
	for i := 0; i < 5; i++ {
		writeManifest(t, runsDir, "run-"+string(rune('a'+i)), now.Add(time.Duration(i)*time.Second))
	}

	policy := Policy{MaxAgeDays: 365, MaxRuns: 2, MaxAuditDays: 90}
	result, err := Run(dir, policy)
	require.NoError(t, err)
	assert.Equal(t, 3, result.RunsRemoved)
}

func TestAuditCleanup(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "runs"), 0755)
	auditDir := filepath.Join(dir, "audit")
	os.MkdirAll(auditDir, 0755)

	// Create old and new audit files
	today := time.Now().Format("2006-01-02")
	old := time.Now().AddDate(0, 0, -100).Format("2006-01-02")
	os.WriteFile(filepath.Join(auditDir, today+".jsonl"), []byte(`{"test":"new"}`+"\n"), 0644)
	os.WriteFile(filepath.Join(auditDir, old+".jsonl"), []byte(`{"test":"old"}`+"\n"), 0644)

	policy := Policy{MaxAgeDays: 30, MaxRuns: 100, MaxAuditDays: 90}
	result, err := Run(dir, policy)
	require.NoError(t, err)
	assert.Equal(t, 1, result.AuditFilesRemoved)
	assert.FileExists(t, filepath.Join(auditDir, today+".jsonl"))
}

func TestDryRun(t *testing.T) {
	dir := t.TempDir()
	runsDir := filepath.Join(dir, "runs")
	os.MkdirAll(filepath.Join(dir, "audit"), 0755)

	now := time.Now()
	writeManifest(t, runsDir, "old", now.AddDate(0, 0, -60))

	policy := Policy{MaxAgeDays: 30, MaxRuns: 100, MaxAuditDays: 90, DryRun: true}
	result, err := Run(dir, policy)
	require.NoError(t, err)
	assert.Equal(t, 1, result.RunsRemoved)
	// Directory should still exist in dry-run
	assert.DirExists(t, filepath.Join(runsDir, "old"))
}

func TestEmptyDir(t *testing.T) {
	dir := t.TempDir()
	// Don't create subdirs — should handle gracefully
	policy := DefaultPolicy()
	result, err := Run(dir, policy)
	require.NoError(t, err)
	assert.Equal(t, 0, result.RunsRemoved)
	assert.Equal(t, 0, result.AuditFilesRemoved)
}

package health

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{GREEN, "GREEN"},
		{YELLOW, "YELLOW"},
		{RED, "RED"},
		{CRITICAL, "CRITICAL"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.expected, tc.level.String(), "Level %d should stringify correctly", tc.level)
	}
}

func TestDetermineAllHealthy(t *testing.T) {
	components := []ComponentStatus{
		{Name: "audit_chain", Category: "critical", Healthy: true, Detail: "ok"},
		{Name: "sandbox_available", Category: "important", Healthy: true, Detail: "ok"},
		{Name: "metrics", Category: "optional", Healthy: true, Detail: "ok"},
	}
	assert.Equal(t, GREEN, Determine(components))
}

func TestDetermineOneImportantFailed(t *testing.T) {
	components := []ComponentStatus{
		{Name: "audit_chain", Category: "critical", Healthy: true, Detail: "ok"},
		{Name: "sandbox_available", Category: "important", Healthy: false, Detail: "sandbox unavailable"},
		{Name: "metrics", Category: "optional", Healthy: true, Detail: "ok"},
	}
	assert.Equal(t, YELLOW, Determine(components))
}

func TestDetermineTwoImportantFailed(t *testing.T) {
	components := []ComponentStatus{
		{Name: "audit_chain", Category: "critical", Healthy: true, Detail: "ok"},
		{Name: "sandbox_available", Category: "important", Healthy: false, Detail: "sandbox unavailable"},
		{Name: "rate_limiter", Category: "important", Healthy: false, Detail: "rate limiter down"},
		{Name: "metrics", Category: "optional", Healthy: true, Detail: "ok"},
	}
	assert.Equal(t, RED, Determine(components))
}

func TestDetermineOneCriticalFailed(t *testing.T) {
	components := []ComponentStatus{
		{Name: "audit_chain", Category: "critical", Healthy: false, Detail: "chain broken"},
		{Name: "sandbox_available", Category: "important", Healthy: true, Detail: "ok"},
		{Name: "metrics", Category: "optional", Healthy: true, Detail: "ok"},
	}
	assert.Equal(t, RED, Determine(components))
}

func TestDetermineTwoCriticalFailed(t *testing.T) {
	components := []ComponentStatus{
		{Name: "audit_chain", Category: "critical", Healthy: false, Detail: "chain broken"},
		{Name: "governance_engine", Category: "critical", Healthy: false, Detail: "engine down"},
		{Name: "sandbox_available", Category: "important", Healthy: true, Detail: "ok"},
	}
	assert.Equal(t, CRITICAL, Determine(components))
}

func TestDetermineMixedFailures(t *testing.T) {
	components := []ComponentStatus{
		{Name: "audit_chain", Category: "critical", Healthy: false, Detail: "chain broken"},
		{Name: "sandbox_available", Category: "important", Healthy: false, Detail: "sandbox unavailable"},
		{Name: "metrics", Category: "optional", Healthy: true, Detail: "ok"},
	}
	// 1 critical + 1 important → RED (critical takes precedence, not CRITICAL since only 1 critical)
	assert.Equal(t, RED, Determine(components))
}

func TestDetermineOptionalIgnored(t *testing.T) {
	components := []ComponentStatus{
		{Name: "audit_chain", Category: "critical", Healthy: true, Detail: "ok"},
		{Name: "sandbox_available", Category: "important", Healthy: true, Detail: "ok"},
		{Name: "metrics", Category: "optional", Healthy: false, Detail: "metrics down"},
		{Name: "logging", Category: "optional", Healthy: false, Detail: "logging down"},
		{Name: "tracing", Category: "optional", Healthy: false, Detail: "tracing down"},
	}
	// Optional failures should not affect the level
	assert.Equal(t, GREEN, Determine(components))
}

func TestNewReport(t *testing.T) {
	components := []ComponentStatus{
		{Name: "audit_chain", Category: "critical", Healthy: false, Detail: "chain broken"},
		{Name: "sandbox_available", Category: "important", Healthy: true, Detail: "ok"},
	}
	report := NewReport(components)
	assert.NotNil(t, report)
	assert.Equal(t, Determine(components), report.Level)
	assert.Equal(t, components, report.Components)
}

// ---------------------------------------------------------------------------
// Component check tests (Task 2)
// ---------------------------------------------------------------------------

func TestCheckAuditChainHealthy(t *testing.T) {
	// Empty audit dir with no entries — empty chain is valid
	base := t.TempDir()
	auditDir := filepath.Join(base, "audit")
	require.NoError(t, os.MkdirAll(auditDir, 0755))

	cs := CheckAuditChain(base)
	assert.True(t, cs.Healthy, "empty audit chain should be healthy")
	assert.Equal(t, "audit_chain", cs.Name)
	assert.Equal(t, "critical", cs.Category)
	assert.Equal(t, "Hash chain intact", cs.Detail)
}

func TestCheckAuditChainNoDir(t *testing.T) {
	// Use a file as the base so that MkdirAll fails (can't create dir under a file)
	base := t.TempDir()
	blocker := filepath.Join(base, "audit")
	require.NoError(t, os.WriteFile(blocker, []byte("not-a-dir"), 0644))

	cs := CheckAuditChain(base)
	assert.False(t, cs.Healthy, "non-directory audit path should be unhealthy")
	assert.Equal(t, "audit_chain", cs.Name)
	assert.Equal(t, "critical", cs.Category)
}

func TestCheckConfigValid(t *testing.T) {
	base := t.TempDir()
	cfgPath := filepath.Join(base, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("claude:\n  model: claude-opus-4-6\n"), 0644))

	cs := CheckConfig(base)
	assert.True(t, cs.Healthy, "valid config should be healthy")
	assert.Equal(t, "config", cs.Name)
	assert.Equal(t, "critical", cs.Category)
	assert.Equal(t, "Configuration loaded", cs.Detail)
}

func TestCheckConfigMissing(t *testing.T) {
	// config.Load returns defaults for missing file — so this should be healthy
	base := t.TempDir()
	cs := CheckConfig(base)
	assert.True(t, cs.Healthy, "missing config should be healthy (defaults used)")
	assert.Equal(t, "Configuration loaded", cs.Detail)
}

func TestCheckKillSwitchInactive(t *testing.T) {
	// Create a temp home dir with no KILL_SWITCH file
	tmpHome := t.TempDir()
	claudeDir := filepath.Join(tmpHome, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	cs := checkKillSwitchAt(filepath.Join(claudeDir, "KILL_SWITCH"))
	assert.True(t, cs.Healthy, "no kill switch file should be healthy")
	assert.Equal(t, "kill_switch", cs.Name)
	assert.Equal(t, "important", cs.Category)
	assert.Equal(t, "Not active", cs.Detail)
}

func TestCheckKillSwitchActive(t *testing.T) {
	tmpHome := t.TempDir()
	claudeDir := filepath.Join(tmpHome, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	ksPath := filepath.Join(claudeDir, "KILL_SWITCH")
	require.NoError(t, os.WriteFile(ksPath, []byte("emergency"), 0644))

	cs := checkKillSwitchAt(ksPath)
	assert.False(t, cs.Healthy, "active kill switch should be unhealthy")
	assert.Equal(t, "ACTIVE — use 'apex resume' to deactivate", cs.Detail)
}

func TestCheckDirWritable(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "audit")
	require.NoError(t, os.MkdirAll(dir, 0755))

	cs := CheckAuditDir(base)
	assert.True(t, cs.Healthy, "writable dir should be healthy")
	assert.Equal(t, "audit_dir", cs.Name)
	assert.Equal(t, "important", cs.Category)
	assert.Equal(t, "Writable", cs.Detail)
}

func TestCheckDirMissing(t *testing.T) {
	base := filepath.Join(t.TempDir(), "does-not-exist")

	cs := CheckAuditDir(base)
	assert.False(t, cs.Healthy, "missing dir should be unhealthy")
	assert.Equal(t, "Missing", cs.Detail)
}

func TestEvaluateAllHealthy(t *testing.T) {
	base := t.TempDir()
	// Create audit dir
	require.NoError(t, os.MkdirAll(filepath.Join(base, "audit"), 0755))
	// Create memory dir
	require.NoError(t, os.MkdirAll(filepath.Join(base, "memory"), 0755))
	// Write a valid config
	require.NoError(t, os.WriteFile(filepath.Join(base, "config.yaml"), []byte("claude:\n  model: test\n"), 0644))

	report := Evaluate(base)
	assert.NotNil(t, report)
	assert.Equal(t, GREEN, report.Level, "all healthy should be GREEN, components: %+v", report.Components)
}

func TestEvaluateWithDegradation(t *testing.T) {
	base := t.TempDir()
	// Create audit dir but corrupt it by writing an invalid jsonl entry
	auditDir := filepath.Join(base, "audit")
	require.NoError(t, os.MkdirAll(auditDir, 0755))
	// Write corrupted audit file — invalid JSON will cause Verify to return error
	require.NoError(t, os.WriteFile(
		filepath.Join(auditDir, "2025-01-01.jsonl"),
		[]byte("not-valid-json\n"),
		0644,
	))
	// Create memory dir
	require.NoError(t, os.MkdirAll(filepath.Join(base, "memory"), 0755))
	// Write valid config
	require.NoError(t, os.WriteFile(filepath.Join(base, "config.yaml"), []byte("claude:\n  model: test\n"), 0644))

	report := Evaluate(base)
	assert.NotNil(t, report)
	// audit_chain is critical and will be unhealthy → at least RED
	assert.True(t, report.Level >= RED, "corrupted audit should cause degradation, got %s", report.Level.String())
}

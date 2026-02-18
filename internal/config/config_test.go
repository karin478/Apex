package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()

	assert.Equal(t, "claude-opus-4-6", cfg.Claude.Model)
	assert.Equal(t, "high", cfg.Claude.Effort)
	assert.Equal(t, 600, cfg.Claude.Timeout)
	assert.Equal(t, 1800, cfg.Claude.LongTaskTimeout)
	assert.Contains(t, cfg.Governance.AutoApprove, "LOW")
	assert.Contains(t, cfg.Governance.Confirm, "MEDIUM")
	assert.Contains(t, cfg.Governance.Reject, "HIGH")
	assert.Contains(t, cfg.Governance.Reject, "CRITICAL")
}

func TestLoadConfigFromFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	content := []byte(`claude:
  model: claude-sonnet-4-6
  timeout: 900
`)
	require.NoError(t, os.WriteFile(configPath, content, 0644))

	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Equal(t, "claude-sonnet-4-6", cfg.Claude.Model)
	assert.Equal(t, 900, cfg.Claude.Timeout)
	// Defaults preserved for unset fields
	assert.Equal(t, "high", cfg.Claude.Effort)
	assert.Equal(t, 1800, cfg.Claude.LongTaskTimeout)
}

func TestLoadConfigFileNotFound(t *testing.T) {
	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err, "missing config file should return defaults, not error")
	assert.Equal(t, "claude-opus-4-6", cfg.Claude.Model)
}

func TestApexDir(t *testing.T) {
	cfg := Default()
	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, ".apex"), cfg.ApexDir())
}

func TestEnsureDirs(t *testing.T) {
	dir := t.TempDir()
	cfg := Default()
	cfg.BaseDir = dir

	require.NoError(t, cfg.EnsureDirs())

	assert.DirExists(t, filepath.Join(dir, "memory", "decisions"))
	assert.DirExists(t, filepath.Join(dir, "memory", "facts"))
	assert.DirExists(t, filepath.Join(dir, "memory", "sessions"))
	assert.DirExists(t, filepath.Join(dir, "audit"))
}

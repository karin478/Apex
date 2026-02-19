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
	assert.Equal(t, 1800, cfg.Claude.Timeout)
	assert.Equal(t, 7200, cfg.Claude.LongTaskTimeout)
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
	assert.Equal(t, 7200, cfg.Claude.LongTaskTimeout)
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

func TestDefaultConfigPhase2(t *testing.T) {
	cfg := Default()

	// Updated timeouts
	assert.Equal(t, 1800, cfg.Claude.Timeout)
	assert.Equal(t, 7200, cfg.Claude.LongTaskTimeout)

	// New planner config
	assert.Equal(t, "claude-opus-4-6", cfg.Planner.Model)
	assert.Equal(t, 120, cfg.Planner.Timeout)

	// New pool config
	assert.Equal(t, 4, cfg.Pool.MaxConcurrent)
}

func TestLoadConfigPhase2Override(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	content := []byte(`planner:
  model: claude-sonnet-4-6
  timeout: 60
pool:
  max_concurrent: 2
`)
	require.NoError(t, os.WriteFile(configPath, content, 0644))

	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Equal(t, "claude-sonnet-4-6", cfg.Planner.Model)
	assert.Equal(t, 60, cfg.Planner.Timeout)
	assert.Equal(t, 2, cfg.Pool.MaxConcurrent)
}

func TestDefaultConfigPhase3(t *testing.T) {
	cfg := Default()
	assert.Equal(t, "text-embedding-3-small", cfg.Embedding.Model)
	assert.Equal(t, "OPENAI_API_KEY", cfg.Embedding.APIKeyEnv)
	assert.Equal(t, 1536, cfg.Embedding.Dimensions)
}

func TestLoadConfigPhase3Override(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`embedding:
  model: text-embedding-3-large
  api_key_env: MY_OPENAI_KEY
  dimensions: 3072
`)
	require.NoError(t, os.WriteFile(configPath, content, 0644))
	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "text-embedding-3-large", cfg.Embedding.Model)
	assert.Equal(t, "MY_OPENAI_KEY", cfg.Embedding.APIKeyEnv)
	assert.Equal(t, 3072, cfg.Embedding.Dimensions)
}

func TestDefaultConfigPhase4(t *testing.T) {
	cfg := Default()
	assert.Equal(t, 60000, cfg.Context.TokenBudget)
}

func TestLoadConfigPhase4Override(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`context:
  token_budget: 30000
`)
	require.NoError(t, os.WriteFile(configPath, content, 0644))
	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, 30000, cfg.Context.TokenBudget)
}

func TestDefaultConfigPhase10(t *testing.T) {
	cfg := Default()
	assert.Equal(t, 3, cfg.Retry.MaxAttempts)
	assert.Equal(t, 2, cfg.Retry.InitDelaySeconds)
	assert.Equal(t, 2.0, cfg.Retry.Multiplier)
	assert.Equal(t, 30, cfg.Retry.MaxDelaySeconds)
}

func TestLoadConfigPhase10Override(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`retry:
  max_attempts: 5
  init_delay_seconds: 1
  multiplier: 3.0
  max_delay_seconds: 60
`)
	require.NoError(t, os.WriteFile(configPath, content, 0644))
	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, 5, cfg.Retry.MaxAttempts)
	assert.Equal(t, 1, cfg.Retry.InitDelaySeconds)
	assert.Equal(t, 3.0, cfg.Retry.Multiplier)
	assert.Equal(t, 60, cfg.Retry.MaxDelaySeconds)
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

func TestDefaultSandboxConfig(t *testing.T) {
	cfg := Default()
	assert.Equal(t, "auto", cfg.Sandbox.Level)
	assert.Equal(t, "ubuntu:22.04", cfg.Sandbox.DockerImage)
	assert.Equal(t, "2g", cfg.Sandbox.MemoryLimit)
	assert.Equal(t, "2", cfg.Sandbox.CPULimit)
	assert.Equal(t, 100, cfg.Sandbox.MaxFileSizeMB)
	assert.Equal(t, 300, cfg.Sandbox.MaxCPUSeconds)
}

func TestDefaultRedactionConfig(t *testing.T) {
	cfg := Default()
	assert.True(t, cfg.Redaction.Enabled)
	assert.Equal(t, "private_only", cfg.Redaction.RedactIPs)
	assert.Equal(t, "[REDACTED]", cfg.Redaction.Placeholder)
}

func TestLoadRedactionConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`redaction:
  enabled: false
  redact_ips: all
  placeholder: "***"
  custom_patterns:
    - "secret-\\d+"
`)
	require.NoError(t, os.WriteFile(configPath, content, 0644))
	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.False(t, cfg.Redaction.Enabled)
	assert.Equal(t, "all", cfg.Redaction.RedactIPs)
	assert.Equal(t, "***", cfg.Redaction.Placeholder)
	assert.Equal(t, []string{"secret-\\d+"}, cfg.Redaction.CustomPatterns)
}

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/redact"
	"gopkg.in/yaml.v3"
)

type ClaudeConfig struct {
	Model          string `yaml:"model"`
	Effort         string `yaml:"effort"`
	Timeout        int    `yaml:"timeout"`
	LongTaskTimeout int   `yaml:"long_task_timeout"`
	Binary         string `yaml:"binary"`
	PermissionMode string `yaml:"permission_mode"`
}

type GovernanceConfig struct {
	AutoApprove []string `yaml:"auto_approve"`
	Confirm     []string `yaml:"confirm"`
	Reject      []string `yaml:"reject"`
}

type PlannerConfig struct {
	Model   string `yaml:"model"`
	Timeout int    `yaml:"timeout"`
}

type PoolConfig struct {
	MaxConcurrent int `yaml:"max_concurrent"`
}

type EmbeddingConfig struct {
	Model      string `yaml:"model"`
	APIKeyEnv  string `yaml:"api_key_env"`
	Dimensions int    `yaml:"dimensions"`
}

type ContextConfig struct {
	TokenBudget int `yaml:"token_budget"`
}

type RetryConfig struct {
	MaxAttempts      int     `yaml:"max_attempts"`
	InitDelaySeconds int     `yaml:"init_delay_seconds"`
	Multiplier       float64 `yaml:"multiplier"`
	MaxDelaySeconds  int     `yaml:"max_delay_seconds"`
}

type SandboxConfig struct {
	Level         string   `yaml:"level"`            // "auto", "docker", "ulimit", "none"
	RequireFor    []string `yaml:"require_for"`      // risk levels requiring sandbox, e.g. ["HIGH","CRITICAL"]
	DockerImage   string   `yaml:"docker_image"`
	MemoryLimit   string   `yaml:"memory_limit"`
	CPULimit      string   `yaml:"cpu_limit"`
	MaxFileSizeMB int      `yaml:"max_file_size_mb"`
	MaxCPUSeconds int      `yaml:"max_cpu_seconds"`
}

type Config struct {
	Claude     ClaudeConfig           `yaml:"claude"`
	Governance GovernanceConfig       `yaml:"governance"`
	Planner    PlannerConfig          `yaml:"planner"`
	Pool       PoolConfig             `yaml:"pool"`
	Embedding  EmbeddingConfig        `yaml:"embedding"`
	Context    ContextConfig          `yaml:"context"`
	Retry      RetryConfig            `yaml:"retry"`
	Sandbox    SandboxConfig          `yaml:"sandbox"`
	Redaction  redact.RedactionConfig `yaml:"redaction"`
	BaseDir    string                 `yaml:"-"`
}

func Default() *Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	return &Config{
		Claude: ClaudeConfig{
			Model:           "claude-opus-4-6",
			Effort:          "high",
			Timeout:         1800,
			LongTaskTimeout: 7200,
		},
		Governance: GovernanceConfig{
			AutoApprove: []string{"LOW"},
			Confirm:     []string{"MEDIUM"},
			Reject:      []string{"HIGH", "CRITICAL"},
		},
		Planner: PlannerConfig{
			Model:   "claude-opus-4-6",
			Timeout: 120,
		},
		Pool: PoolConfig{
			MaxConcurrent: 4,
		},
		Embedding: EmbeddingConfig{
			Model:      "text-embedding-3-small",
			APIKeyEnv:  "OPENAI_API_KEY",
			Dimensions: 1536,
		},
		Context: ContextConfig{
			TokenBudget: 60000,
		},
		Retry: RetryConfig{
			MaxAttempts:      3,
			InitDelaySeconds: 2,
			Multiplier:       2.0,
			MaxDelaySeconds:  30,
		},
		Sandbox: SandboxConfig{
			Level:         "auto",
			DockerImage:   "ubuntu:22.04",
			MemoryLimit:   "2g",
			CPULimit:      "2",
			MaxFileSizeMB: 100,
			MaxCPUSeconds: 300,
		},
		Redaction: redact.RedactionConfig{
			Enabled:     true,
			RedactIPs:   "private_only",
			Placeholder: "[REDACTED]",
		},
		BaseDir: filepath.Join(home, ".apex"),
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Ensure defaults for zero values
	if cfg.Claude.Model == "" {
		cfg.Claude.Model = "claude-opus-4-6"
	}
	if cfg.Claude.Effort == "" {
		cfg.Claude.Effort = "high"
	}
	if cfg.Claude.Timeout == 0 {
		cfg.Claude.Timeout = 1800
	}
	if cfg.Claude.LongTaskTimeout == 0 {
		cfg.Claude.LongTaskTimeout = 7200
	}
	if cfg.Planner.Model == "" {
		cfg.Planner.Model = "claude-opus-4-6"
	}
	if cfg.Planner.Timeout == 0 {
		cfg.Planner.Timeout = 120
	}
	if cfg.Pool.MaxConcurrent == 0 {
		cfg.Pool.MaxConcurrent = 4
	}
	if cfg.Embedding.Model == "" {
		cfg.Embedding.Model = "text-embedding-3-small"
	}
	if cfg.Embedding.APIKeyEnv == "" {
		cfg.Embedding.APIKeyEnv = "OPENAI_API_KEY"
	}
	if cfg.Embedding.Dimensions == 0 {
		cfg.Embedding.Dimensions = 1536
	}
	if cfg.Context.TokenBudget == 0 {
		cfg.Context.TokenBudget = 60000
	}
	if cfg.Retry.MaxAttempts == 0 {
		cfg.Retry.MaxAttempts = 3
	}
	if cfg.Retry.InitDelaySeconds == 0 {
		cfg.Retry.InitDelaySeconds = 2
	}
	if cfg.Retry.Multiplier == 0 {
		cfg.Retry.Multiplier = 2.0
	}
	if cfg.Retry.MaxDelaySeconds == 0 {
		cfg.Retry.MaxDelaySeconds = 30
	}
	if cfg.Sandbox.Level == "" {
		cfg.Sandbox.Level = "auto"
	}
	if cfg.Sandbox.DockerImage == "" {
		cfg.Sandbox.DockerImage = "ubuntu:22.04"
	}
	if cfg.Sandbox.MemoryLimit == "" {
		cfg.Sandbox.MemoryLimit = "2g"
	}
	if cfg.Sandbox.CPULimit == "" {
		cfg.Sandbox.CPULimit = "2"
	}
	if cfg.Sandbox.MaxFileSizeMB == 0 {
		cfg.Sandbox.MaxFileSizeMB = 100
	}
	if cfg.Sandbox.MaxCPUSeconds == 0 {
		cfg.Sandbox.MaxCPUSeconds = 300
	}
	if cfg.Redaction.Placeholder == "" {
		cfg.Redaction.Placeholder = "[REDACTED]"
	}
	if cfg.Redaction.RedactIPs == "" {
		cfg.Redaction.RedactIPs = "private_only"
	}
	if cfg.BaseDir == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			home = "/tmp"
		}
		cfg.BaseDir = filepath.Join(home, ".apex")
	}

	return cfg, nil
}

func (c *Config) ApexDir() string {
	return c.BaseDir
}

// Validate checks configuration values for sanity. Returns an error if any
// value is out of acceptable range.
func (c *Config) Validate() error {
	if c.Pool.MaxConcurrent < 1 || c.Pool.MaxConcurrent > 64 {
		return fmt.Errorf("pool.max_concurrent must be 1-64, got %d", c.Pool.MaxConcurrent)
	}
	if c.Retry.MaxAttempts < 1 || c.Retry.MaxAttempts > 20 {
		return fmt.Errorf("retry.max_attempts must be 1-20, got %d", c.Retry.MaxAttempts)
	}
	if c.Retry.Multiplier < 1.0 || c.Retry.Multiplier > 10.0 {
		return fmt.Errorf("retry.multiplier must be 1.0-10.0, got %.1f", c.Retry.Multiplier)
	}
	if c.Claude.Timeout < 10 || c.Claude.Timeout > 86400 {
		return fmt.Errorf("claude.timeout must be 10-86400, got %d", c.Claude.Timeout)
	}
	if c.Context.TokenBudget < 1000 || c.Context.TokenBudget > 1000000 {
		return fmt.Errorf("context.token_budget must be 1000-1000000, got %d", c.Context.TokenBudget)
	}
	validSandbox := map[string]bool{"auto": true, "docker": true, "ulimit": true, "none": true}
	if !validSandbox[c.Sandbox.Level] {
		return fmt.Errorf("sandbox.level must be auto/docker/ulimit/none, got %q", c.Sandbox.Level)
	}
	return nil
}

func (c *Config) EnsureDirs() error {
	dirs := []string{
		filepath.Join(c.BaseDir, "memory", "decisions"),
		filepath.Join(c.BaseDir, "memory", "facts"),
		filepath.Join(c.BaseDir, "memory", "sessions"),
		filepath.Join(c.BaseDir, "audit"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return nil
}

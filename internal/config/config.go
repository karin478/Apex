package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ClaudeConfig struct {
	Model           string `yaml:"model"`
	Effort          string `yaml:"effort"`
	Timeout         int    `yaml:"timeout"`
	LongTaskTimeout int    `yaml:"long_task_timeout"`
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

type Config struct {
	Claude     ClaudeConfig     `yaml:"claude"`
	Governance GovernanceConfig `yaml:"governance"`
	Planner    PlannerConfig    `yaml:"planner"`
	Pool       PoolConfig       `yaml:"pool"`
	BaseDir    string           `yaml:"-"`
}

func Default() *Config {
	home, _ := os.UserHomeDir()
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
	if cfg.BaseDir == "" {
		home, _ := os.UserHomeDir()
		cfg.BaseDir = filepath.Join(home, ".apex")
	}

	return cfg, nil
}

func (c *Config) ApexDir() string {
	return c.BaseDir
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

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

type Config struct {
	Claude     ClaudeConfig     `yaml:"claude"`
	Governance GovernanceConfig `yaml:"governance"`
	BaseDir    string           `yaml:"-"`
}

func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Claude: ClaudeConfig{
			Model:           "claude-opus-4-6",
			Effort:          "high",
			Timeout:         600,
			LongTaskTimeout: 1800,
		},
		Governance: GovernanceConfig{
			AutoApprove: []string{"LOW"},
			Confirm:     []string{"MEDIUM"},
			Reject:      []string{"HIGH", "CRITICAL"},
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
		cfg.Claude.Timeout = 600
	}
	if cfg.Claude.LongTaskTimeout == 0 {
		cfg.Claude.LongTaskTimeout = 1800
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

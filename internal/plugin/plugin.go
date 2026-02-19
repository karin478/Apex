package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ReasoningConfig struct {
	Protocol string   `yaml:"protocol"`
	Steps    int      `yaml:"steps"`
	Roles    []string `yaml:"roles"`
}

type Plugin struct {
	Name        string           `yaml:"name"`
	Version     string           `yaml:"version"`
	Type        string           `yaml:"type"`
	Description string           `yaml:"description"`
	Author      string           `yaml:"author"`
	Checksum    string           `yaml:"checksum"`
	Enabled     bool             `yaml:"enabled"`
	Reasoning   *ReasoningConfig `yaml:"reasoning,omitempty"`
	Dir         string           `yaml:"-"`
}

const pluginFile = "plugin.yaml"

func LoadPlugin(dir string) (*Plugin, error) {
	path := filepath.Join(dir, pluginFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read plugin.yaml: %w", err)
	}
	var p Plugin
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse plugin.yaml: %w", err)
	}
	p.Dir = dir
	return &p, nil
}

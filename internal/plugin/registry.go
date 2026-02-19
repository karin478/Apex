package plugin

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Registry struct {
	dir     string
	plugins []Plugin
}

func NewRegistry(pluginsDir string) *Registry {
	return &Registry{dir: pluginsDir}
}

func (r *Registry) Scan() ([]Plugin, error) {
	r.plugins = nil

	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(r.dir, entry.Name())
		p, err := LoadPlugin(fullPath)
		if err != nil {
			continue // skip invalid plugins
		}
		r.plugins = append(r.plugins, *p)
	}

	sort.Slice(r.plugins, func(i, j int) bool {
		return r.plugins[i].Name < r.plugins[j].Name
	})

	return r.plugins, nil
}

func (r *Registry) List() []Plugin {
	return r.plugins
}

func (r *Registry) Get(name string) (*Plugin, bool) {
	for i := range r.plugins {
		if r.plugins[i].Name == name {
			return &r.plugins[i], true
		}
	}
	return nil, false
}

var checksumLineRe = regexp.MustCompile(`(?m)^checksum:.*\n?`)

func (r *Registry) Enable(name string) error {
	return r.setEnabled(name, true)
}

func (r *Registry) Disable(name string) error {
	return r.setEnabled(name, false)
}

func (r *Registry) setEnabled(name string, enabled bool) error {
	p, ok := r.Get(name)
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	p.Enabled = enabled

	// Write back to plugin.yaml
	path := filepath.Join(p.Dir, pluginFile)
	data, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal plugin: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write plugin.yaml: %w", err)
	}
	return nil
}

func (r *Registry) Verify(name string) (bool, error) {
	p, ok := r.Get(name)
	if !ok {
		return false, fmt.Errorf("plugin %q not found", name)
	}
	if p.Checksum == "" {
		return false, nil
	}

	// Read raw file content
	path := filepath.Join(p.Dir, pluginFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	// Remove the checksum line and compute hash of the rest
	content := checksumLineRe.ReplaceAllString(string(data), "")
	hash := sha256.Sum256([]byte(content))
	expected := fmt.Sprintf("sha256:%x", hash)

	return strings.EqualFold(p.Checksum, expected), nil
}

package plugin

import (
	"os"
	"path/filepath"
	"sort"
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

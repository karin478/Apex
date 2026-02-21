package template

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/lyndonlyu/apex/internal/dag"
	"gopkg.in/yaml.v3"
)

// ErrTemplateNotFound is returned when a template cannot be found in the registry.
var ErrTemplateNotFound = errors.New("template: not found")

// TemplateVar describes a variable that can be substituted in a template.
type TemplateVar struct {
	Name    string `json:"name"    yaml:"name"`
	Default string `json:"default" yaml:"default"`
	Desc    string `json:"desc"    yaml:"desc"`
}

// TemplateNode describes a single node within a template pipeline.
type TemplateNode struct {
	ID      string   `json:"id"      yaml:"id"`
	Task    string   `json:"task"    yaml:"task"`
	Depends []string `json:"depends" yaml:"depends"`
}

// Template represents a reusable task pipeline with variable substitution support.
type Template struct {
	Name  string         `json:"name"  yaml:"name"`
	Desc  string         `json:"desc"  yaml:"desc"`
	Vars  []TemplateVar  `json:"vars"  yaml:"vars"`
	Nodes []TemplateNode `json:"nodes" yaml:"nodes"`
}

// Registry is a thread-safe store of named templates.
type Registry struct {
	mu        sync.RWMutex
	templates map[string]Template
}

// Load parses YAML bytes into a Template.
func Load(data []byte) (Template, error) {
	var t Template
	if err := yaml.Unmarshal(data, &t); err != nil {
		return Template{}, fmt.Errorf("template: failed to parse YAML: %w", err)
	}
	return t, nil
}

// LoadDir loads all *.yaml and *.yml files from a directory and returns
// successfully parsed templates. Invalid files are silently skipped.
func LoadDir(dir string) ([]Template, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("template: failed to read directory %q: %w", dir, err)
	}

	var templates []Template
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		t, err := Load(data)
		if err != nil {
			continue
		}
		templates = append(templates, t)
	}
	return templates, nil
}

// Expand substitutes template variables in task strings and returns a slice
// of dag.NodeSpec ready for DAG construction. Missing variables are filled
// from defaults via ApplyDefaults.
func (t *Template) Expand(vars map[string]string) ([]dag.NodeSpec, error) {
	merged := t.ApplyDefaults(vars)
	specs := make([]dag.NodeSpec, len(t.Nodes))
	for i, node := range t.Nodes {
		task := node.Task
		for _, v := range t.Vars {
			placeholder := "{{." + v.Name + "}}"
			task = strings.ReplaceAll(task, placeholder, merged[v.Name])
		}
		specs[i] = dag.NodeSpec{
			ID:      node.ID,
			Task:    task,
			Depends: node.Depends,
		}
	}
	return specs, nil
}

// VarNames returns the names of all variables defined in the template.
func (t *Template) VarNames() []string {
	names := make([]string, len(t.Vars))
	for i, v := range t.Vars {
		names[i] = v.Name
	}
	return names
}

// ApplyDefaults returns a merged variable map. Variables present in vars
// take precedence; missing variables are filled from template defaults.
// Extra variables not defined in the template are also preserved.
func (t *Template) ApplyDefaults(vars map[string]string) map[string]string {
	result := make(map[string]string)
	for _, v := range t.Vars {
		if val, ok := vars[v.Name]; ok {
			result[v.Name] = val
		} else {
			result[v.Name] = v.Default
		}
	}
	// Also include any extra vars passed in.
	for k, v := range vars {
		if _, exists := result[k]; !exists {
			result[k] = v
		}
	}
	return result
}

// NewRegistry creates an empty template registry.
func NewRegistry() *Registry {
	return &Registry{
		templates: make(map[string]Template),
	}
}

// Register adds a template to the registry. Returns an error if the
// template name is empty.
func (r *Registry) Register(t Template) error {
	if t.Name == "" {
		return fmt.Errorf("template: cannot register template with empty name")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.templates[t.Name] = t
	return nil
}

// Get retrieves a template by name. Returns ErrTemplateNotFound if the
// template does not exist in the registry.
func (r *Registry) Get(name string) (Template, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.templates[name]
	if !ok {
		return Template{}, ErrTemplateNotFound
	}
	return t, nil
}

// List returns all registered templates sorted alphabetically by name.
func (r *Registry) List() []Template {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]Template, 0, len(r.templates))
	for _, t := range r.templates {
		list = append(list, t)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	return list
}

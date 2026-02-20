// Package profile manages named configuration profiles for environment switching.
package profile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Sentinel errors.
var (
	ErrProfileNotFound = errors.New("profile: not found")
	ErrNoActiveProfile = errors.New("profile: no active profile")
)

// Profile holds a named configuration preset.
type Profile struct {
	Name        string `json:"name"         yaml:"name"`
	Mode        string `json:"mode"         yaml:"mode"`
	Sandbox     string `json:"sandbox"      yaml:"sandbox"`
	RateLimit   string `json:"rate_limit"   yaml:"rate_limit"`
	Concurrency int    `json:"concurrency"  yaml:"concurrency"`
	Description string `json:"description"  yaml:"description"`
}

// Registry manages profile registration and activation.
type Registry struct {
	mu       sync.RWMutex
	profiles map[string]Profile
	active   string
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{profiles: make(map[string]Profile)}
}

// DefaultProfiles returns the 3 built-in profiles.
func DefaultProfiles() []Profile {
	return []Profile{
		{Name: "dev", Mode: "NORMAL", Sandbox: "none", RateLimit: "default", Concurrency: 2, Description: "Development environment"},
		{Name: "staging", Mode: "EXPLORATORY", Sandbox: "ulimit", RateLimit: "standard", Concurrency: 4, Description: "Staging environment"},
		{Name: "prod", Mode: "BATCH", Sandbox: "docker", RateLimit: "strict", Concurrency: 8, Description: "Production environment"},
	}
}

// Register adds a profile to the registry. Returns an error if name is empty.
func (r *Registry) Register(p Profile) error {
	if p.Name == "" {
		return fmt.Errorf("profile: name cannot be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.profiles[p.Name] = p
	return nil
}

// Activate sets the active profile. Returns ErrProfileNotFound if not registered.
func (r *Registry) Activate(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.profiles[name]; !ok {
		return fmt.Errorf("%w: %s", ErrProfileNotFound, name)
	}
	r.active = name
	return nil
}

// Active returns the currently active profile.
func (r *Registry) Active() (Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.active == "" {
		return Profile{}, ErrNoActiveProfile
	}
	p, ok := r.profiles[r.active]
	if !ok {
		return Profile{}, fmt.Errorf("%w: %s", ErrProfileNotFound, r.active)
	}
	return p, nil
}

// Get returns a profile by name.
func (r *Registry) Get(name string) (Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("%w: %s", ErrProfileNotFound, name)
	}
	return p, nil
}

// List returns all profiles sorted by name.
func (r *Registry) List() []Profile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Profile, 0, len(r.profiles))
	for _, p := range r.profiles {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// LoadProfile parses YAML bytes into a Profile.
func LoadProfile(data []byte) (Profile, error) {
	var p Profile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Profile{}, fmt.Errorf("profile: parse: %w", err)
	}
	return p, nil
}

// LoadProfileDir loads all *.yaml/*.yml files from a directory, skipping invalid ones.
func LoadProfileDir(dir string) ([]Profile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("profile: read dir: %w", err)
	}
	var profiles []Profile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		p, err := LoadProfile(data)
		if err != nil {
			continue
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

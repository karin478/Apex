# Configuration Profile Manager Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an `internal/profile` package that manages named configuration profiles for environment switching.

**Architecture:** Profile struct, Registry with mutex-protected map + active state, DefaultProfiles factory, YAML loading. Format functions + Cobra CLI.

**Tech Stack:** Go, `sync`, `os`, `path/filepath`, `encoding/json`, `sort`, `gopkg.in/yaml.v3`, Testify, Cobra CLI

---

### Task 1: Configuration Profile Core — Registry + Load + DefaultProfiles (7 tests)

**Files:**
- Create: `internal/profile/profile.go`
- Create: `internal/profile/profile_test.go`

**Step 1: Write 7 failing tests**

```go
package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	list := r.List()
	assert.Empty(t, list)
}

func TestDefaultProfiles(t *testing.T) {
	profiles := DefaultProfiles()
	assert.Len(t, profiles, 3)

	names := make([]string, len(profiles))
	for i, p := range profiles {
		names[i] = p.Name
	}
	assert.Contains(t, names, "dev")
	assert.Contains(t, names, "staging")
	assert.Contains(t, names, "prod")
}

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()

	t.Run("valid profile", func(t *testing.T) {
		err := r.Register(Profile{Name: "test", Mode: "NORMAL"})
		require.NoError(t, err)

		p, err := r.Get("test")
		require.NoError(t, err)
		assert.Equal(t, "test", p.Name)
	})

	t.Run("empty name", func(t *testing.T) {
		err := r.Register(Profile{Name: ""})
		assert.Error(t, err)
	})
}

func TestRegistryActivate(t *testing.T) {
	r := NewRegistry()
	err := r.Register(Profile{Name: "dev", Mode: "NORMAL"})
	require.NoError(t, err)

	t.Run("valid activate", func(t *testing.T) {
		err := r.Activate("dev")
		require.NoError(t, err)

		active, err := r.Active()
		require.NoError(t, err)
		assert.Equal(t, "dev", active.Name)
	})

	t.Run("unknown profile", func(t *testing.T) {
		err := r.Activate("unknown")
		assert.ErrorIs(t, err, ErrProfileNotFound)
	})
}

func TestRegistryActive(t *testing.T) {
	r := NewRegistry()

	t.Run("no active profile", func(t *testing.T) {
		_, err := r.Active()
		assert.ErrorIs(t, err, ErrNoActiveProfile)
	})

	t.Run("with active profile", func(t *testing.T) {
		err := r.Register(Profile{Name: "staging", Mode: "EXPLORATORY"})
		require.NoError(t, err)
		err = r.Activate("staging")
		require.NoError(t, err)

		active, err := r.Active()
		require.NoError(t, err)
		assert.Equal(t, "staging", active.Name)
		assert.Equal(t, "EXPLORATORY", active.Mode)
	})
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()
	err := r.Register(Profile{Name: "prod", Mode: "BATCH", Concurrency: 8})
	require.NoError(t, err)

	t.Run("known profile", func(t *testing.T) {
		p, err := r.Get("prod")
		require.NoError(t, err)
		assert.Equal(t, "prod", p.Name)
		assert.Equal(t, 8, p.Concurrency)
	})

	t.Run("unknown profile", func(t *testing.T) {
		_, err := r.Get("nope")
		assert.ErrorIs(t, err, ErrProfileNotFound)
	})
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	for _, p := range DefaultProfiles() {
		err := r.Register(p)
		require.NoError(t, err)
	}

	list := r.List()
	assert.Len(t, list, 3)
	// Verify sorted by name.
	for i := 1; i < len(list); i++ {
		assert.True(t, list[i-1].Name <= list[i].Name,
			"expected sorted order, got %s before %s", list[i-1].Name, list[i].Name)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/profile/ -v -count=1`
Expected: FAIL — functions not defined

**Step 3: Write minimal implementation**

```go
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
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/profile/ -v -count=1 -race`
Expected: PASS (7 tests)

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/profile/profile.go internal/profile/profile_test.go
git commit -m "feat(profile): add configuration profile registry with YAML loading and default profiles

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: Format Functions + CLI Commands

**Files:**
- Create: `internal/profile/format.go`
- Create: `cmd/apex/profile.go`
- Modify: `cmd/apex/main.go` (add `rootCmd.AddCommand(profileCmd)` after the `progressCmd` line)

**Step 1: Write format.go**

```go
package profile

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatProfileList formats a list of profiles as a human-readable table.
func FormatProfileList(profiles []Profile) string {
	if len(profiles) == 0 {
		return "No profiles registered.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-12s %-14s %-10s %-12s %s\n",
		"NAME", "MODE", "SANDBOX", "RATE_LIMIT", "CONCURRENCY")
	for _, p := range profiles {
		fmt.Fprintf(&b, "%-12s %-14s %-10s %-12s %d\n",
			p.Name, p.Mode, p.Sandbox, p.RateLimit, p.Concurrency)
	}
	return b.String()
}

// FormatProfile formats a single profile for display.
func FormatProfile(p Profile) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name:        %s\n", p.Name)
	fmt.Fprintf(&b, "Mode:        %s\n", p.Mode)
	fmt.Fprintf(&b, "Sandbox:     %s\n", p.Sandbox)
	fmt.Fprintf(&b, "Rate Limit:  %s\n", p.RateLimit)
	fmt.Fprintf(&b, "Concurrency: %d\n", p.Concurrency)
	fmt.Fprintf(&b, "Description: %s\n", p.Description)
	return b.String()
}

// FormatProfileListJSON formats profiles as indented JSON.
func FormatProfileListJSON(profiles []Profile) (string, error) {
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return "", fmt.Errorf("profile: json marshal: %w", err)
	}
	return string(data), nil
}
```

**Step 2: Write cmd/apex/profile.go CLI**

```go
package main

import (
	"fmt"

	"github.com/lyndonlyu/apex/internal/profile"
	"github.com/spf13/cobra"
)

var profileFormat string

// profileRegistry is a package-level registry pre-loaded with defaults.
var profileRegistry = func() *profile.Registry {
	r := profile.NewRegistry()
	for _, p := range profile.DefaultProfiles() {
		_ = r.Register(p)
	}
	return r
}()

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Configuration profile management",
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered profiles",
	RunE:  runProfileList,
}

var profileShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show configuration for a specific profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileShow,
}

var profileActivateCmd = &cobra.Command{
	Use:   "activate <name>",
	Short: "Activate a named profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileActivate,
}

func init() {
	profileListCmd.Flags().StringVar(&profileFormat, "format", "", "Output format (json)")
	profileCmd.AddCommand(profileListCmd, profileShowCmd, profileActivateCmd)
}

func runProfileList(cmd *cobra.Command, args []string) error {
	list := profileRegistry.List()

	if profileFormat == "json" {
		out, err := profile.FormatProfileListJSON(list)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(profile.FormatProfileList(list))
	}
	return nil
}

func runProfileShow(cmd *cobra.Command, args []string) error {
	p, err := profileRegistry.Get(args[0])
	if err != nil {
		return err
	}
	fmt.Print(profile.FormatProfile(p))
	return nil
}

func runProfileActivate(cmd *cobra.Command, args []string) error {
	if err := profileRegistry.Activate(args[0]); err != nil {
		return err
	}
	active, err := profileRegistry.Active()
	if err != nil {
		return err
	}
	fmt.Printf("Profile activated: %s\n", active.Name)
	return nil
}
```

**Step 3: Add command to main.go**

Add `rootCmd.AddCommand(profileCmd)` after the `progressCmd` line in `cmd/apex/main.go`.

**Step 4: Run build + tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build ./cmd/apex/ && go test ./internal/profile/ -v -count=1`
Expected: BUILD OK, PASS

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/profile/format.go cmd/apex/profile.go cmd/apex/main.go
git commit -m "feat(profile): add format functions and CLI for profile list/show/activate

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: E2E Tests (3 tests)

**Files:**
- Create: `e2e/profile_test.go`

**Step 1: Write E2E tests**

```go
package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProfileList(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("profile", "list")

	assert.Equal(t, 0, exitCode,
		"apex profile list should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "dev"),
		"stdout should contain dev, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "staging"),
		"stdout should contain staging, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "prod"),
		"stdout should contain prod, got: %s", stdout)
}

func TestProfileActivate(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("profile", "activate", "dev")

	assert.Equal(t, 0, exitCode,
		"apex profile activate dev should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "dev"),
		"stdout should contain dev, got: %s", stdout)
}

func TestProfileShow(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("profile", "show", "prod")

	assert.Equal(t, 0, exitCode,
		"apex profile show prod should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "prod"),
		"stdout should contain prod, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "BATCH"),
		"stdout should contain BATCH mode, got: %s", stdout)
}
```

**Step 2: Build and run E2E tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build -o bin/apex ./cmd/apex/ && go test ./e2e/ -run TestProfile -v -count=1`
Expected: PASS (3 tests)

**Step 3: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add e2e/profile_test.go
git commit -m "test(e2e): add E2E tests for profile list, activate, and show

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Update PROGRESS.md

**Files:**
- Modify: `PROGRESS.md`

**Step 1: Update completed phases table**

Add row: `| 39 | Configuration Profile Manager | \`2026-02-20-phase39-config-profile-design.md\` | Done |`

**Step 2: Update Current section**

Change "Phase 39 — TBD" → "Phase 40 — TBD"

**Step 3: Update test counts**

- Unit tests: 44 → 45 packages
- E2E tests: 114 → 117 tests

**Step 4: Add Key Package**

Add: `| \`internal/profile\` | Named configuration profiles with registry, YAML loading, and environment switching (dev/staging/prod) |`

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add PROGRESS.md
git commit -m "docs: mark Phase 39 Configuration Profile Manager as complete

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

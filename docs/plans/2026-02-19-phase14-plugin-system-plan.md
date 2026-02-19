# Phase 14: Plugin System — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a plugin management framework with directory scanning, SHA-256 verification, enable/disable lifecycle, and CLI commands (`apex plugin scan|list|enable|disable|verify`).

**Architecture:** New `internal/plugin` package with `Registry` for plugin discovery and management. New `internal/reasoning/registry.go` for reasoning protocol registration. CLI subcommands follow the `snapshot.go` pattern (parent + child commands). Adversarial Review registered as built-in protocol.

**Tech Stack:** Go, Cobra CLI, `gopkg.in/yaml.v3` (already in go.mod), `crypto/sha256`, Testify

---

### Task 1: Plugin Data Model + Load

**Files:**
- Create: `internal/plugin/plugin.go`
- Create: `internal/plugin/plugin_test.go`

**Step 1: Write the failing test**

Create `internal/plugin/plugin_test.go`:

```go
package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPlugin(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "my-plugin")
	os.MkdirAll(pluginDir, 0755)

	yaml := `name: my-plugin
version: "1.0.0"
type: reasoning
description: "A test plugin"
author: test
checksum: ""
enabled: true
reasoning:
  protocol: my-protocol
  steps: 4
  roles: ["a", "b", "c", "d"]
`
	os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(yaml), 0644)

	p, err := LoadPlugin(pluginDir)
	require.NoError(t, err)
	assert.Equal(t, "my-plugin", p.Name)
	assert.Equal(t, "1.0.0", p.Version)
	assert.Equal(t, "reasoning", p.Type)
	assert.Equal(t, "A test plugin", p.Description)
	assert.True(t, p.Enabled)
	assert.Equal(t, pluginDir, p.Dir)
	require.NotNil(t, p.Reasoning)
	assert.Equal(t, "my-protocol", p.Reasoning.Protocol)
	assert.Equal(t, 4, p.Reasoning.Steps)
	assert.Len(t, p.Reasoning.Roles, 4)
}

func TestLoadPluginNotFound(t *testing.T) {
	_, err := LoadPlugin("/nonexistent")
	assert.Error(t, err)
}

func TestLoadPluginInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte("not: [valid: yaml"), 0644)
	_, err := LoadPlugin(dir)
	assert.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/plugin/... -run "TestLoadPlugin" -v`
Expected: FAIL — package does not exist

**Step 3: Write minimal implementation**

Create `internal/plugin/plugin.go`:

```go
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
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/plugin/... -run "TestLoadPlugin" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/plugin/plugin.go internal/plugin/plugin_test.go
git commit -m "feat(plugin): add Plugin data model and LoadPlugin"
```

---

### Task 2: Registry — Scan, List, Get

**Files:**
- Create: `internal/plugin/registry.go`
- Modify: `internal/plugin/plugin_test.go`

**Step 1: Write the failing test**

Add to `internal/plugin/plugin_test.go`:

```go
func writePluginYAML(t *testing.T, dir, name, version, ptype string, enabled bool) {
	t.Helper()
	pluginDir := filepath.Join(dir, name)
	os.MkdirAll(pluginDir, 0755)
	yaml := fmt.Sprintf(`name: %s
version: "%s"
type: %s
description: "Test plugin %s"
author: test
checksum: ""
enabled: %t
`, name, version, ptype, name, enabled)
	os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(yaml), 0644)
}

func TestRegistryScan(t *testing.T) {
	dir := t.TempDir()
	writePluginYAML(t, dir, "plugin-a", "1.0.0", "reasoning", true)
	writePluginYAML(t, dir, "plugin-b", "0.2.0", "reasoning", false)

	reg := NewRegistry(dir)
	plugins, err := reg.Scan()
	require.NoError(t, err)
	assert.Len(t, plugins, 2)
}

func TestRegistryScanEmptyDir(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	plugins, err := reg.Scan()
	require.NoError(t, err)
	assert.Empty(t, plugins)
}

func TestRegistryScanNonexistentDir(t *testing.T) {
	reg := NewRegistry("/nonexistent/path")
	plugins, err := reg.Scan()
	require.NoError(t, err)
	assert.Empty(t, plugins)
}

func TestRegistryList(t *testing.T) {
	dir := t.TempDir()
	writePluginYAML(t, dir, "plugin-a", "1.0.0", "reasoning", true)

	reg := NewRegistry(dir)
	reg.Scan()
	list := reg.List()
	assert.Len(t, list, 1)
	assert.Equal(t, "plugin-a", list[0].Name)
}

func TestRegistryGet(t *testing.T) {
	dir := t.TempDir()
	writePluginYAML(t, dir, "plugin-a", "1.0.0", "reasoning", true)

	reg := NewRegistry(dir)
	reg.Scan()

	p, ok := reg.Get("plugin-a")
	assert.True(t, ok)
	assert.Equal(t, "plugin-a", p.Name)

	_, ok = reg.Get("nonexistent")
	assert.False(t, ok)
}
```

Add `"fmt"` to imports.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/plugin/... -run "TestRegistry" -v`
Expected: FAIL — NewRegistry not defined

**Step 3: Write minimal implementation**

Create `internal/plugin/registry.go`:

```go
package plugin

import (
	"os"
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
		p, err := LoadPlugin(entry.Name())
		if err != nil {
			// Use full path
			fullPath := r.dir + "/" + entry.Name()
			p, err = LoadPlugin(fullPath)
			if err != nil {
				continue // skip invalid plugins
			}
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
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/plugin/... -run "TestRegistry" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/plugin/registry.go internal/plugin/plugin_test.go
git commit -m "feat(plugin): add Registry with Scan, List, Get"
```

---

### Task 3: Enable/Disable + SHA-256 Verify

**Files:**
- Modify: `internal/plugin/registry.go`
- Modify: `internal/plugin/plugin_test.go`

**Step 1: Write the failing test**

Add to `internal/plugin/plugin_test.go`:

```go
import "crypto/sha256"

func TestRegistryEnableDisable(t *testing.T) {
	dir := t.TempDir()
	writePluginYAML(t, dir, "my-plugin", "1.0.0", "reasoning", true)

	reg := NewRegistry(dir)
	reg.Scan()

	// Disable
	err := reg.Disable("my-plugin")
	require.NoError(t, err)

	p, _ := reg.Get("my-plugin")
	assert.False(t, p.Enabled)

	// Verify file was updated
	reloaded, _ := LoadPlugin(filepath.Join(dir, "my-plugin"))
	assert.False(t, reloaded.Enabled)

	// Enable
	err = reg.Enable("my-plugin")
	require.NoError(t, err)

	p, _ = reg.Get("my-plugin")
	assert.True(t, p.Enabled)
}

func TestRegistryEnableNotFound(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	reg.Scan()
	assert.Error(t, reg.Enable("nonexistent"))
}

func TestRegistryVerifyOK(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "my-plugin")
	os.MkdirAll(pluginDir, 0755)

	// Write plugin.yaml without checksum first
	content := `name: my-plugin
version: "1.0.0"
type: reasoning
description: "Test"
author: test
enabled: true
`
	// Compute checksum of content
	hash := sha256.Sum256([]byte(content))
	checksum := fmt.Sprintf("sha256:%x", hash)

	// Write with checksum
	fullContent := content + fmt.Sprintf("checksum: \"%s\"\n", checksum)
	os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(fullContent), 0644)

	reg := NewRegistry(dir)
	reg.Scan()

	ok, err := reg.Verify("my-plugin")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestRegistryVerifyMismatch(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "my-plugin")
	os.MkdirAll(pluginDir, 0755)

	content := `name: my-plugin
version: "1.0.0"
type: reasoning
description: "Test"
author: test
enabled: true
checksum: "sha256:wronghash"
`
	os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(content), 0644)

	reg := NewRegistry(dir)
	reg.Scan()

	ok, err := reg.Verify("my-plugin")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestRegistryVerifyEmptyChecksum(t *testing.T) {
	dir := t.TempDir()
	writePluginYAML(t, dir, "my-plugin", "1.0.0", "reasoning", true)

	reg := NewRegistry(dir)
	reg.Scan()

	ok, err := reg.Verify("my-plugin")
	require.NoError(t, err)
	assert.False(t, ok, "empty checksum should fail verification")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/plugin/... -run "TestRegistryEnable|TestRegistryVerify" -v`
Expected: FAIL — Enable, Disable, Verify not defined

**Step 3: Write minimal implementation**

Add to `internal/plugin/registry.go`:

```go
import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)
```

(merge with existing imports)

```go
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
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/plugin/... -v`
Expected: ALL tests pass

**Step 5: Commit**

```bash
git add internal/plugin/registry.go internal/plugin/plugin_test.go
git commit -m "feat(plugin): add Enable/Disable and SHA-256 Verify"
```

---

### Task 4: Reasoning Protocol Registry

**Files:**
- Create: `internal/reasoning/registry.go`
- Create: `internal/reasoning/registry_test.go`

**Step 1: Write the failing test**

Create `internal/reasoning/registry_test.go`:

```go
package reasoning

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterAndGetProtocol(t *testing.T) {
	// Clear registry for test isolation
	clearRegistry()

	Register(Protocol{
		Name:        "test-protocol",
		Description: "A test protocol",
		Run: func(ctx context.Context, runner Runner, proposal string, progress ProgressFunc) (*ReviewResult, error) {
			return &ReviewResult{Proposal: proposal}, nil
		},
	})

	p, ok := GetProtocol("test-protocol")
	require.True(t, ok)
	assert.Equal(t, "test-protocol", p.Name)
	assert.Equal(t, "A test protocol", p.Description)

	// Run it
	result, err := p.Run(context.Background(), nil, "test", nil)
	require.NoError(t, err)
	assert.Equal(t, "test", result.Proposal)
}

func TestGetUnknownProtocol(t *testing.T) {
	clearRegistry()
	_, ok := GetProtocol("nonexistent")
	assert.False(t, ok)
}

func TestListProtocols(t *testing.T) {
	clearRegistry()
	Register(Protocol{Name: "proto-a", Description: "A"})
	Register(Protocol{Name: "proto-b", Description: "B"})

	list := ListProtocols()
	assert.Len(t, list, 2)
}

func TestBuiltinAdversarialReview(t *testing.T) {
	// Re-register builtins
	clearRegistry()
	registerBuiltins()

	p, ok := GetProtocol("adversarial-review")
	require.True(t, ok)
	assert.Equal(t, "adversarial-review", p.Name)
	assert.NotNil(t, p.Run)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/reasoning/... -run "TestRegister|TestGetUnknown|TestList|TestBuiltin" -v`
Expected: FAIL — Protocol, Register, GetProtocol, etc. not defined

**Step 3: Write minimal implementation**

Create `internal/reasoning/registry.go`:

```go
package reasoning

import (
	"context"
	"sort"
	"sync"
)

// Protocol defines a registered reasoning protocol.
type Protocol struct {
	Name        string
	Description string
	Run         func(ctx context.Context, runner Runner, proposal string, progress ProgressFunc) (*ReviewResult, error)
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Protocol{}
)

func Register(p Protocol) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[p.Name] = p
}

func GetProtocol(name string) (Protocol, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	p, ok := registry[name]
	return p, ok
}

func ListProtocols() []Protocol {
	registryMu.RLock()
	defer registryMu.RUnlock()
	var list []Protocol
	for _, p := range registry {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	return list
}

func clearRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = map[string]Protocol{}
}

func registerBuiltins() {
	Register(Protocol{
		Name:        "adversarial-review",
		Description: "4-step Advocate/Critic/Judge debate",
		Run: func(ctx context.Context, runner Runner, proposal string, progress ProgressFunc) (*ReviewResult, error) {
			return RunReviewWithProgress(ctx, runner, proposal, progress)
		},
	})
}

func init() {
	registerBuiltins()
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/reasoning/... -v`
Expected: ALL tests pass (8 existing + 4 new = 12)

**Step 5: Commit**

```bash
git add internal/reasoning/registry.go internal/reasoning/registry_test.go
git commit -m "feat(reasoning): add protocol registry with built-in adversarial-review"
```

---

### Task 5: CLI Commands (`apex plugin`)

**Files:**
- Create: `cmd/apex/plugin.go`
- Modify: `cmd/apex/main.go` — add `rootCmd.AddCommand(pluginCmd)` in `init()`

**Step 1: Write the implementation**

Create `cmd/apex/plugin.go`:

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/plugin"
	"github.com/spf13/cobra"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage plugins",
}

var pluginScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan for plugins in ~/.apex/plugins/",
	RunE:  pluginScan,
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all plugins",
	RunE:  pluginList,
}

var pluginEnableCmd = &cobra.Command{
	Use:   "enable [name]",
	Short: "Enable a plugin",
	Args:  cobra.ExactArgs(1),
	RunE:  pluginEnable,
}

var pluginDisableCmd = &cobra.Command{
	Use:   "disable [name]",
	Short: "Disable a plugin",
	Args:  cobra.ExactArgs(1),
	RunE:  pluginDisable,
}

var pluginVerifyCmd = &cobra.Command{
	Use:   "verify [name]",
	Short: "Verify plugin checksum",
	Args:  cobra.ExactArgs(1),
	RunE:  pluginVerify,
}

func init() {
	pluginCmd.AddCommand(pluginScanCmd)
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginEnableCmd)
	pluginCmd.AddCommand(pluginDisableCmd)
	pluginCmd.AddCommand(pluginVerifyCmd)
}

func pluginsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".apex", "plugins")
}

func pluginScan(cmd *cobra.Command, args []string) error {
	reg := plugin.NewRegistry(pluginsDir())
	plugins, err := reg.Scan()
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}
	fmt.Printf("Found %d plugin(s)\n", len(plugins))
	for _, p := range plugins {
		fmt.Printf("  %s v%s [%s]\n", p.Name, p.Version, p.Type)
	}
	return nil
}

func pluginList(cmd *cobra.Command, args []string) error {
	reg := plugin.NewRegistry(pluginsDir())
	if _, err := reg.Scan(); err != nil {
		return err
	}
	plugins := reg.List()
	if len(plugins) == 0 {
		fmt.Println("No plugins found.")
		fmt.Printf("Place plugins in %s/\n", pluginsDir())
		return nil
	}

	fmt.Printf("Plugins (%d found)\n", len(plugins))
	fmt.Println("=================")
	fmt.Println()
	for _, p := range plugins {
		status := "enabled"
		if !p.Enabled {
			status = "disabled"
		}
		fmt.Printf("  %-24s v%-8s [%s]  %s\n", p.Name, p.Version, status, p.Description)
	}
	return nil
}

func pluginEnable(cmd *cobra.Command, args []string) error {
	reg := plugin.NewRegistry(pluginsDir())
	if _, err := reg.Scan(); err != nil {
		return err
	}
	if err := reg.Enable(args[0]); err != nil {
		return err
	}
	fmt.Printf("Plugin %q enabled\n", args[0])
	return nil
}

func pluginDisable(cmd *cobra.Command, args []string) error {
	reg := plugin.NewRegistry(pluginsDir())
	if _, err := reg.Scan(); err != nil {
		return err
	}
	if err := reg.Disable(args[0]); err != nil {
		return err
	}
	fmt.Printf("Plugin %q disabled\n", args[0])
	return nil
}

func pluginVerify(cmd *cobra.Command, args []string) error {
	reg := plugin.NewRegistry(pluginsDir())
	if _, err := reg.Scan(); err != nil {
		return err
	}
	p, ok := reg.Get(args[0])
	if !ok {
		return fmt.Errorf("plugin %q not found", args[0])
	}

	valid, err := reg.Verify(args[0])
	if err != nil {
		return err
	}
	if valid {
		fmt.Printf("Checksum: OK (%s)\n", p.Checksum)
	} else if p.Checksum == "" {
		fmt.Println("Checksum: SKIP (no checksum defined)")
	} else {
		fmt.Printf("Checksum: MISMATCH (expected %s)\n", p.Checksum)
	}
	return nil
}
```

**Step 2: Register command in `cmd/apex/main.go`**

Add `rootCmd.AddCommand(pluginCmd)` to the `init()` function.

**Step 3: Verify compilation**

Run: `go build ./cmd/apex/`
Expected: success

**Step 4: Commit**

```bash
git add cmd/apex/plugin.go cmd/apex/main.go
git commit -m "feat(cli): add apex plugin command with scan/list/enable/disable/verify"
```

---

### Task 6: E2E Tests

**Files:**
- Create: `e2e/plugin_test.go`

**Step 1: Write the E2E tests**

Create `e2e/plugin_test.go`:

```go
package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginListEmpty(t *testing.T) {
	env := newTestEnv(t)
	stdout, _, code := env.runApex("plugin", "list")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "No plugins found")
}

func TestPluginScanAndList(t *testing.T) {
	env := newTestEnv(t)

	// Create a plugin
	pluginsDir := filepath.Join(env.Home, ".apex", "plugins", "test-plugin")
	require.NoError(t, os.MkdirAll(pluginsDir, 0755))
	yaml := `name: test-plugin
version: "1.0.0"
type: reasoning
description: "A test plugin"
author: test
checksum: ""
enabled: true
`
	require.NoError(t, os.WriteFile(filepath.Join(pluginsDir, "plugin.yaml"), []byte(yaml), 0644))

	// Scan
	stdout, _, code := env.runApex("plugin", "scan")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "1 plugin(s)")
	assert.Contains(t, stdout, "test-plugin")

	// List
	stdout, _, code = env.runApex("plugin", "list")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "test-plugin")
	assert.Contains(t, stdout, "enabled")
}

func TestPluginEnableDisable(t *testing.T) {
	env := newTestEnv(t)

	pluginsDir := filepath.Join(env.Home, ".apex", "plugins", "test-plugin")
	os.MkdirAll(pluginsDir, 0755)
	yaml := `name: test-plugin
version: "1.0.0"
type: reasoning
description: "Test"
author: test
checksum: ""
enabled: true
`
	os.WriteFile(filepath.Join(pluginsDir, "plugin.yaml"), []byte(yaml), 0644)

	// Disable
	stdout, _, code := env.runApex("plugin", "disable", "test-plugin")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "disabled")

	// Verify it's disabled in list
	stdout, _, _ = env.runApex("plugin", "list")
	assert.Contains(t, stdout, "disabled")

	// Enable
	stdout, _, code = env.runApex("plugin", "enable", "test-plugin")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "enabled")
}

func TestPluginVerify(t *testing.T) {
	env := newTestEnv(t)

	pluginsDir := filepath.Join(env.Home, ".apex", "plugins", "test-plugin")
	os.MkdirAll(pluginsDir, 0755)
	yaml := `name: test-plugin
version: "1.0.0"
type: reasoning
description: "Test"
author: test
enabled: true
checksum: ""
`
	os.WriteFile(filepath.Join(pluginsDir, "plugin.yaml"), []byte(yaml), 0644)

	stdout, _, code := env.runApex("plugin", "verify", "test-plugin")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "SKIP")
}

func TestPluginEnableNotFound(t *testing.T) {
	env := newTestEnv(t)
	_, stderr, code := env.runApex("plugin", "enable", "nonexistent")
	assert.NotEqual(t, 0, code)
	_ = stderr // error printed
}
```

Add `"fmt"` to imports if not already present.

**Step 2: Run tests**

Run: `go test ./e2e/... -v -count=1 -timeout=120s`
Expected: ALL tests pass (36 existing + 5 new = 41)

**Step 3: Commit**

```bash
git add e2e/plugin_test.go
git commit -m "test(e2e): add plugin management E2E tests"
```

---

### Task 7: Update PROGRESS.md

**Files:**
- Modify: `PROGRESS.md`

**Step 1: Update progress**

- Add row: `| 14 | Plugin System | \`2026-02-19-phase14-plugin-system-design.md\` | Done |`
- Change "Current: Phase 14" to "Current: Phase 15 — TBD"
- Update E2E test count to 41
- Update unit test package count to 24
- Add `internal/plugin` and update `internal/reasoning` in Key Packages

**Step 2: Commit**

```bash
git add PROGRESS.md
git commit -m "docs: mark Phase 14 Plugin System as complete"
```

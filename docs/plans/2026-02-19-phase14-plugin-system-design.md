# Phase 14: Plugin System — Design Document

> Approved: 2026-02-19

## Overview

Establish Apex's plugin management framework. The first version supports Reasoning Protocol plugins only, discovered via directory scanning, verified with SHA-256 checksums, and registered through Go interfaces.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Extension point | Reasoning Protocols only | Validate framework with minimal scope; Adversarial Review just built |
| Discovery | Directory scan `~/.apex/plugins/` | Simple, consistent with `~/.apex/` structure |
| Integrity | SHA-256 checksum | Meets LOW risk requirement; signatures deferred |
| Execution | Go interface + built-in registry | Avoids dynamic loading complexity |

## Plugin Directory Structure

```
~/.apex/plugins/
  adversarial-review/
    plugin.yaml
  custom-protocol/
    plugin.yaml
```

Each plugin is a subdirectory containing a `plugin.yaml` manifest.

## plugin.yaml Format

```yaml
name: adversarial-review
version: "1.0.0"
type: reasoning
description: "4-step Advocate/Critic/Judge debate"
author: apex
checksum: "sha256:<hex>"
enabled: true

reasoning:
  protocol: adversarial-review
  steps: 4
  roles: ["advocate", "critic", "advocate", "judge"]
```

- `type` must be `"reasoning"` (only supported type in v1)
- `checksum` is SHA-256 of the `plugin.yaml` content excluding the checksum line itself
- `enabled` controls whether the plugin is active

## New Package: `internal/plugin`

### Data Model

```go
type Plugin struct {
    Name        string            `yaml:"name"`
    Version     string            `yaml:"version"`
    Type        string            `yaml:"type"`
    Description string            `yaml:"description"`
    Author      string            `yaml:"author"`
    Checksum    string            `yaml:"checksum"`
    Enabled     bool              `yaml:"enabled"`
    Reasoning   *ReasoningConfig  `yaml:"reasoning,omitempty"`
    Dir         string            `yaml:"-"`
}

type ReasoningConfig struct {
    Protocol string   `yaml:"protocol"`
    Steps    int      `yaml:"steps"`
    Roles    []string `yaml:"roles"`
}
```

### Registry

```go
type Registry struct { ... }

func NewRegistry(pluginsDir string) *Registry
func (r *Registry) Scan() ([]Plugin, error)
func (r *Registry) List() []Plugin
func (r *Registry) Get(name string) (*Plugin, bool)
func (r *Registry) Enable(name string) error
func (r *Registry) Disable(name string) error
func (r *Registry) Verify(name string) (bool, error)
```

- `Scan()` walks `pluginsDir`, reads each `plugin.yaml`, populates `Dir`
- `Enable/Disable` toggles `enabled` in `plugin.yaml` and writes back
- `Verify` computes SHA-256 of plugin.yaml (excluding checksum line) and compares

## Reasoning Protocol Registry

Enhance `internal/reasoning/` with a protocol registry:

```go
type Protocol struct {
    Name        string
    Description string
    Run         func(ctx context.Context, runner Runner, proposal string, progress ProgressFunc) (*ReviewResult, error)
}

func Register(p Protocol)
func GetProtocol(name string) (Protocol, bool)
func ListProtocols() []Protocol
```

The existing Adversarial Review is registered as `"adversarial-review"` at init time.

`apex review` is updated to accept `--protocol <name>` flag (default: `"adversarial-review"`).

## CLI Commands

```
apex plugin scan              — scan ~/.apex/plugins/ for plugins
apex plugin list              — list all plugins with status
apex plugin enable <name>     — enable a plugin
apex plugin disable <name>    — disable a plugin
apex plugin verify <name>     — verify plugin checksum
```

### Output Examples

```
$ apex plugin list
Plugins (2 found)
=================

  adversarial-review  v1.0.0  [enabled]   4-step Advocate/Critic/Judge debate
  custom-protocol     v0.1.0  [disabled]  Custom reasoning protocol

$ apex plugin verify adversarial-review
Checksum: OK (sha256:abc123...)
```

## Testing Strategy

### Unit Tests (`internal/plugin/plugin_test.go`)

- `TestScanPlugins` — temp dir with plugin.yaml, verify discovery
- `TestScanEmptyDir` — empty directory returns empty list
- `TestPluginEnableDisable` — toggle enabled flag, verify yaml rewrite
- `TestPluginVerifyChecksumOK` — correct checksum passes
- `TestPluginVerifyChecksumMismatch` — wrong checksum fails
- `TestPluginGetByName` — lookup by name

### Unit Tests (`internal/reasoning/registry_test.go`)

- `TestRegisterAndGet` — register protocol, retrieve by name
- `TestListProtocols` — list all registered protocols
- `TestGetUnknownProtocol` — unknown name returns false

### E2E Tests (`e2e/plugin_test.go`)

- `TestPluginListEmpty` — no plugins, show message
- `TestPluginScanAndList` — create plugin dir, scan, list
- `TestPluginEnableDisable` — CLI toggle operations
- `TestPluginVerify` — verify checksum via CLI

## Dependencies

- `gopkg.in/yaml.v3` — already in go.mod
- No new external dependencies

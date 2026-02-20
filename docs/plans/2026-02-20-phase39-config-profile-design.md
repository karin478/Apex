# Phase 39: Configuration Profile Manager

> Design doc for Apex Agent CLI — named configuration profiles for environment switching.

## Problem

The system has multiple configuration dimensions (sandbox level, execution mode, rate limit group, concurrency). Switching between environments (dev/staging/prod) requires manually editing YAML files. There is no mechanism to define named presets and switch between them quickly.

## Solution

A `profile` package that manages named configuration profiles. Each profile bundles mode, sandbox level, rate limit group, and concurrency into a single switchable unit. A `Registry` manages profile registration, activation, and lookup.

## Architecture

```
internal/profile/
├── profile.go       # Profile, Registry, Load, lifecycle methods
└── profile_test.go  # 7 unit tests
```

## Key Types

### Profile

```go
type Profile struct {
    Name        string `json:"name"         yaml:"name"`
    Mode        string `json:"mode"         yaml:"mode"`
    Sandbox     string `json:"sandbox"      yaml:"sandbox"`
    RateLimit   string `json:"rate_limit"   yaml:"rate_limit"`
    Concurrency int    `json:"concurrency"  yaml:"concurrency"`
    Description string `json:"description"  yaml:"description"`
}
```

### Registry

```go
type Registry struct {
    mu       sync.RWMutex
    profiles map[string]Profile
    active   string
}
```

## Core Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `NewRegistry` | `() *Registry` | Creates empty registry |
| `DefaultProfiles` | `() []Profile` | Returns 3 built-in profiles (dev, staging, prod) |
| `(*Registry) Register` | `(profile Profile) error` | Registers profile; error if name is empty |
| `(*Registry) Activate` | `(name string) error` | Activates a profile; error if not found |
| `(*Registry) Active` | `() (Profile, error)` | Returns active profile; ErrNoActiveProfile if none |
| `(*Registry) Get` | `(name string) (Profile, error)` | Returns profile by name; ErrProfileNotFound if missing |
| `(*Registry) List` | `() []Profile` | Returns all profiles sorted by name |
| `LoadProfile` | `(data []byte) (Profile, error)` | Parses YAML bytes into Profile |
| `LoadProfileDir` | `(dir string) ([]Profile, error)` | Loads all *.yaml/*.yml from directory, skips invalid |

Sentinel errors:
- `var ErrProfileNotFound = errors.New("profile: not found")`
- `var ErrNoActiveProfile = errors.New("profile: no active profile")`

## Default Profiles

| Name | Mode | Sandbox | Rate Limit | Concurrency | Description |
|------|------|---------|------------|-------------|-------------|
| `dev` | NORMAL | none | default | 2 | Development environment |
| `staging` | EXPLORATORY | ulimit | standard | 4 | Staging environment |
| `prod` | BATCH | docker | strict | 8 | Production environment |

## Design Decisions

### Profile as Data Struct

Profiles are plain data — they don't directly configure subsystems. The caller (agent framework) reads the active profile and applies settings to Mode Selector, Sandbox, etc. This keeps the profile package decoupled.

### Registry with Mutex

Thread-safe with `sync.RWMutex`, consistent with project patterns (Mode Selector, Progress Tracker, Connector).

### LoadProfileDir Resilience

`LoadProfileDir` skips invalid files (continues on error), matching the `LoadDir` pattern in datapuller and credinjector packages.

### Active Profile State

`Activate` sets the active profile name. `Active()` returns ErrNoActiveProfile if none is activated. This is explicit — no implicit default activation.

### DefaultProfiles as a Function

`DefaultProfiles()` returns a slice (not a map) for easy iteration and registration. The caller can selectively register defaults.

## CLI Commands

### `apex profile list [--format json]`
Lists all registered profiles with their configurations.

### `apex profile show <name>`
Shows detailed configuration for a specific profile.

### `apex profile activate <name>`
Activates a named profile. Confirms the activation.

## Unit Tests (7)

| Test | Description |
|------|-------------|
| `TestNewRegistry` | Creates empty registry, List returns empty |
| `TestDefaultProfiles` | Returns 3 profiles with correct names |
| `TestRegistryRegister` | Register succeeds; empty name returns error |
| `TestRegistryActivate` | Activate valid profile; unknown returns ErrProfileNotFound |
| `TestRegistryActive` | Returns active profile; no active returns ErrNoActiveProfile |
| `TestRegistryGet` | Known profile returned; unknown returns ErrProfileNotFound |
| `TestRegistryList` | Multiple profiles sorted alphabetically |

## E2E Tests (3)

| Test | Description |
|------|-------------|
| `TestProfileList` | CLI invocation → profile list output |
| `TestProfileActivate` | CLI invocation → activation confirmation |
| `TestProfileShow` | CLI invocation → profile details |

## Format Functions

| Function | Description |
|----------|-------------|
| `FormatProfileList(profiles []Profile) string` | Table: NAME / MODE / SANDBOX / RATE_LIMIT / CONCURRENCY |
| `FormatProfile(profile Profile) string` | Detailed single-profile display |
| `FormatProfileListJSON(profiles []Profile) (string, error)` | JSON output |

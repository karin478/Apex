# Phase 35: Credential Injector

> Design doc for Apex Agent CLI — zero-trust credential injection with placeholder-based secret management.

## Problem

Agents interact with external APIs (Connectors, Data Pullers) that require authentication tokens. Currently, auth tokens are resolved directly from environment variables at call time, meaning the agent code path has access to real secret values. If an error trace, log, or debug output leaks, real credentials are exposed.

## Solution

A `credinjector` package that manages credential references as placeholders (`<NAME_REF>`), resolves them only at the execution boundary, and provides a `Scrub` function to replace any leaked real values back to placeholders in error paths.

## Architecture

```
~/.claude/credentials/
├── grafana.yaml          # CredentialRef YAML files
├── github.yaml
└── slack.yml

internal/credinjector/
├── credinjector.go       # CredentialRef, Vault, Injector, Scrub
└── credinjector_test.go  # 7 unit tests
```

## Key Types

### CredentialRef

```go
type CredentialRef struct {
    Placeholder string `yaml:"placeholder" json:"placeholder"` // e.g. "<GRAFANA_TOKEN_REF>"
    Source      string `yaml:"source" json:"source"`           // "env" | "file"
    Key         string `yaml:"key" json:"key"`                 // env var name or file path
}
```

### Vault

```go
type Vault struct {
    Credentials []CredentialRef `yaml:"credentials" json:"credentials"`
}
```

### InjectionResult

```go
type InjectionResult struct {
    Output     string   `json:"output"`
    Injected   []string `json:"injected"`
    Unresolved []string `json:"unresolved"`
}
```

## Core Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `LoadVault` | `(path string) (*Vault, error)` | Parse single YAML file into Vault |
| `LoadVaultDir` | `(dir string) (*Vault, error)` | Merge all `*.yaml`/`*.yml` from directory into one Vault |
| `Resolve` | `(ref CredentialRef) (string, error)` | Resolve single ref: env → `os.Getenv`, file → `os.ReadFile` with trim |
| `ValidateVault` | `(vault *Vault) []error` | Check all refs are resolvable, return per-ref errors |
| `Inject` | `(template string, vault *Vault) InjectionResult` | Replace `<..._REF>` placeholders with resolved values |
| `Scrub` | `(text string, vault *Vault) string` | Reverse: replace real values back to placeholders |

## Design Decisions

### Placeholder Format

Regex: `<[A-Z][A-Z0-9_]*_REF>`. Uppercase with underscores, must end with `_REF`. Simple, distinctive, unlikely to collide with normal text.

### Source Types

Only `env` and `file`:
- `env` — `os.Getenv(key)`, error if empty
- `file` — `os.ReadFile(key)` with `strings.TrimSpace`, error if missing

No vault service integration (HashiCorp Vault, AWS Secrets Manager) — YAGNI for current scope.

### Scrub for Error Path Protection

`Scrub` resolves all credentials in the vault, then does `strings.ReplaceAll` for each resolved value → placeholder. This ensures error logs, exception traces, and HTTP dumps never contain real secrets. Order: longer values replaced first to avoid partial matches.

### Vault Merging

`LoadVaultDir` merges credentials from all YAML files into a single Vault. Duplicate placeholder names across files are detected and return an error.

## CLI Commands

### `apex credential list`
Lists all configured credential references (placeholder + source + key). Never shows real values.

### `apex credential validate`
Resolves all references and reports OK/FAIL per credential.

### `apex credential scrub`
Reads text from stdin, scrubs real credential values to placeholders, outputs result.

## Unit Tests (7)

| Test | Description |
|------|-------------|
| `TestLoadVault` | Valid YAML → correct Vault with CredentialRefs |
| `TestLoadVaultInvalid` | Missing file, invalid YAML → error |
| `TestLoadVaultDir` | Multiple YAML files merged, `.txt` ignored |
| `TestResolve` | env source → `os.Getenv`; file source → read+trim; missing → error |
| `TestValidateVault` | Mixed valid/invalid refs → correct error list |
| `TestInject` | Template with placeholders → resolved output + injected/unresolved lists |
| `TestScrub` | Text with real values → placeholders restored; longer values first |

## E2E Tests (3)

| Test | Description |
|------|-------------|
| `TestCredentialList` | Create vault YAML → `apex credential list` → placeholders shown |
| `TestCredentialValidate` | Valid + invalid refs → `apex credential validate` → OK/FAIL output |
| `TestCredentialListEmpty` | No credentials dir → `apex credential list` → "No credentials configured." |

## Format Functions

| Function | Description |
|----------|-------------|
| `FormatCredentialList(vault *Vault) string` | Table: PLACEHOLDER, SOURCE, KEY |
| `FormatValidationResult(refs []CredentialRef, errs []error) string` | OK/FAIL per credential |
| `FormatCredentialListJSON(vault *Vault) (string, error)` | JSON array output |

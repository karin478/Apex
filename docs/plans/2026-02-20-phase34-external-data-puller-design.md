# Phase 34: External Data Puller

> Design doc for Apex Agent CLI — autonomous data ingestion from external sources.

## Problem

The agent system can react to events (Phase 32) and connect to external tools (Phase 31), but has no mechanism to **proactively pull data** from external sources on a schedule. Without this, agents are purely reactive — they can only process what's pushed to them.

## Solution

A `datapuller` package that loads YAML-defined data source specs, executes HTTP pulls with auth, applies JSON path transforms, and returns structured results ready for event emission.

## Architecture

```
~/.claude/data_sources/
├── cve-feed.yaml        # SourceSpec YAML files
├── weather-api.yaml
└── metrics-endpoint.yml

internal/datapuller/
├── datapuller.go         # SourceSpec, Puller, PullResult
└── datapuller_test.go    # 7 unit tests

cmd/apex/
└── datasource.go         # CLI: apex datasource list|pull|validate
```

## Key Types

### SourceSpec

```go
type SourceSpec struct {
    Name       string            `yaml:"name" json:"name"`
    URL        string            `yaml:"url" json:"url"`
    Schedule   string            `yaml:"schedule" json:"schedule"`     // cron expression (informational)
    AuthType   string            `yaml:"auth_type" json:"auth_type"`   // "bearer"|"none"
    AuthToken  string            `yaml:"auth_token" json:"auth_token"` // env var ref: "$ENV_VAR"
    Headers    map[string]string `yaml:"headers" json:"headers"`
    Transform  string            `yaml:"transform" json:"transform"`   // JSON path expression
    EmitEvent  string            `yaml:"emit_event" json:"emit_event"` // event type to emit on success
    MaxRetries int               `yaml:"max_retries" json:"max_retries"`
}
```

### PullResult

```go
type PullResult struct {
    Source       string    `json:"source"`
    StatusCode   int       `json:"status_code"`
    RawBytes     int       `json:"raw_bytes"`
    Transformed  []byte    `json:"transformed"`
    EventEmitted string    `json:"event_emitted"`
    PulledAt     time.Time `json:"pulled_at"`
    Error        error     `json:"-"`
}
```

## Core Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `LoadSpec` | `(path string) (SourceSpec, error)` | Parse single YAML file into SourceSpec |
| `LoadDir` | `(dir string) ([]SourceSpec, error)` | Load all `*.yaml`/`*.yml` from directory |
| `ValidateSpec` | `(spec SourceSpec) error` | Validate required fields (name, URL) |
| `ResolveAuth` | `(spec SourceSpec) (string, error)` | Resolve `$ENV_VAR` references via `os.Getenv` |
| `Pull` | `(spec SourceSpec, client HTTPClient) PullResult` | Execute HTTP GET with auth headers, apply transform |
| `ApplyTransform` | `(data []byte, expr string) ([]byte, error)` | Extract JSON path from response data |

## Design Decisions

### JSON Path Transform (not full jq)

The `Transform` field uses a simple built-in JSON path syntax:
- `.field` — extract top-level field
- `.field.nested` — extract nested field
- `.field[].subfield` — extract subfield from each array element

This avoids a dependency on external `jq` binary. Sufficient for common data extraction patterns. If empty, raw response is returned as-is.

### Auth Resolution

Auth tokens use `$ENV_VAR` format referencing environment variables:
- `ResolveAuth` calls `os.Getenv` stripping the `$` prefix
- Returns error if env var is not set and `auth_type != "none"`
- Keeps actual secrets out of YAML files

### HTTP Client Interface

```go
type HTTPClient interface {
    Do(req *http.Request) (*http.Response, error)
}
```

Injected into `Pull` for testability. Production uses `http.DefaultClient`, tests use `httptest.Server`.

### Event Emission

`Pull` does NOT directly call the Event Runtime. Instead, `PullResult.EventEmitted` carries the event type string. The caller (CLI or scheduler) is responsible for creating and dispatching the event. This keeps `datapuller` decoupled from `internal/event`.

## CLI Commands

### `apex datasource list`
Lists all configured data sources from the default directory.

### `apex datasource pull <name>`
Manually triggers a pull for the named data source. Prints status code, bytes received, and transform result summary.

### `apex datasource validate`
Validates all YAML specs in the directory. Reports errors for malformed or incomplete specs.

## Unit Tests (7)

| Test | Description |
|------|-------------|
| `TestLoadSpec` | Valid YAML → correct SourceSpec fields |
| `TestLoadSpecInvalid` | Malformed YAML / missing fields → error |
| `TestLoadDir` | Directory with multiple YAML files → correct count |
| `TestValidateSpec` | Missing name/URL → error; valid spec → nil |
| `TestResolveAuth` | `$ENV_VAR` resolution; missing var → error; auth_type none → skip |
| `TestPull` | Mock HTTP server → correct PullResult with status, bytes, transform |
| `TestApplyTransform` | `.field`, `.nested.path`, `.items[].name` → correct extraction |

## E2E Tests (3)

| Test | Description |
|------|-------------|
| `TestDatasourceList` | Create temp YAML → `apex datasource list` → name appears in output |
| `TestDatasourcePull` | Create temp YAML + httptest mock → `apex datasource pull` → success output |
| `TestDatasourceValidate` | Valid + invalid YAML → `apex datasource validate` → correct error reporting |

## Format Functions

| Function | Description |
|----------|-------------|
| `FormatSourceList(specs []SourceSpec) string` | Table: Name, URL, Schedule, AuthType |
| `FormatPullResult(result PullResult) string` | Human-readable pull summary |
| `FormatSourceListJSON(specs []SourceSpec) (string, error)` | JSON array output |

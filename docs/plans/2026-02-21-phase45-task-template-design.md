# Phase 45: Task Template System

> Design doc for Apex Agent CLI — reusable DAG templates with variable substitution.

## Problem

Every DAG execution requires manually defining node specs. Common patterns (build-test-deploy, research-analyze-report) are recreated from scratch each time. There is no mechanism to define reusable task templates with parameterized inputs.

## Solution

A `template` package that defines reusable DAG templates with variable placeholders (`{{.VarName}}`). Templates are loaded from YAML files, variables are substituted at expansion time, and the result is a slice of `dag.NodeSpec` ready for DAG creation.

## Architecture

```
internal/template/
├── template.go       # Template, TemplateVar, Expand, Load
└── template_test.go  # 7 unit tests
```

## Key Types

### TemplateVar

```go
type TemplateVar struct {
    Name    string `json:"name"    yaml:"name"`
    Default string `json:"default" yaml:"default"`
    Desc    string `json:"desc"    yaml:"desc"`
}
```

### TemplateNode

```go
type TemplateNode struct {
    ID      string   `json:"id"      yaml:"id"`
    Task    string   `json:"task"    yaml:"task"`
    Depends []string `json:"depends" yaml:"depends"`
}
```

### Template

```go
type Template struct {
    Name  string         `json:"name"  yaml:"name"`
    Desc  string         `json:"desc"  yaml:"desc"`
    Vars  []TemplateVar  `json:"vars"  yaml:"vars"`
    Nodes []TemplateNode `json:"nodes" yaml:"nodes"`
}
```

## Core Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `Load` | `(data []byte) (Template, error)` | Parse YAML bytes into Template |
| `LoadDir` | `(dir string) ([]Template, error)` | Load all *.yaml/*.yml from directory, skip invalid |
| `(*Template) Expand` | `(vars map[string]string) ([]dag.NodeSpec, error)` | Substitute variables in task strings, return NodeSpecs |
| `(*Template) VarNames` | `() []string` | Returns variable names |
| `(*Template) ApplyDefaults` | `(vars map[string]string) map[string]string` | Fills in defaults for missing variables |
| `NewRegistry` | `() *Registry` | Creates empty template registry |
| `(*Registry) Register` | `(t Template) error` | Register template; error if name empty |
| `(*Registry) Get` | `(name string) (Template, error)` | Get by name; ErrTemplateNotFound if missing |
| `(*Registry) List` | `() []Template` | Returns all templates sorted by name |

Sentinel error:
- `var ErrTemplateNotFound = errors.New("template: not found")`

## Variable Substitution

`Expand(vars)` replaces `{{.VarName}}` patterns in each node's Task string:
1. Call `ApplyDefaults(vars)` to fill in missing variables with defaults
2. For each node, use `strings.ReplaceAll` for each variable: `{{.Name}}` → value
3. Return `[]dag.NodeSpec` with substituted task strings

## Example Template YAML

```yaml
name: build-test-deploy
desc: Standard build, test, and deploy pipeline
vars:
  - name: project
    default: myapp
    desc: Project name
  - name: env
    default: staging
    desc: Target environment
nodes:
  - id: build
    task: "Build {{.project}} for {{.env}}"
  - id: test
    task: "Test {{.project}}"
    depends: [build]
  - id: deploy
    task: "Deploy {{.project}} to {{.env}}"
    depends: [test]
```

## Design Decisions

### Simple String Replacement

Uses `strings.ReplaceAll` instead of `text/template` to avoid complexity. The `{{.VarName}}` syntax is familiar but implementation is simple string replacement.

### Templates as Data

Templates produce `dag.NodeSpec` slices — they don't create DAGs directly. The caller passes the specs to `dag.New()`. This keeps the template package decoupled from DAG execution.

### Registry with Mutex

Thread-safe with `sync.RWMutex`, consistent with project patterns.

### LoadDir Resilience

Skips invalid files, continues on error. Matches the pattern in datapuller, credinjector, and profile packages.

## CLI Commands

### `apex template list [--format json]`
Lists all registered templates.

### `apex template show <name>`
Shows template details including variables and nodes.

### `apex template expand <name> [--var key=value ...]`
Expands a template with given variables, shows resulting node specs.

## Unit Tests (7)

| Test | Description |
|------|-------------|
| `TestLoad` | Parse valid YAML → correct Template fields |
| `TestLoadInvalid` | Invalid YAML → error |
| `TestExpand` | Substitute variables, verify NodeSpec task strings |
| `TestExpandDefaults` | Missing vars filled from defaults |
| `TestRegistryRegisterGet` | Register and retrieve template |
| `TestRegistryList` | Multiple templates sorted by name |
| `TestVarNames` | Returns correct variable names |

## E2E Tests (3)

| Test | Description |
|------|-------------|
| `TestTemplateList` | CLI lists templates |
| `TestTemplateShow` | CLI shows template details |
| `TestTemplateExpand` | CLI expands template with vars |

## Format Functions

| Function | Description |
|----------|-------------|
| `FormatTemplateList(templates []Template) string` | Table: NAME / DESCRIPTION / VARS / NODES |
| `FormatTemplate(t Template) string` | Detailed template display with vars and nodes |
| `FormatNodeSpecs(specs []dag.NodeSpec) string` | Expanded node spec display |
| `FormatTemplateListJSON(templates []Template) (string, error)` | JSON output |

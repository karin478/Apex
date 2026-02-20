# External Data Puller Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an `internal/datapuller` package that loads YAML data source specs, executes HTTP pulls with auth, applies JSON path transforms, and exposes CLI commands.

**Architecture:** SourceSpec YAML loading (like `internal/connector`), HTTPClient interface for testable pulls, simple JSON path transform, format functions + Cobra CLI commands.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, `encoding/json`, `net/http`, `net/http/httptest`, Testify, Cobra CLI

---

### Task 1: Data Puller Core — SourceSpec + Pull + Transform (7 tests)

**Files:**
- Create: `internal/datapuller/datapuller.go`
- Create: `internal/datapuller/datapuller_test.go`

**Step 1: Write 7 failing tests**

```go
package datapuller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSpec(t *testing.T) {
	dir := t.TempDir()
	content := `name: test-feed
url: https://api.example.com/data
schedule: "*/5 * * * *"
auth_type: bearer
auth_token: "$TEST_TOKEN"
headers:
  Accept: application/json
transform: ".items"
emit_event: data.fetched
max_retries: 3
`
	path := filepath.Join(dir, "test-feed.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	spec, err := LoadSpec(path)
	require.NoError(t, err)
	assert.Equal(t, "test-feed", spec.Name)
	assert.Equal(t, "https://api.example.com/data", spec.URL)
	assert.Equal(t, "*/5 * * * *", spec.Schedule)
	assert.Equal(t, "bearer", spec.AuthType)
	assert.Equal(t, "$TEST_TOKEN", spec.AuthToken)
	assert.Equal(t, "application/json", spec.Headers["Accept"])
	assert.Equal(t, ".items", spec.Transform)
	assert.Equal(t, "data.fetched", spec.EmitEvent)
	assert.Equal(t, 3, spec.MaxRetries)
}

func TestLoadSpecInvalid(t *testing.T) {
	dir := t.TempDir()

	t.Run("missing file", func(t *testing.T) {
		_, err := LoadSpec(filepath.Join(dir, "nope.yaml"))
		assert.Error(t, err)
	})

	t.Run("invalid yaml", func(t *testing.T) {
		p := filepath.Join(dir, "bad.yaml")
		require.NoError(t, os.WriteFile(p, []byte(":::"), 0644))
		_, err := LoadSpec(p)
		assert.Error(t, err)
	})

	t.Run("missing name", func(t *testing.T) {
		p := filepath.Join(dir, "noname.yaml")
		require.NoError(t, os.WriteFile(p, []byte("url: https://x.com\n"), 0644))
		_, err := LoadSpec(p)
		assert.Error(t, err)
	})

	t.Run("missing url", func(t *testing.T) {
		p := filepath.Join(dir, "nourl.yaml")
		require.NoError(t, os.WriteFile(p, []byte("name: foo\n"), 0644))
		_, err := LoadSpec(p)
		assert.Error(t, err)
	})
}

func TestLoadDir(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.yaml", "b.yml", "c.txt"} {
		content := "name: " + name + "\nurl: https://example.com\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
	}

	specs, err := LoadDir(dir)
	require.NoError(t, err)
	assert.Len(t, specs, 2, "should load .yaml and .yml but not .txt")
}

func TestValidateSpec(t *testing.T) {
	assert.Error(t, ValidateSpec(SourceSpec{}), "empty spec should fail")
	assert.Error(t, ValidateSpec(SourceSpec{Name: "x"}), "missing URL should fail")
	assert.Error(t, ValidateSpec(SourceSpec{URL: "http://x"}), "missing name should fail")
	assert.NoError(t, ValidateSpec(SourceSpec{Name: "x", URL: "http://x"}))
}

func TestResolveAuth(t *testing.T) {
	t.Run("bearer with env var", func(t *testing.T) {
		t.Setenv("TEST_PULLER_TOKEN", "secret123")
		spec := SourceSpec{AuthType: "bearer", AuthToken: "$TEST_PULLER_TOKEN"}
		token, err := ResolveAuth(spec)
		require.NoError(t, err)
		assert.Equal(t, "secret123", token)
	})

	t.Run("bearer missing env var", func(t *testing.T) {
		spec := SourceSpec{AuthType: "bearer", AuthToken: "$MISSING_VAR_XYZ"}
		_, err := ResolveAuth(spec)
		assert.Error(t, err)
	})

	t.Run("none auth type", func(t *testing.T) {
		spec := SourceSpec{AuthType: "none"}
		token, err := ResolveAuth(spec)
		require.NoError(t, err)
		assert.Empty(t, token)
	})

	t.Run("empty auth type", func(t *testing.T) {
		spec := SourceSpec{}
		token, err := ResolveAuth(spec)
		require.NoError(t, err)
		assert.Empty(t, token)
	})
}

func TestPull(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer tok123", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"items": []string{"a", "b"},
		})
	}))
	defer server.Close()

	spec := SourceSpec{
		Name:      "test",
		URL:       server.URL,
		AuthType:  "bearer",
		AuthToken: "$PULL_TEST_TOKEN",
		Headers:   map[string]string{"Accept": "application/json"},
		Transform: ".items",
		EmitEvent: "data.fetched",
	}
	t.Setenv("PULL_TEST_TOKEN", "tok123")

	result := Pull(spec, server.Client())
	require.NoError(t, result.Error)
	assert.Equal(t, 200, result.StatusCode)
	assert.Equal(t, "test", result.Source)
	assert.Equal(t, "data.fetched", result.EventEmitted)
	assert.Greater(t, result.RawBytes, 0)
	assert.NotEmpty(t, result.Transformed)
}

func TestApplyTransform(t *testing.T) {
	data := []byte(`{"items":[{"name":"a"},{"name":"b"}],"count":2}`)

	t.Run("top-level field", func(t *testing.T) {
		out, err := ApplyTransform(data, ".count")
		require.NoError(t, err)
		assert.Equal(t, "2", string(out))
	})

	t.Run("nested array", func(t *testing.T) {
		out, err := ApplyTransform(data, ".items")
		require.NoError(t, err)
		var items []any
		require.NoError(t, json.Unmarshal(out, &items))
		assert.Len(t, items, 2)
	})

	t.Run("empty transform", func(t *testing.T) {
		out, err := ApplyTransform(data, "")
		require.NoError(t, err)
		assert.Equal(t, data, out, "empty transform should return raw data")
	})

	t.Run("missing field", func(t *testing.T) {
		_, err := ApplyTransform(data, ".nonexistent")
		assert.Error(t, err)
	})
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/datapuller/ -v -count=1`
Expected: FAIL — functions not defined

**Step 3: Write minimal implementation**

```go
// Package datapuller provides YAML-driven external data source loading,
// HTTP pulling with auth, and JSON path transforms.
package datapuller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SourceSpec defines an external data source loaded from YAML.
type SourceSpec struct {
	Name       string            `yaml:"name" json:"name"`
	URL        string            `yaml:"url" json:"url"`
	Schedule   string            `yaml:"schedule" json:"schedule"`
	AuthType   string            `yaml:"auth_type" json:"auth_type"`
	AuthToken  string            `yaml:"auth_token" json:"auth_token"`
	Headers    map[string]string `yaml:"headers" json:"headers"`
	Transform  string            `yaml:"transform" json:"transform"`
	EmitEvent  string            `yaml:"emit_event" json:"emit_event"`
	MaxRetries int               `yaml:"max_retries" json:"max_retries"`
}

// PullResult holds the outcome of a single data pull.
type PullResult struct {
	Source       string    `json:"source"`
	StatusCode   int       `json:"status_code"`
	RawBytes     int       `json:"raw_bytes"`
	Transformed  []byte    `json:"transformed"`
	EventEmitted string    `json:"event_emitted"`
	PulledAt     time.Time `json:"pulled_at"`
	Error        error     `json:"-"`
}

// HTTPClient is an interface for HTTP request execution (for testability).
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// LoadSpec reads and parses a YAML data source spec from the given path.
func LoadSpec(path string) (*SourceSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("datapuller: read spec: %w", err)
	}

	var spec SourceSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("datapuller: parse spec: %w", err)
	}

	if err := ValidateSpec(spec); err != nil {
		return nil, err
	}

	return &spec, nil
}

// LoadDir loads all *.yaml and *.yml files from the given directory.
func LoadDir(dir string) ([]*SourceSpec, error) {
	var specs []*SourceSpec
	for _, ext := range []string{"*.yaml", "*.yml"} {
		matches, err := filepath.Glob(filepath.Join(dir, ext))
		if err != nil {
			return nil, fmt.Errorf("datapuller: glob %s: %w", ext, err)
		}
		for _, path := range matches {
			spec, err := LoadSpec(path)
			if err != nil {
				return nil, err
			}
			specs = append(specs, spec)
		}
	}
	return specs, nil
}

// ValidateSpec checks that required fields (Name, URL) are present.
func ValidateSpec(spec SourceSpec) error {
	if spec.Name == "" {
		return fmt.Errorf("datapuller: spec missing required field: name")
	}
	if spec.URL == "" {
		return fmt.Errorf("datapuller: spec missing required field: url")
	}
	return nil
}

// ResolveAuth resolves the auth token from an environment variable reference.
// AuthToken format: "$ENV_VAR_NAME". Returns empty string for auth_type "none" or empty.
func ResolveAuth(spec SourceSpec) (string, error) {
	if spec.AuthType == "" || spec.AuthType == "none" {
		return "", nil
	}

	if !strings.HasPrefix(spec.AuthToken, "$") {
		return spec.AuthToken, nil
	}

	envName := strings.TrimPrefix(spec.AuthToken, "$")
	val := os.Getenv(envName)
	if val == "" {
		return "", fmt.Errorf("datapuller: env var %s not set", envName)
	}
	return val, nil
}

// Pull executes an HTTP GET against the spec's URL with auth and headers,
// then applies the optional transform.
func Pull(spec SourceSpec, client HTTPClient) PullResult {
	result := PullResult{
		Source:   spec.Name,
		PulledAt: time.Now(),
	}

	req, err := http.NewRequest(http.MethodGet, spec.URL, nil)
	if err != nil {
		result.Error = fmt.Errorf("datapuller: create request: %w", err)
		return result
	}

	// Resolve and set auth header.
	token, err := ResolveAuth(spec)
	if err != nil {
		result.Error = err
		return result
	}
	if token != "" && spec.AuthType == "bearer" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Set custom headers.
	for k, v := range spec.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		result.Error = fmt.Errorf("datapuller: http request: %w", err)
		return result
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Errorf("datapuller: read body: %w", err)
		return result
	}

	result.StatusCode = resp.StatusCode
	result.RawBytes = len(body)

	// Apply transform.
	transformed, err := ApplyTransform(body, spec.Transform)
	if err != nil {
		result.Error = fmt.Errorf("datapuller: transform: %w", err)
		return result
	}
	result.Transformed = transformed
	result.EventEmitted = spec.EmitEvent

	return result
}

// ApplyTransform extracts a JSON path from data. Supports ".field" notation
// for top-level field access. Empty expression returns raw data.
func ApplyTransform(data []byte, expr string) ([]byte, error) {
	if expr == "" {
		return data, nil
	}

	// Parse the JSON into a generic map.
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Strip leading dot.
	field := strings.TrimPrefix(expr, ".")
	if field == "" {
		return data, nil
	}

	val, ok := root[field]
	if !ok {
		return nil, fmt.Errorf("field %q not found in JSON", field)
	}

	out, err := json.Marshal(val)
	if err != nil {
		return nil, fmt.Errorf("marshal transformed value: %w", err)
	}
	return out, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/datapuller/ -v -count=1 -race`
Expected: PASS (7 tests)

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/datapuller/datapuller.go internal/datapuller/datapuller_test.go
git commit -m "feat(datapuller): add external data puller with spec loading, HTTP pull, and JSON transform

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: Format Functions + CLI Commands

**Files:**
- Create: `internal/datapuller/format.go`
- Create: `cmd/apex/datasource.go`
- Modify: `cmd/apex/main.go:52` (add `rootCmd.AddCommand(datasourceCmd)`)

**Step 1: Write format.go**

```go
package datapuller

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatSourceList formats a slice of source specs as a human-readable table.
func FormatSourceList(specs []*SourceSpec) string {
	if len(specs) == 0 {
		return "No data sources configured."
	}

	nameW := len("NAME")
	urlW := len("URL")
	schedW := len("SCHEDULE")
	authW := len("AUTH")

	for _, s := range specs {
		if l := len(s.Name); l > nameW {
			nameW = l
		}
		if l := len(s.URL); l > urlW {
			urlW = l
		}
		if l := len(s.Schedule); l > schedW {
			schedW = l
		}
		if l := len(s.AuthType); l > authW {
			authW = l
		}
	}

	var b strings.Builder
	rowFmt := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%s\n", nameW, urlW, schedW)
	fmt.Fprintf(&b, rowFmt, "NAME", "URL", "SCHEDULE", "AUTH")
	for _, s := range specs {
		auth := s.AuthType
		if auth == "" {
			auth = "none"
		}
		fmt.Fprintf(&b, rowFmt, s.Name, s.URL, s.Schedule, auth)
	}
	return b.String()
}

// FormatPullResult formats a pull result as a human-readable summary.
func FormatPullResult(result PullResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Source:    %s\n", result.Source)
	fmt.Fprintf(&b, "Status:   %d\n", result.StatusCode)
	fmt.Fprintf(&b, "Bytes:    %d\n", result.RawBytes)
	if result.EventEmitted != "" {
		fmt.Fprintf(&b, "Event:    %s\n", result.EventEmitted)
	}
	fmt.Fprintf(&b, "Pulled:   %s\n", result.PulledAt.Format("2006-01-02 15:04:05"))
	if result.Error != nil {
		fmt.Fprintf(&b, "Error:    %v\n", result.Error)
	}
	return b.String()
}

// FormatSourceListJSON formats source specs as indented JSON.
func FormatSourceListJSON(specs []*SourceSpec) (string, error) {
	if len(specs) == 0 {
		return "[]", nil
	}
	data, err := json.MarshalIndent(specs, "", "  ")
	if err != nil {
		return "", fmt.Errorf("datapuller: json marshal: %w", err)
	}
	return string(data), nil
}
```

**Step 2: Write datasource.go CLI**

```go
package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/datapuller"
	"github.com/spf13/cobra"
)

var datasourceFormat string

var datasourceCmd = &cobra.Command{
	Use:   "datasource",
	Short: "Manage external data sources",
}

var datasourceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured data sources",
	RunE:  runDatasourceList,
}

var datasourcePullCmd = &cobra.Command{
	Use:   "pull [name]",
	Short: "Pull data from a named source",
	Args:  cobra.ExactArgs(1),
	RunE:  runDatasourcePull,
}

var datasourceValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate data source YAML specs",
	RunE:  runDatasourceValidate,
}

func init() {
	datasourceListCmd.Flags().StringVar(&datasourceFormat, "format", "", "Output format (json)")
	datasourceCmd.AddCommand(datasourceListCmd, datasourcePullCmd, datasourceValidateCmd)
}

func datasourceDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("datapuller: home dir: %w", err)
	}
	return filepath.Join(home, ".claude", "data_sources"), nil
}

func loadAllSources() ([]*datapuller.SourceSpec, error) {
	dir, err := datasourceDir()
	if err != nil {
		return nil, err
	}
	return datapuller.LoadDir(dir)
}

func runDatasourceList(cmd *cobra.Command, args []string) error {
	specs, err := loadAllSources()
	if err != nil {
		return err
	}

	if datasourceFormat == "json" {
		out, err := datapuller.FormatSourceListJSON(specs)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(datapuller.FormatSourceList(specs))
	}
	return nil
}

func runDatasourcePull(cmd *cobra.Command, args []string) error {
	specs, err := loadAllSources()
	if err != nil {
		return err
	}

	name := args[0]
	var target *datapuller.SourceSpec
	for _, s := range specs {
		if s.Name == name {
			target = s
			break
		}
	}
	if target == nil {
		return fmt.Errorf("data source %q not found", name)
	}

	result := datapuller.Pull(*target, http.DefaultClient)
	fmt.Print(datapuller.FormatPullResult(result))
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func runDatasourceValidate(cmd *cobra.Command, args []string) error {
	dir, err := datasourceDir()
	if err != nil {
		return err
	}

	var paths []string
	for _, ext := range []string{"*.yaml", "*.yml"} {
		matches, _ := filepath.Glob(filepath.Join(dir, ext))
		paths = append(paths, matches...)
	}

	if len(paths) == 0 {
		fmt.Println("No data source specs found.")
		return nil
	}

	hasErrors := false
	for _, p := range paths {
		spec, err := datapuller.LoadSpec(p)
		if err != nil {
			fmt.Printf("FAIL  %s: %v\n", filepath.Base(p), err)
			hasErrors = true
			continue
		}
		fmt.Printf("OK    %s (%s)\n", filepath.Base(p), spec.Name)
	}
	if hasErrors {
		return fmt.Errorf("one or more specs failed validation")
	}
	return nil
}
```

**Step 3: Add command to main.go**

Add `rootCmd.AddCommand(datasourceCmd)` after the `migrationCmd` line in `cmd/apex/main.go`.

**Step 4: Run build + tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build ./cmd/apex/ && go test ./internal/datapuller/ -v -count=1`
Expected: BUILD OK, PASS

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/datapuller/format.go cmd/apex/datasource.go cmd/apex/main.go
git commit -m "feat(datapuller): add format functions and CLI for datasource list/pull/validate

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: E2E Tests (3 tests)

**Files:**
- Create: `e2e/datasource_test.go`

**Step 1: Write E2E tests**

```go
package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatasourceList(t *testing.T) {
	env := newTestEnv(t)

	dsDir := filepath.Join(env.Home, ".claude", "data_sources")
	require.NoError(t, os.MkdirAll(dsDir, 0755))

	specContent := `name: test-feed
url: https://api.example.com/data
schedule: "*/5 * * * *"
auth_type: none
`
	require.NoError(t, os.WriteFile(
		filepath.Join(dsDir, "test-feed.yaml"),
		[]byte(specContent), 0644))

	stdout, stderr, exitCode := env.runApex("datasource", "list")

	assert.Equal(t, 0, exitCode,
		"apex datasource list should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "test-feed"),
		"stdout should contain source name 'test-feed', got: %s", stdout)
}

func TestDatasourceValidate(t *testing.T) {
	env := newTestEnv(t)

	dsDir := filepath.Join(env.Home, ".claude", "data_sources")
	require.NoError(t, os.MkdirAll(dsDir, 0755))

	// Valid spec
	require.NoError(t, os.WriteFile(
		filepath.Join(dsDir, "good.yaml"),
		[]byte("name: good\nurl: https://example.com\n"), 0644))

	// Invalid spec (missing url)
	require.NoError(t, os.WriteFile(
		filepath.Join(dsDir, "bad.yaml"),
		[]byte("name: bad\n"), 0644))

	stdout, _, exitCode := env.runApex("datasource", "validate")

	assert.NotEqual(t, 0, exitCode,
		"apex datasource validate should exit non-zero when a spec fails")
	assert.True(t, strings.Contains(stdout, "OK"),
		"stdout should contain OK for valid spec, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "FAIL"),
		"stdout should contain FAIL for invalid spec, got: %s", stdout)
}

func TestDatasourceListEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("datasource", "list")

	assert.Equal(t, 0, exitCode,
		"apex datasource list should exit 0 with empty dir; stderr=%s", stderr)

	lower := strings.ToLower(stdout)
	assert.True(t, strings.Contains(lower, "no data sources"),
		"stdout should contain 'No data sources' (case-insensitive), got: %s", stdout)
}
```

**Step 2: Build and run E2E tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test -tags e2e ./e2e/ -run TestDatasource -v -count=1`
Expected: PASS (3 tests)

**Step 3: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add e2e/datasource_test.go
git commit -m "test(e2e): add E2E tests for datasource list, validate, and empty list

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Update PROGRESS.md

**Files:**
- Modify: `PROGRESS.md`

**Step 1: Update completed phases table**

Add row: `| 34 | External Data Puller | \`2026-02-20-phase34-external-data-puller-design.md\` | Done |`

**Step 2: Update Current section**

Change "Phase 34" → "Phase 35 — TBD"

**Step 3: Update test counts**

- Unit tests: 39 → 40 packages
- E2E tests: 99 → 102 tests

**Step 4: Add Key Package**

Add: `| \`internal/datapuller\` | External data puller with YAML spec loading, HTTP pull with auth, and JSON path transform |`

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add PROGRESS.md
git commit -m "docs: mark Phase 34 External Data Puller as complete

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

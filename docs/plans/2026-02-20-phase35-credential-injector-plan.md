# Credential Injector Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an `internal/credinjector` package that manages credential placeholders, resolves secrets at the execution boundary, and scrubs leaked values from error paths.

**Architecture:** YAML vault loading (like `internal/datapuller`), `<NAME_REF>` placeholder regex matching, `Inject` for forward resolution, `Scrub` for reverse replacement, format functions + Cobra CLI.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, `regexp`, `encoding/json`, Testify, Cobra CLI

---

### Task 1: Credential Injector Core — Vault + Resolve + Inject + Scrub (7 tests)

**Files:**
- Create: `internal/credinjector/credinjector.go`
- Create: `internal/credinjector/credinjector_test.go`

**Step 1: Write 7 failing tests**

```go
package credinjector

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadVault(t *testing.T) {
	dir := t.TempDir()
	content := `credentials:
  - placeholder: "<GRAFANA_TOKEN_REF>"
    source: env
    key: GRAFANA_TOKEN
  - placeholder: "<SLACK_WEBHOOK_REF>"
    source: file
    key: /tmp/slack-webhook.txt
`
	path := filepath.Join(dir, "creds.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	vault, err := LoadVault(path)
	require.NoError(t, err)
	require.Len(t, vault.Credentials, 2)
	assert.Equal(t, "<GRAFANA_TOKEN_REF>", vault.Credentials[0].Placeholder)
	assert.Equal(t, "env", vault.Credentials[0].Source)
	assert.Equal(t, "GRAFANA_TOKEN", vault.Credentials[0].Key)
	assert.Equal(t, "<SLACK_WEBHOOK_REF>", vault.Credentials[1].Placeholder)
	assert.Equal(t, "file", vault.Credentials[1].Source)
}

func TestLoadVaultInvalid(t *testing.T) {
	dir := t.TempDir()

	t.Run("missing file", func(t *testing.T) {
		_, err := LoadVault(filepath.Join(dir, "nope.yaml"))
		assert.Error(t, err)
	})

	t.Run("invalid yaml", func(t *testing.T) {
		p := filepath.Join(dir, "bad.yaml")
		require.NoError(t, os.WriteFile(p, []byte(":::"), 0644))
		_, err := LoadVault(p)
		assert.Error(t, err)
	})
}

func TestLoadVaultDir(t *testing.T) {
	dir := t.TempDir()

	f1 := `credentials:
  - placeholder: "<TOKEN_A_REF>"
    source: env
    key: TOKEN_A
`
	f2 := `credentials:
  - placeholder: "<TOKEN_B_REF>"
    source: env
    key: TOKEN_B
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(f1), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.yml"), []byte(f2), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.txt"), []byte("ignored"), 0644))

	vault, err := LoadVaultDir(dir)
	require.NoError(t, err)
	assert.Len(t, vault.Credentials, 2, "should merge .yaml and .yml, ignore .txt")
}

func TestResolve(t *testing.T) {
	t.Run("env source", func(t *testing.T) {
		t.Setenv("TEST_CRED_TOKEN", "my-secret")
		ref := CredentialRef{Placeholder: "<TEST_CRED_REF>", Source: "env", Key: "TEST_CRED_TOKEN"}
		val, err := Resolve(ref)
		require.NoError(t, err)
		assert.Equal(t, "my-secret", val)
	})

	t.Run("env source missing", func(t *testing.T) {
		ref := CredentialRef{Placeholder: "<MISSING_REF>", Source: "env", Key: "NONEXISTENT_VAR_XYZ"}
		_, err := Resolve(ref)
		assert.Error(t, err)
	})

	t.Run("file source", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "secret.txt")
		require.NoError(t, os.WriteFile(f, []byte("  file-secret\n"), 0644))
		ref := CredentialRef{Placeholder: "<FILE_REF>", Source: "file", Key: f}
		val, err := Resolve(ref)
		require.NoError(t, err)
		assert.Equal(t, "file-secret", val, "should trim whitespace")
	})

	t.Run("file source missing", func(t *testing.T) {
		ref := CredentialRef{Placeholder: "<GONE_REF>", Source: "file", Key: "/tmp/nonexistent-cred-file"}
		_, err := Resolve(ref)
		assert.Error(t, err)
	})

	t.Run("unknown source", func(t *testing.T) {
		ref := CredentialRef{Placeholder: "<X_REF>", Source: "vault", Key: "x"}
		_, err := Resolve(ref)
		assert.Error(t, err)
	})
}

func TestValidateVault(t *testing.T) {
	t.Setenv("VALID_CRED", "ok")

	vault := &Vault{
		Credentials: []CredentialRef{
			{Placeholder: "<VALID_REF>", Source: "env", Key: "VALID_CRED"},
			{Placeholder: "<INVALID_REF>", Source: "env", Key: "NONEXISTENT_CRED_XYZ"},
		},
	}

	errs := ValidateVault(vault)
	assert.Len(t, errs, 1, "should have 1 error for the unresolvable ref")
	assert.Contains(t, errs[0].Error(), "NONEXISTENT_CRED_XYZ")
}

func TestInject(t *testing.T) {
	t.Setenv("INJ_TOKEN", "real-secret")

	vault := &Vault{
		Credentials: []CredentialRef{
			{Placeholder: "<INJ_TOKEN_REF>", Source: "env", Key: "INJ_TOKEN"},
			{Placeholder: "<MISSING_TOKEN_REF>", Source: "env", Key: "NOPE_XYZ"},
		},
	}

	template := "Authorization: Bearer <INJ_TOKEN_REF> and also <MISSING_TOKEN_REF> here"
	result := Inject(template, vault)

	assert.Equal(t, "Authorization: Bearer real-secret and also <MISSING_TOKEN_REF> here", result.Output)
	assert.Equal(t, []string{"<INJ_TOKEN_REF>"}, result.Injected)
	assert.Equal(t, []string{"<MISSING_TOKEN_REF>"}, result.Unresolved)
}

func TestScrub(t *testing.T) {
	t.Setenv("SCRUB_SHORT", "abc")
	t.Setenv("SCRUB_LONG", "abcdef")

	vault := &Vault{
		Credentials: []CredentialRef{
			{Placeholder: "<SHORT_REF>", Source: "env", Key: "SCRUB_SHORT"},
			{Placeholder: "<LONG_REF>", Source: "env", Key: "SCRUB_LONG"},
		},
	}

	text := "error: token abcdef was rejected, also abc appeared"
	scrubbed := Scrub(text, vault)

	assert.Contains(t, scrubbed, "<LONG_REF>", "longer value should be replaced first")
	assert.Contains(t, scrubbed, "<SHORT_REF>")
	assert.NotContains(t, scrubbed, "abcdef")
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/credinjector/ -v -count=1`
Expected: FAIL — functions not defined

**Step 3: Write minimal implementation**

```go
// Package credinjector provides zero-trust credential injection with
// placeholder-based secret management. Agents see only placeholders
// like <NAME_REF>; real values are injected at the execution boundary.
package credinjector

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// placeholderRe matches credential placeholders like <GRAFANA_TOKEN_REF>.
var placeholderRe = regexp.MustCompile(`<[A-Z][A-Z0-9_]*_REF>`)

// CredentialRef maps a placeholder to a secret source.
type CredentialRef struct {
	Placeholder string `yaml:"placeholder" json:"placeholder"`
	Source      string `yaml:"source" json:"source"`
	Key         string `yaml:"key" json:"key"`
}

// Vault holds a collection of credential references.
type Vault struct {
	Credentials []CredentialRef `yaml:"credentials" json:"credentials"`
}

// InjectionResult holds the outcome of injecting credentials into a template.
type InjectionResult struct {
	Output     string   `json:"output"`
	Injected   []string `json:"injected"`
	Unresolved []string `json:"unresolved"`
}

// LoadVault reads and parses a YAML vault file.
func LoadVault(path string) (*Vault, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("credinjector: read vault: %w", err)
	}

	var vault Vault
	if err := yaml.Unmarshal(data, &vault); err != nil {
		return nil, fmt.Errorf("credinjector: parse vault: %w", err)
	}

	return &vault, nil
}

// LoadVaultDir loads and merges all *.yaml and *.yml files from a directory.
func LoadVaultDir(dir string) (*Vault, error) {
	merged := &Vault{}
	for _, ext := range []string{"*.yaml", "*.yml"} {
		matches, err := filepath.Glob(filepath.Join(dir, ext))
		if err != nil {
			return nil, fmt.Errorf("credinjector: glob %s: %w", ext, err)
		}
		for _, path := range matches {
			v, err := LoadVault(path)
			if err != nil {
				continue // skip invalid files
			}
			merged.Credentials = append(merged.Credentials, v.Credentials...)
		}
	}
	return merged, nil
}

// Resolve resolves a single credential reference to its real value.
func Resolve(ref CredentialRef) (string, error) {
	switch ref.Source {
	case "env":
		val := os.Getenv(ref.Key)
		if val == "" {
			return "", fmt.Errorf("credinjector: env var %s not set", ref.Key)
		}
		return val, nil

	case "file":
		data, err := os.ReadFile(ref.Key)
		if err != nil {
			return "", fmt.Errorf("credinjector: read file %s: %w", ref.Key, err)
		}
		return strings.TrimSpace(string(data)), nil

	default:
		return "", fmt.Errorf("credinjector: unknown source type %q", ref.Source)
	}
}

// ValidateVault attempts to resolve all credentials and returns per-ref errors.
func ValidateVault(vault *Vault) []error {
	var errs []error
	for _, ref := range vault.Credentials {
		if _, err := Resolve(ref); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// Inject replaces <..._REF> placeholders in template with resolved values.
// Unresolvable placeholders are left as-is and tracked in Unresolved.
func Inject(template string, vault *Vault) InjectionResult {
	result := InjectionResult{Output: template}

	// Build lookup map.
	lookup := make(map[string]CredentialRef)
	for _, ref := range vault.Credentials {
		lookup[ref.Placeholder] = ref
	}

	// Find all placeholders in the template.
	matches := placeholderRe.FindAllString(template, -1)
	seen := make(map[string]bool)

	for _, ph := range matches {
		if seen[ph] {
			continue
		}
		seen[ph] = true

		ref, ok := lookup[ph]
		if !ok {
			result.Unresolved = append(result.Unresolved, ph)
			continue
		}

		val, err := Resolve(ref)
		if err != nil {
			result.Unresolved = append(result.Unresolved, ph)
			continue
		}

		result.Output = strings.ReplaceAll(result.Output, ph, val)
		result.Injected = append(result.Injected, ph)
	}

	return result
}

// Scrub replaces resolved credential values in text with their placeholders.
// Longer values are replaced first to avoid partial matches.
func Scrub(text string, vault *Vault) string {
	type pair struct {
		value       string
		placeholder string
	}

	var pairs []pair
	for _, ref := range vault.Credentials {
		val, err := Resolve(ref)
		if err != nil || val == "" {
			continue
		}
		pairs = append(pairs, pair{value: val, placeholder: ref.Placeholder})
	}

	// Sort by value length descending — replace longer values first.
	sort.Slice(pairs, func(i, j int) bool {
		return len(pairs[i].value) > len(pairs[j].value)
	})

	for _, p := range pairs {
		text = strings.ReplaceAll(text, p.value, p.placeholder)
	}

	return text
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/credinjector/ -v -count=1 -race`
Expected: PASS (7 tests)

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/credinjector/credinjector.go internal/credinjector/credinjector_test.go
git commit -m "feat(credinjector): add credential injector with vault loading, resolve, inject, and scrub

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: Format Functions + CLI Commands

**Files:**
- Create: `internal/credinjector/format.go`
- Create: `cmd/apex/credential.go`
- Modify: `cmd/apex/main.go:52` (add `rootCmd.AddCommand(credentialCmd)`)

**Step 1: Write format.go**

```go
package credinjector

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatCredentialList formats vault credentials as a human-readable table.
func FormatCredentialList(vault *Vault) string {
	if vault == nil || len(vault.Credentials) == 0 {
		return "No credentials configured."
	}

	phW := len("PLACEHOLDER")
	srcW := len("SOURCE")
	keyW := len("KEY")

	for _, c := range vault.Credentials {
		if l := len(c.Placeholder); l > phW {
			phW = l
		}
		if l := len(c.Source); l > srcW {
			srcW = l
		}
		if l := len(c.Key); l > keyW {
			keyW = l
		}
	}

	var b strings.Builder
	rowFmt := fmt.Sprintf("%%-%ds  %%-%ds  %%s\n", phW, srcW)
	fmt.Fprintf(&b, rowFmt, "PLACEHOLDER", "SOURCE", "KEY")
	for _, c := range vault.Credentials {
		fmt.Fprintf(&b, rowFmt, c.Placeholder, c.Source, c.Key)
	}
	return b.String()
}

// FormatValidationResult formats validation results as OK/FAIL per credential.
func FormatValidationResult(vault *Vault) string {
	var b strings.Builder
	for _, ref := range vault.Credentials {
		_, err := Resolve(ref)
		if err != nil {
			fmt.Fprintf(&b, "FAIL  %s: %v\n", ref.Placeholder, err)
		} else {
			fmt.Fprintf(&b, "OK    %s\n", ref.Placeholder)
		}
	}
	return b.String()
}

// FormatCredentialListJSON formats vault credentials as indented JSON.
func FormatCredentialListJSON(vault *Vault) (string, error) {
	if vault == nil || len(vault.Credentials) == 0 {
		return "[]", nil
	}
	data, err := json.MarshalIndent(vault.Credentials, "", "  ")
	if err != nil {
		return "", fmt.Errorf("credinjector: json marshal: %w", err)
	}
	return string(data), nil
}
```

**Step 2: Write credential.go CLI**

```go
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/credinjector"
	"github.com/spf13/cobra"
)

var credentialFormat string

var credentialCmd = &cobra.Command{
	Use:   "credential",
	Short: "Manage credential references",
}

var credentialListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured credential references",
	RunE:  runCredentialList,
}

var credentialValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate all credential references are resolvable",
	RunE:  runCredentialValidate,
}

var credentialScrubCmd = &cobra.Command{
	Use:   "scrub",
	Short: "Scrub credential values from stdin text",
	RunE:  runCredentialScrub,
}

func init() {
	credentialListCmd.Flags().StringVar(&credentialFormat, "format", "", "Output format (json)")
	credentialCmd.AddCommand(credentialListCmd, credentialValidateCmd, credentialScrubCmd)
}

func credentialDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("credinjector: home dir: %w", err)
	}
	return filepath.Join(home, ".claude", "credentials"), nil
}

func loadVault() (*credinjector.Vault, error) {
	dir, err := credentialDir()
	if err != nil {
		return nil, err
	}
	return credinjector.LoadVaultDir(dir)
}

func runCredentialList(cmd *cobra.Command, args []string) error {
	vault, err := loadVault()
	if err != nil {
		return err
	}

	if credentialFormat == "json" {
		out, err := credinjector.FormatCredentialListJSON(vault)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(credinjector.FormatCredentialList(vault))
	}
	return nil
}

func runCredentialValidate(cmd *cobra.Command, args []string) error {
	vault, err := loadVault()
	if err != nil {
		return err
	}

	if len(vault.Credentials) == 0 {
		fmt.Println("No credentials configured.")
		return nil
	}

	fmt.Print(credinjector.FormatValidationResult(vault))

	errs := credinjector.ValidateVault(vault)
	if len(errs) > 0 {
		return fmt.Errorf("%d credential(s) failed validation", len(errs))
	}
	return nil
}

func runCredentialScrub(cmd *cobra.Command, args []string) error {
	vault, err := loadVault()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		fmt.Println(credinjector.Scrub(scanner.Text(), vault))
	}
	return scanner.Err()
}
```

**Step 3: Add command to main.go**

Add `rootCmd.AddCommand(credentialCmd)` after the `datasourceCmd` line in `cmd/apex/main.go`.

**Step 4: Run build + tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build ./cmd/apex/ && go test ./internal/credinjector/ -v -count=1`
Expected: BUILD OK, PASS

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/credinjector/format.go cmd/apex/credential.go cmd/apex/main.go
git commit -m "feat(credinjector): add format functions and CLI for credential list/validate/scrub

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: E2E Tests (3 tests)

**Files:**
- Create: `e2e/credential_test.go`

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

func TestCredentialList(t *testing.T) {
	env := newTestEnv(t)

	credDir := filepath.Join(env.Home, ".claude", "credentials")
	require.NoError(t, os.MkdirAll(credDir, 0755))

	specContent := `credentials:
  - placeholder: "<API_KEY_REF>"
    source: env
    key: API_KEY
`
	require.NoError(t, os.WriteFile(
		filepath.Join(credDir, "api.yaml"),
		[]byte(specContent), 0644))

	stdout, stderr, exitCode := env.runApex("credential", "list")

	assert.Equal(t, 0, exitCode,
		"apex credential list should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "<API_KEY_REF>"),
		"stdout should contain placeholder, got: %s", stdout)
}

func TestCredentialValidate(t *testing.T) {
	env := newTestEnv(t)

	credDir := filepath.Join(env.Home, ".claude", "credentials")
	require.NoError(t, os.MkdirAll(credDir, 0755))

	specContent := `credentials:
  - placeholder: "<GOOD_REF>"
    source: env
    key: CRED_TEST_GOOD
  - placeholder: "<BAD_REF>"
    source: env
    key: CRED_NONEXISTENT_XYZ
`
	require.NoError(t, os.WriteFile(
		filepath.Join(credDir, "mixed.yaml"),
		[]byte(specContent), 0644))

	stdout, _, exitCode := env.runApexWithEnv(
		map[string]string{"CRED_TEST_GOOD": "valid"},
		"credential", "validate")

	assert.NotEqual(t, 0, exitCode,
		"apex credential validate should exit non-zero when a ref fails")
	assert.True(t, strings.Contains(stdout, "OK"),
		"stdout should contain OK, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "FAIL"),
		"stdout should contain FAIL, got: %s", stdout)
}

func TestCredentialListEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("credential", "list")

	assert.Equal(t, 0, exitCode,
		"apex credential list should exit 0 with no creds; stderr=%s", stderr)

	lower := strings.ToLower(stdout)
	assert.True(t, strings.Contains(lower, "no credentials"),
		"stdout should contain 'No credentials' (case-insensitive), got: %s", stdout)
}
```

**Step 2: Build and run E2E tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./e2e/ -run TestCredential -v -count=1`
Expected: PASS (3 tests)

**Step 3: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add e2e/credential_test.go
git commit -m "test(e2e): add E2E tests for credential list, validate, and empty list

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Update PROGRESS.md

**Files:**
- Modify: `PROGRESS.md`

**Step 1: Update completed phases table**

Add row: `| 35 | Credential Injector | \`2026-02-20-phase35-credential-injector-design.md\` | Done |`

**Step 2: Update Current section**

Change "Phase 35" → "Phase 36 — TBD"

**Step 3: Update test counts**

- Unit tests: 40 → 41 packages
- E2E tests: 102 → 105 tests

**Step 4: Add Key Package**

Add: `| \`internal/credinjector\` | Zero-trust credential injection with placeholder refs, vault loading, inject/scrub, and error path protection |`

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add PROGRESS.md
git commit -m "docs: mark Phase 35 Credential Injector as complete

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

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

	// Sort by value length descending - replace longer values first.
	sort.Slice(pairs, func(i, j int) bool {
		return len(pairs[i].value) > len(pairs[j].value)
	})

	for _, p := range pairs {
		text = strings.ReplaceAll(text, p.value, p.placeholder)
	}

	return text
}

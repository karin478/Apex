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

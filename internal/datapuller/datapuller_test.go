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

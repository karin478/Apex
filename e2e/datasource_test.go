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

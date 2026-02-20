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

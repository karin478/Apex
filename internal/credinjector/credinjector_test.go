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
		require.NoError(t, os.WriteFile(p, []byte("[[["), 0644))
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

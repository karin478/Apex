package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSandboxDefaultNone(t *testing.T) {
	env := newTestEnv(t)
	stdout, _, code := env.runApex("run", "list files")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "Sandbox: none")
}

func TestSandboxExplicitNone(t *testing.T) {
	env := newTestEnv(t)
	// Override config to sandbox level: none (already default in test env, but be explicit)
	writeTestConfig(t, env, "none", nil)
	stdout, _, code := env.runApex("run", "say hello")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "Sandbox: none")
	assert.Contains(t, stdout, "Done")
}

func TestSandboxUlimit(t *testing.T) {
	env := newTestEnv(t)
	writeTestConfig(t, env, "ulimit", nil)
	stdout, _, code := env.runApex("run", "say hello")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "Sandbox: ulimit")
	assert.Contains(t, stdout, "Done")
}

func TestSandboxFailClosedRejects(t *testing.T) {
	env := newTestEnv(t)
	// Configure: sandbox=none but require_for=[HIGH]
	writeTestConfig(t, env, "none", []string{"HIGH"})
	// "delete" triggers HIGH risk — should be rejected by fail-closed
	stdout, stderr, code := env.runApex("run", "delete old files")
	combined := stdout + stderr
	assert.NotEqual(t, 0, code)
	assert.Contains(t, strings.ToLower(combined), "fail-closed")
}

func TestSandboxLevelInAudit(t *testing.T) {
	env := newTestEnv(t)
	env.runApex("run", "say hello")

	// Check audit log for sandbox_level field
	auditFiles, err := filepath.Glob(filepath.Join(env.auditDir(), "*.jsonl"))
	require.NoError(t, err)
	require.NotEmpty(t, auditFiles, "no audit files found")
	content := env.readFile(auditFiles[0])
	assert.Contains(t, content, "sandbox_level")
	assert.Contains(t, content, "none")
}

// writeTestConfig writes a config.yaml with the given sandbox level and require_for settings.
func writeTestConfig(t *testing.T, env *TestEnv, sandboxLevel string, requireFor []string) {
	t.Helper()
	requireForYaml := ""
	if len(requireFor) > 0 {
		items := make([]string, len(requireFor))
		for i, r := range requireFor {
			items[i] = fmt.Sprintf("%q", r)
		}
		requireForYaml = fmt.Sprintf("  require_for: [%s]\n", strings.Join(items, ", "))
	}
	config := fmt.Sprintf(`claude:
  model: "mock-model"
  effort: "low"
  timeout: 10
  binary: %q
planner:
  model: "mock-model"
  timeout: 10
pool:
  max_concurrent: 2
retry:
  max_attempts: 3
  init_delay_seconds: 0
  multiplier: 1.0
  max_delay_seconds: 0
sandbox:
  level: %q
%s`, env.MockBin, sandboxLevel, requireForYaml)

	configPath := filepath.Join(env.Home, ".apex", "config.yaml")
	err := os.WriteFile(configPath, []byte(config), 0644)
	require.NoError(t, err)
}

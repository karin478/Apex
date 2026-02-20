package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditPolicyEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, _, exitCode := env.runApex("audit-policy")

	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "No policy changes detected")
}

func TestAuditPolicyAfterRun(t *testing.T) {
	env := newTestEnv(t)

	// Run a task first — this triggers policy check on config.yaml
	_, _, exitCode := env.runApex("run", "say hello")
	require.Equal(t, 0, exitCode, "initial run should succeed")

	// Now audit-policy should show the first-time policy entry
	var stdout string
	stdout, _, exitCode = env.runApex("audit-policy")

	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "Policy Change History")
	assert.Contains(t, stdout, "config.yaml")
}

func TestAuditPolicyDetectsConfigChange(t *testing.T) {
	env := newTestEnv(t)

	// First run to establish baseline checksum
	_, _, exitCode := env.runApex("run", "say hello")
	require.Equal(t, 0, exitCode, "baseline run should succeed")

	// Modify config.yaml so the checksum changes
	configPath := filepath.Join(env.Home, ".apex", "config.yaml")
	existing, err := os.ReadFile(configPath)
	require.NoError(t, err)
	err = os.WriteFile(configPath, append(existing, []byte("\n# modified")...), 0644)
	require.NoError(t, err)

	// Second run triggers change detection
	_, _, exitCode = env.runApex("run", "say hello again")
	require.Equal(t, 0, exitCode, "second run should succeed")

	// Verify audit-policy shows the change
	var stdout string
	stdout, _, exitCode = env.runApex("audit-policy")

	require.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "config.yaml")
}

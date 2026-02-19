package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginListEmpty(t *testing.T) {
	env := newTestEnv(t)
	stdout, _, code := env.runApex("plugin", "list")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "No plugins found")
}

func TestPluginScanAndList(t *testing.T) {
	env := newTestEnv(t)

	// Create a plugin
	pluginsDir := filepath.Join(env.Home, ".apex", "plugins", "test-plugin")
	require.NoError(t, os.MkdirAll(pluginsDir, 0755))
	yaml := `name: test-plugin
version: "1.0.0"
type: reasoning
description: "A test plugin"
author: test
checksum: ""
enabled: true
`
	require.NoError(t, os.WriteFile(filepath.Join(pluginsDir, "plugin.yaml"), []byte(yaml), 0644))

	// Scan
	stdout, _, code := env.runApex("plugin", "scan")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "1 plugin(s)")
	assert.Contains(t, stdout, "test-plugin")

	// List
	stdout, _, code = env.runApex("plugin", "list")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "test-plugin")
	assert.Contains(t, stdout, "enabled")
}

func TestPluginEnableDisable(t *testing.T) {
	env := newTestEnv(t)

	pluginsDir := filepath.Join(env.Home, ".apex", "plugins", "test-plugin")
	os.MkdirAll(pluginsDir, 0755)
	yaml := `name: test-plugin
version: "1.0.0"
type: reasoning
description: "Test"
author: test
checksum: ""
enabled: true
`
	os.WriteFile(filepath.Join(pluginsDir, "plugin.yaml"), []byte(yaml), 0644)

	// Disable
	stdout, _, code := env.runApex("plugin", "disable", "test-plugin")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "disabled")

	// Verify it's disabled in list
	stdout, _, _ = env.runApex("plugin", "list")
	assert.Contains(t, stdout, "disabled")

	// Enable
	stdout, _, code = env.runApex("plugin", "enable", "test-plugin")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "enabled")
}

func TestPluginVerify(t *testing.T) {
	env := newTestEnv(t)

	pluginsDir := filepath.Join(env.Home, ".apex", "plugins", "test-plugin")
	os.MkdirAll(pluginsDir, 0755)
	yaml := `name: test-plugin
version: "1.0.0"
type: reasoning
description: "Test"
author: test
enabled: true
checksum: ""
`
	os.WriteFile(filepath.Join(pluginsDir, "plugin.yaml"), []byte(yaml), 0644)

	stdout, _, code := env.runApex("plugin", "verify", "test-plugin")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "SKIP")
}

func TestPluginEnableNotFound(t *testing.T) {
	env := newTestEnv(t)
	_, stderr, code := env.runApex("plugin", "enable", "nonexistent")
	assert.NotEqual(t, 0, code)
	_ = stderr
}

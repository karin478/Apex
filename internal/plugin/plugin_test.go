package plugin

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPlugin(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "my-plugin")
	os.MkdirAll(pluginDir, 0755)

	yaml := `name: my-plugin
version: "1.0.0"
type: reasoning
description: "A test plugin"
author: test
checksum: ""
enabled: true
reasoning:
  protocol: my-protocol
  steps: 4
  roles: ["a", "b", "c", "d"]
`
	os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(yaml), 0644)

	p, err := LoadPlugin(pluginDir)
	require.NoError(t, err)
	assert.Equal(t, "my-plugin", p.Name)
	assert.Equal(t, "1.0.0", p.Version)
	assert.Equal(t, "reasoning", p.Type)
	assert.Equal(t, "A test plugin", p.Description)
	assert.True(t, p.Enabled)
	assert.Equal(t, pluginDir, p.Dir)
	require.NotNil(t, p.Reasoning)
	assert.Equal(t, "my-protocol", p.Reasoning.Protocol)
	assert.Equal(t, 4, p.Reasoning.Steps)
	assert.Len(t, p.Reasoning.Roles, 4)
}

func TestLoadPluginNotFound(t *testing.T) {
	_, err := LoadPlugin("/nonexistent")
	assert.Error(t, err)
}

func TestLoadPluginInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte("not: [valid: yaml"), 0644)
	_, err := LoadPlugin(dir)
	assert.Error(t, err)
}

func writePluginYAML(t *testing.T, dir, name, version, ptype string, enabled bool) {
	t.Helper()
	pluginDir := filepath.Join(dir, name)
	os.MkdirAll(pluginDir, 0755)
	yaml := fmt.Sprintf(`name: %s
version: "%s"
type: %s
description: "Test plugin %s"
author: test
checksum: ""
enabled: %t
`, name, version, ptype, name, enabled)
	os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(yaml), 0644)
}

func TestRegistryScan(t *testing.T) {
	dir := t.TempDir()
	writePluginYAML(t, dir, "plugin-a", "1.0.0", "reasoning", true)
	writePluginYAML(t, dir, "plugin-b", "0.2.0", "reasoning", false)

	reg := NewRegistry(dir)
	plugins, err := reg.Scan()
	require.NoError(t, err)
	assert.Len(t, plugins, 2)
}

func TestRegistryScanEmptyDir(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	plugins, err := reg.Scan()
	require.NoError(t, err)
	assert.Empty(t, plugins)
}

func TestRegistryScanNonexistentDir(t *testing.T) {
	reg := NewRegistry("/nonexistent/path")
	plugins, err := reg.Scan()
	require.NoError(t, err)
	assert.Empty(t, plugins)
}

func TestRegistryList(t *testing.T) {
	dir := t.TempDir()
	writePluginYAML(t, dir, "plugin-a", "1.0.0", "reasoning", true)

	reg := NewRegistry(dir)
	reg.Scan()
	list := reg.List()
	assert.Len(t, list, 1)
	assert.Equal(t, "plugin-a", list[0].Name)
}

func TestRegistryGet(t *testing.T) {
	dir := t.TempDir()
	writePluginYAML(t, dir, "plugin-a", "1.0.0", "reasoning", true)

	reg := NewRegistry(dir)
	reg.Scan()

	p, ok := reg.Get("plugin-a")
	assert.True(t, ok)
	assert.Equal(t, "plugin-a", p.Name)

	_, ok = reg.Get("nonexistent")
	assert.False(t, ok)
}

func TestRegistryEnableDisable(t *testing.T) {
	dir := t.TempDir()
	writePluginYAML(t, dir, "my-plugin", "1.0.0", "reasoning", true)

	reg := NewRegistry(dir)
	reg.Scan()

	// Disable
	err := reg.Disable("my-plugin")
	require.NoError(t, err)

	p, _ := reg.Get("my-plugin")
	assert.False(t, p.Enabled)

	// Verify file was updated
	reloaded, _ := LoadPlugin(filepath.Join(dir, "my-plugin"))
	assert.False(t, reloaded.Enabled)

	// Enable
	err = reg.Enable("my-plugin")
	require.NoError(t, err)

	p, _ = reg.Get("my-plugin")
	assert.True(t, p.Enabled)
}

func TestRegistryEnableNotFound(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	reg.Scan()
	assert.Error(t, reg.Enable("nonexistent"))
}

func TestRegistryVerifyOK(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "my-plugin")
	os.MkdirAll(pluginDir, 0755)

	// Write plugin.yaml without checksum first
	content := `name: my-plugin
version: "1.0.0"
type: reasoning
description: "Test"
author: test
enabled: true
`
	// Compute checksum of content
	hash := sha256.Sum256([]byte(content))
	checksum := fmt.Sprintf("sha256:%x", hash)

	// Write with checksum
	fullContent := content + fmt.Sprintf("checksum: \"%s\"\n", checksum)
	os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(fullContent), 0644)

	reg := NewRegistry(dir)
	reg.Scan()

	ok, err := reg.Verify("my-plugin")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestRegistryVerifyMismatch(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "my-plugin")
	os.MkdirAll(pluginDir, 0755)

	content := `name: my-plugin
version: "1.0.0"
type: reasoning
description: "Test"
author: test
enabled: true
checksum: "sha256:wronghash"
`
	os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(content), 0644)

	reg := NewRegistry(dir)
	reg.Scan()

	ok, err := reg.Verify("my-plugin")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestRegistryVerifyEmptyChecksum(t *testing.T) {
	dir := t.TempDir()
	writePluginYAML(t, dir, "my-plugin", "1.0.0", "reasoning", true)

	reg := NewRegistry(dir)
	reg.Scan()

	ok, err := reg.Verify("my-plugin")
	require.NoError(t, err)
	assert.False(t, ok, "empty checksum should fail verification")
}

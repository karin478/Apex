package plugin

import (
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

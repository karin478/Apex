package template

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testYAML = `name: test-pipeline
desc: Test pipeline template
vars:
  - name: project
    default: myapp
    desc: Project name
  - name: env
    default: staging
    desc: Target environment
nodes:
  - id: build
    task: "Build {{.project}} for {{.env}}"
  - id: test
    task: "Test {{.project}}"
    depends: [build]
  - id: deploy
    task: "Deploy {{.project}} to {{.env}}"
    depends: [test]
`

func TestLoad(t *testing.T) {
	tmpl, err := Load([]byte(testYAML))
	require.NoError(t, err)

	assert.Equal(t, "test-pipeline", tmpl.Name)
	assert.Equal(t, "Test pipeline template", tmpl.Desc)

	// Vars
	require.Len(t, tmpl.Vars, 2)
	assert.Equal(t, "project", tmpl.Vars[0].Name)
	assert.Equal(t, "myapp", tmpl.Vars[0].Default)
	assert.Equal(t, "Project name", tmpl.Vars[0].Desc)
	assert.Equal(t, "env", tmpl.Vars[1].Name)
	assert.Equal(t, "staging", tmpl.Vars[1].Default)
	assert.Equal(t, "Target environment", tmpl.Vars[1].Desc)

	// Nodes
	require.Len(t, tmpl.Nodes, 3)
	assert.Equal(t, "build", tmpl.Nodes[0].ID)
	assert.Equal(t, "Build {{.project}} for {{.env}}", tmpl.Nodes[0].Task)
	assert.Empty(t, tmpl.Nodes[0].Depends)

	assert.Equal(t, "test", tmpl.Nodes[1].ID)
	assert.Equal(t, "Test {{.project}}", tmpl.Nodes[1].Task)
	assert.Equal(t, []string{"build"}, tmpl.Nodes[1].Depends)

	assert.Equal(t, "deploy", tmpl.Nodes[2].ID)
	assert.Equal(t, "Deploy {{.project}} to {{.env}}", tmpl.Nodes[2].Task)
	assert.Equal(t, []string{"test"}, tmpl.Nodes[2].Depends)
}

func TestLoadInvalid(t *testing.T) {
	invalidYAML := []byte(`{{{invalid yaml`)
	_, err := Load(invalidYAML)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template: failed to parse YAML")
}

func TestExpand(t *testing.T) {
	tmpl, err := Load([]byte(testYAML))
	require.NoError(t, err)

	vars := map[string]string{
		"project": "apex",
		"env":     "production",
	}

	specs, err := tmpl.Expand(vars)
	require.NoError(t, err)
	require.Len(t, specs, 3)

	assert.Equal(t, "build", specs[0].ID)
	assert.Equal(t, "Build apex for production", specs[0].Task)
	assert.Empty(t, specs[0].Depends)

	assert.Equal(t, "test", specs[1].ID)
	assert.Equal(t, "Test apex", specs[1].Task)
	assert.Equal(t, []string{"build"}, specs[1].Depends)

	assert.Equal(t, "deploy", specs[2].ID)
	assert.Equal(t, "Deploy apex to production", specs[2].Task)
	assert.Equal(t, []string{"test"}, specs[2].Depends)
}

func TestExpandDefaults(t *testing.T) {
	tmpl, err := Load([]byte(testYAML))
	require.NoError(t, err)

	// Provide only "project", leave "env" to use default ("staging").
	vars := map[string]string{
		"project": "apex",
	}

	specs, err := tmpl.Expand(vars)
	require.NoError(t, err)
	require.Len(t, specs, 3)

	assert.Equal(t, "Build apex for staging", specs[0].Task)
	assert.Equal(t, "Test apex", specs[1].Task)
	assert.Equal(t, "Deploy apex to staging", specs[2].Task)
}

func TestRegistryRegisterGet(t *testing.T) {
	reg := NewRegistry()

	tmpl, err := Load([]byte(testYAML))
	require.NoError(t, err)

	// Register successfully.
	err = reg.Register(tmpl)
	require.NoError(t, err)

	// Get the registered template.
	got, err := reg.Get("test-pipeline")
	require.NoError(t, err)
	assert.Equal(t, tmpl.Name, got.Name)
	assert.Equal(t, tmpl.Desc, got.Desc)

	// Empty name returns error.
	err = reg.Register(Template{Name: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty name")

	// Unknown template returns ErrTemplateNotFound.
	_, err = reg.Get("nonexistent")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrTemplateNotFound))
}

func TestRegistryList(t *testing.T) {
	reg := NewRegistry()

	// Register multiple templates in non-alphabetical order.
	require.NoError(t, reg.Register(Template{Name: "charlie"}))
	require.NoError(t, reg.Register(Template{Name: "alpha"}))
	require.NoError(t, reg.Register(Template{Name: "bravo"}))

	list := reg.List()
	require.Len(t, list, 3)
	assert.Equal(t, "alpha", list[0].Name)
	assert.Equal(t, "bravo", list[1].Name)
	assert.Equal(t, "charlie", list[2].Name)
}

func TestVarNames(t *testing.T) {
	tmpl, err := Load([]byte(testYAML))
	require.NoError(t, err)

	names := tmpl.VarNames()
	assert.Equal(t, []string{"project", "env"}, names)
}

func TestLoadDir(t *testing.T) {
	dir := t.TempDir()

	// Write a valid YAML file.
	err := os.WriteFile(filepath.Join(dir, "valid.yaml"), []byte(testYAML), 0644)
	require.NoError(t, err)

	// Write an invalid YAML file (should be skipped).
	err = os.WriteFile(filepath.Join(dir, "invalid.yml"), []byte(`{{{bad`), 0644)
	require.NoError(t, err)

	// Write a non-YAML file (should be ignored).
	err = os.WriteFile(filepath.Join(dir, "readme.txt"), []byte(`hello`), 0644)
	require.NoError(t, err)

	templates, err := LoadDir(dir)
	require.NoError(t, err)
	require.Len(t, templates, 1)
	assert.Equal(t, "test-pipeline", templates[0].Name)
}

package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTemplateList(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("template", "list")

	assert.Equal(t, 0, exitCode,
		"apex template list should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "build-test-deploy"),
		"stdout should contain build-test-deploy, got: %s", stdout)
}

func TestTemplateShow(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("template", "show", "build-test-deploy")

	assert.Equal(t, 0, exitCode,
		"apex template show should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "build-test-deploy"),
		"stdout should contain build-test-deploy, got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "project"),
		"stdout should contain 'project', got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "env"),
		"stdout should contain 'env', got: %s", stdout)
}

func TestTemplateExpand(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("template", "expand", "build-test-deploy",
		"--var", "project=apex", "--var", "env=prod")

	assert.Equal(t, 0, exitCode,
		"apex template expand should exit 0; stderr=%s", stderr)
	assert.True(t, strings.Contains(stdout, "Build apex for prod"),
		"stdout should contain 'Build apex for prod', got: %s", stdout)
}

package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnchorCreatedOnRun verifies that "apex run" creates a daily anchor.
func TestAnchorCreatedOnRun(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, code := env.runApex("run", "say hello")
	require.Equal(t, 0, code, "apex run should succeed; stdout=%s stderr=%s", stdout, stderr)

	// Check anchors.jsonl exists
	anchorPath := filepath.Join(env.auditDir(), "anchors.jsonl")
	require.True(t, env.fileExists(anchorPath), "anchors.jsonl should exist after run")

	// Parse and validate
	data, err := os.ReadFile(anchorPath)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 1, "should have exactly 1 anchor")

	var anchor struct {
		Date        string `json:"date"`
		ChainHash   string `json:"chain_hash"`
		RecordCount int    `json:"record_count"`
		GitTag      string `json:"git_tag"`
	}
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &anchor))

	assert.NotEmpty(t, anchor.Date)
	assert.NotEmpty(t, anchor.ChainHash)
	assert.Greater(t, anchor.RecordCount, 0)
	assert.Contains(t, anchor.GitTag, "apex-audit-anchor-")
}

// TestAnchorUpdatedOnSecondRun verifies that a second run updates the anchor.
func TestAnchorUpdatedOnSecondRun(t *testing.T) {
	env := newTestEnv(t)

	// First run
	env.runApex("run", "say hello")

	// Capture baseline record count after first run
	anchorPath := filepath.Join(env.auditDir(), "anchors.jsonl")
	data1, err := os.ReadFile(anchorPath)
	require.NoError(t, err)
	lines1 := strings.Split(strings.TrimSpace(string(data1)), "\n")
	require.Len(t, lines1, 1, "should have 1 anchor after first run")
	var anchor1 struct {
		RecordCount int `json:"record_count"`
	}
	require.NoError(t, json.Unmarshal([]byte(lines1[0]), &anchor1))
	require.Greater(t, anchor1.RecordCount, 0, "first run should produce at least 1 record")

	// Second run
	env.runApex("run", "say goodbye")

	// Should still have 1 anchor (updated, not duplicated)
	data2, err := os.ReadFile(anchorPath)
	require.NoError(t, err)
	lines2 := strings.Split(strings.TrimSpace(string(data2)), "\n")
	assert.Len(t, lines2, 1, "should have 1 anchor, not 2")

	var anchor2 struct {
		RecordCount int `json:"record_count"`
	}
	require.NoError(t, json.Unmarshal([]byte(lines2[0]), &anchor2))
	assert.Equal(t, anchor1.RecordCount+1, anchor2.RecordCount, "second run should add exactly 1 record")
}

// TestAnchorGitTag verifies that a git tag is created in the working directory.
func TestAnchorGitTag(t *testing.T) {
	env := newTestEnv(t)

	env.runApex("run", "say hello")

	// Check git tag exists in WorkDir
	stdout, _, code := env.runApexWithEnv(nil, "doctor")
	require.Equal(t, 0, code)
	assert.Contains(t, stdout, "Git tag anchors")
}

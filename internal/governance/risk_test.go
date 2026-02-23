package governance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyLow(t *testing.T) {
	tests := []string{
		"read the README file",
		"explain this function",
		"search for TODO comments",
		"list all go files",
		"run the tests",
		"write unit tests",       // was MEDIUM before — "write" alone shouldn't trigger
		"add a comment",          // was MEDIUM before — "add" alone shouldn't trigger
		"create a PR",            // was MEDIUM before — "create" alone without file/dir context
		"analyze code quality",
		"compare SQLite DELETE mode with WAL mode", // technical usage, not destructive
	}
	for _, task := range tests {
		assert.Equal(t, LOW, Classify(task), "task: %s", task)
	}
}

func TestClassifyMedium(t *testing.T) {
	tests := []string{
		"write to the config file",
		"modify config settings",
		"install package cobra",
		"update config for staging",
		"create file handler.go",
		"edit file main.go",
		"modify setting for auth",
		"install dep for api",
		"overwrite the output",
	}
	for _, task := range tests {
		assert.Equal(t, MEDIUM, Classify(task), "task: %s", task)
	}
}

func TestClassifyHigh(t *testing.T) {
	tests := []string{
		"delete from users where active = false",
		"deploy the new version",
		"drop table users",
		"migrate the database schema",
		"rm -rf the temp directory",
		"delete database backups",
		"deploy to prod environment",
		"force push to main",
	}
	for _, task := range tests {
		assert.Equal(t, HIGH, Classify(task), "task: %s", task)
	}
}

func TestClassifyCritical(t *testing.T) {
	tests := []string{
		"deploy to production with new encryption keys",
		"rotate the production api key",
	}
	for _, task := range tests {
		assert.Equal(t, CRITICAL, Classify(task), "task: %s", task)
	}
}

func TestClassifyCaseInsensitive(t *testing.T) {
	assert.Equal(t, HIGH, Classify("DELETE FROM users"))
	assert.Equal(t, MEDIUM, Classify("WRITE TO config"))
}

func TestRiskLevelString(t *testing.T) {
	assert.Equal(t, "LOW", LOW.String())
	assert.Equal(t, "MEDIUM", MEDIUM.String())
	assert.Equal(t, "HIGH", HIGH.String())
	assert.Equal(t, "CRITICAL", CRITICAL.String())
}

func TestParseRiskLevel(t *testing.T) {
	assert.Equal(t, LOW, ParseRiskLevel("LOW"))
	assert.Equal(t, MEDIUM, ParseRiskLevel("medium"))
	assert.Equal(t, HIGH, ParseRiskLevel("High"))
	assert.Equal(t, CRITICAL, ParseRiskLevel("CRITICAL"))
	assert.Equal(t, LOW, ParseRiskLevel("unknown"))
}

func TestDefaultPolicy(t *testing.T) {
	// Reset to default
	SetPolicy(DefaultPolicy())
	defer SetPolicy(DefaultPolicy())

	assert.True(t, LOW.ShouldAutoApprove())
	assert.False(t, MEDIUM.ShouldAutoApprove())
	assert.False(t, HIGH.ShouldAutoApprove())
	assert.False(t, CRITICAL.ShouldAutoApprove())

	assert.False(t, LOW.ShouldConfirm())
	assert.True(t, MEDIUM.ShouldConfirm())
	assert.False(t, HIGH.ShouldConfirm())
	assert.False(t, CRITICAL.ShouldConfirm())

	assert.False(t, LOW.ShouldReject())
	assert.False(t, MEDIUM.ShouldReject())
	assert.False(t, HIGH.ShouldReject())
	assert.True(t, CRITICAL.ShouldReject())

	assert.False(t, LOW.ShouldRequireApproval())
	assert.False(t, MEDIUM.ShouldRequireApproval())
	assert.True(t, HIGH.ShouldRequireApproval())
	assert.False(t, CRITICAL.ShouldRequireApproval())
}

func TestCustomPolicy(t *testing.T) {
	SetPolicy(Policy{
		AutoApprove: []string{"LOW", "MEDIUM"},
		Confirm:     []string{"HIGH"},
		Reject:      []string{"CRITICAL"},
	})
	defer SetPolicy(DefaultPolicy())

	assert.True(t, MEDIUM.ShouldAutoApprove())
	assert.True(t, HIGH.ShouldConfirm())
	assert.False(t, HIGH.ShouldRequireApproval())
}

func TestContainsWord(t *testing.T) {
	assert.True(t, containsWord("deploy the app", "deploy"))
	assert.False(t, containsWord("redeployment", "deploy"))
	assert.True(t, containsWord("migrate data", "migrate"))
	assert.False(t, containsWord("immigration policy", "migrate"))
}

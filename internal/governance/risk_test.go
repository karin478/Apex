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
	}
	for _, task := range tests {
		assert.Equal(t, LOW, Classify(task), "task: %s", task)
	}
}

func TestClassifyMedium(t *testing.T) {
	tests := []string{
		"write a new function for parsing",
		"modify the config file",
		"install the cobra package",
		"update the README",
		"create a new test file",
		"修改配置文件",
		"安装依赖",
	}
	for _, task := range tests {
		assert.Equal(t, MEDIUM, Classify(task), "task: %s", task)
	}
}

func TestClassifyHigh(t *testing.T) {
	tests := []string{
		"delete the old migration files",
		"deploy to staging",
		"drop the users table",
		"migrate the database schema",
		"rm -rf the temp directory",
		"删除旧数据",
		"部署到生产",
	}
	for _, task := range tests {
		assert.Equal(t, HIGH, Classify(task), "task: %s", task)
	}
}

func TestClassifyCritical(t *testing.T) {
	tests := []string{
		"deploy to production with new encryption keys",
		"rotate the production API密钥",
	}
	for _, task := range tests {
		assert.Equal(t, CRITICAL, Classify(task), "task: %s", task)
	}
}

func TestClassifyCaseInsensitive(t *testing.T) {
	assert.Equal(t, HIGH, Classify("DELETE all temp files"))
	assert.Equal(t, MEDIUM, Classify("WRITE a new module"))
}

func TestRiskLevelString(t *testing.T) {
	assert.Equal(t, "LOW", LOW.String())
	assert.Equal(t, "MEDIUM", MEDIUM.String())
	assert.Equal(t, "HIGH", HIGH.String())
	assert.Equal(t, "CRITICAL", CRITICAL.String())
}

func TestShouldAutoApprove(t *testing.T) {
	assert.True(t, LOW.ShouldAutoApprove())
	assert.False(t, MEDIUM.ShouldAutoApprove())
	assert.False(t, HIGH.ShouldAutoApprove())
	assert.False(t, CRITICAL.ShouldAutoApprove())
}

func TestShouldConfirm(t *testing.T) {
	assert.False(t, LOW.ShouldConfirm())
	assert.True(t, MEDIUM.ShouldConfirm())
	assert.False(t, HIGH.ShouldConfirm())
	assert.False(t, CRITICAL.ShouldConfirm())
}

func TestShouldReject(t *testing.T) {
	assert.False(t, LOW.ShouldReject())
	assert.False(t, MEDIUM.ShouldReject())
	assert.False(t, HIGH.ShouldReject())
	assert.True(t, CRITICAL.ShouldReject())
}

func TestShouldRequireApproval(t *testing.T) {
	assert.False(t, LOW.ShouldRequireApproval())
	assert.False(t, MEDIUM.ShouldRequireApproval())
	assert.True(t, HIGH.ShouldRequireApproval())
	assert.False(t, CRITICAL.ShouldRequireApproval())
}

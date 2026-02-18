package executor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExecutor(t *testing.T) {
	exec := New(Options{
		Model:   "claude-opus-4-6",
		Effort:  "high",
		Timeout: 600 * time.Second,
	})
	assert.NotNil(t, exec)
}

func TestBuildArgs(t *testing.T) {
	exec := New(Options{
		Model:   "claude-opus-4-6",
		Effort:  "high",
		Timeout: 600 * time.Second,
	})

	args := exec.buildArgs("explain this code")
	assert.Contains(t, args, "-p")
	assert.Contains(t, args, "--model")
	assert.Contains(t, args, "claude-opus-4-6")
	assert.Contains(t, args, "--effort")
	assert.Contains(t, args, "high")
	assert.Contains(t, args, "--output-format")
	assert.Contains(t, args, "json")
}

func TestBuildArgsContainsPrompt(t *testing.T) {
	exec := New(Options{
		Model:   "claude-opus-4-6",
		Effort:  "high",
		Timeout: 600 * time.Second,
	})

	args := exec.buildArgs("do something")
	// Last arg should be the prompt
	assert.Equal(t, "do something", args[len(args)-1])
}

func TestExecuteWithMockBinary(t *testing.T) {
	// Use echo as a mock for claude CLI
	exec := New(Options{
		Model:   "claude-opus-4-6",
		Effort:  "high",
		Timeout: 10 * time.Second,
		Binary:  "echo",
	})

	result, err := exec.Run(context.Background(), "hello world")
	require.NoError(t, err)
	assert.NotEmpty(t, result.Output)
	assert.Equal(t, 0, result.ExitCode)
}

func TestExecuteTimeout(t *testing.T) {
	exec := New(Options{
		Model:   "claude-opus-4-6",
		Effort:  "high",
		Timeout: 100 * time.Millisecond,
		Binary:  "sleep",
	})

	result, err := exec.Run(context.Background(), "10")
	assert.Error(t, err)
	assert.True(t, result.TimedOut)
}

func TestResultDuration(t *testing.T) {
	exec := New(Options{
		Model:   "claude-opus-4-6",
		Effort:  "high",
		Timeout: 10 * time.Second,
		Binary:  "echo",
	})

	result, err := exec.Run(context.Background(), "fast task")
	require.NoError(t, err)
	assert.True(t, result.Duration > 0)
	assert.True(t, result.Duration < 5*time.Second)
}

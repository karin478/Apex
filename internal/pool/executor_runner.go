package pool

import (
	"context"

	"github.com/lyndonlyu/apex/internal/executor"
)

// ClaudeRunner adapts executor.Executor to satisfy the Runner interface,
// allowing the pool to execute tasks via Claude Code CLI.
type ClaudeRunner struct {
	Executor *executor.Executor
}

// NewClaudeRunner creates a new ClaudeRunner wrapping the given executor.
func NewClaudeRunner(exec *executor.Executor) *ClaudeRunner {
	return &ClaudeRunner{Executor: exec}
}

// RunTask executes a task string via Claude Code and returns the output.
func (r *ClaudeRunner) RunTask(ctx context.Context, task string) (string, error) {
	result, err := r.Executor.Run(ctx, task)
	if err != nil {
		return "", err
	}
	return result.Output, nil
}

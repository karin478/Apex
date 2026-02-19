package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/lyndonlyu/apex/internal/sandbox"
)

type Options struct {
	Model   string
	Effort  string
	Timeout time.Duration
	Binary  string          // defaults to "claude"
	Sandbox sandbox.Sandbox // optional sandbox wrapper
}

type Result struct {
	Output   string
	Stderr   string
	ExitCode int
	Duration time.Duration
	TimedOut bool
}

type Executor struct {
	opts Options
}

func New(opts Options) *Executor {
	if opts.Binary == "" {
		opts.Binary = "claude"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 600 * time.Second
	}
	return &Executor{opts: opts}
}

func (e *Executor) buildArgs(task string) []string {
	args := []string{
		"-p",
		"--model", e.opts.Model,
		"--effort", e.opts.Effort,
		"--output-format", "json",
		task,
	}
	return args
}

func (e *Executor) Run(ctx context.Context, task string) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, e.opts.Timeout)
	defer cancel()

	binary := e.opts.Binary
	args := e.buildArgs(task)

	if e.opts.Sandbox != nil {
		var err error
		binary, args, err = e.opts.Sandbox.Wrap(ctx, binary, args)
		if err != nil {
			return Result{}, fmt.Errorf("sandbox wrap: %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, binary, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := Result{
		Output:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if err != nil {
		if ctx.Err() != nil {
			result.TimedOut = true
			return result, ctx.Err()
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		return result, err
	}

	return result, nil
}

package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/lyndonlyu/apex/internal/sandbox"
)

// claudeJSONEnvelope represents the JSON output format of the Claude CLI
// when invoked with --output-format json.
type claudeJSONEnvelope struct {
	Result  string `json:"result"`
	IsError bool   `json:"is_error"`
}

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
		// Even on non-zero exit, try to extract the result text from the
		// Claude JSON envelope so callers get a meaningful error message.
		result.Output = extractResult(result.Output)
		return result, err
	}

	result.Output = extractResult(result.Output)

	return result, nil
}

// extractResult attempts to parse the Claude CLI JSON envelope and return
// the inner result text. If parsing fails (e.g. output from a mock binary
// that doesn't use the JSON envelope), the original string is returned
// unchanged for backward compatibility.
func extractResult(raw string) string {
	var env claudeJSONEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return raw
	}
	if env.Result == "" {
		return raw
	}
	return env.Result
}

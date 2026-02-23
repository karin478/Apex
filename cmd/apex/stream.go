package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lyndonlyu/apex/internal/config"
	"github.com/lyndonlyu/apex/internal/dag"
	"github.com/lyndonlyu/apex/internal/executor"
	"github.com/lyndonlyu/apex/internal/governance"
	"github.com/lyndonlyu/apex/internal/planner"
	"github.com/lyndonlyu/apex/internal/pool"
	"github.com/lyndonlyu/apex/internal/sandbox"
)

// stripJSONEnvelope extracts the result text from a Claude CLI JSON envelope.
// If the line is not a JSON envelope, it is returned as-is.
func stripJSONEnvelope(line string) string {
	var env struct {
		Result  string `json:"result"`
		IsError bool   `json:"is_error"`
	}
	if err := json.Unmarshal([]byte(line), &env); err != nil {
		return line
	}
	if env.Result == "" {
		return line
	}
	return env.Result
}

// runInteractiveTask executes a single task through the full pipeline with
// streaming output. Returns the result summary for session context.
func runInteractiveTask(cfg *config.Config, task string, sessionContext string) (string, error) {
	// Classify risk
	risk := governance.Classify(task)
	fmt.Println(renderRisk(risk.String()) + " " + styleInfo.Render("Planning..."))

	// Confirm if needed
	if risk.ShouldConfirm() {
		fmt.Printf("Warning: %s risk. Proceed? (y/n): ", risk)
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			return "", fmt.Errorf("cancelled by user")
		}
	}
	if risk.ShouldReject() {
		return "", fmt.Errorf("task rejected (%s risk)", risk)
	}

	// Resolve sandbox
	var sb sandbox.Sandbox
	level, _ := sandbox.ParseLevel(cfg.Sandbox.Level)
	sb, _ = sandbox.ForLevel(level)

	// Plan
	planExec := executor.New(executor.Options{
		Model:   cfg.Planner.Model,
		Effort:  "high",
		Timeout: time.Duration(cfg.Planner.Timeout) * time.Second,
		Binary:  cfg.Claude.Binary,
		Sandbox: sb,
	})

	// Prepend session context to task for continuity
	enrichedTask := task
	if sessionContext != "" {
		enrichedTask = "Context from previous tasks:\n" + sessionContext + "\n\nNew task: " + task
	}

	nodes, err := planner.Plan(context.Background(), planExec, enrichedTask, cfg.Planner.Model, cfg.Planner.Timeout)
	if err != nil {
		return "", fmt.Errorf("planning failed: %w", err)
	}

	d, err := dag.New(nodes)
	if err != nil {
		return "", fmt.Errorf("invalid DAG: %w", err)
	}

	fmt.Printf("%s %d steps\n", styleInfo.Render("Plan:"), len(d.Nodes))

	// Execute with streaming
	exec := executor.New(executor.Options{
		Model:          cfg.Claude.Model,
		Effort:         cfg.Claude.Effort,
		Timeout:        time.Duration(cfg.Claude.Timeout) * time.Second,
		Binary:         cfg.Claude.Binary,
		Sandbox:        sb,
		PermissionMode: cfg.Claude.PermissionMode,
		OnOutput: func(chunk string) {
			text := stripJSONEnvelope(chunk)
			for _, line := range strings.Split(text, "\n") {
				if line != "" {
					fmt.Println(styleDim.Render("  " + line))
				}
			}
		},
	})

	runner := pool.NewClaudeRunner(exec)
	p := pool.New(cfg.Pool.MaxConcurrent, runner)

	start := time.Now()
	execErr := p.Execute(context.Background(), d)
	duration := time.Since(start)

	if execErr != nil {
		fmt.Println(styleError.Render("✗ Failed") + styleDim.Render(fmt.Sprintf(" (%.1fs)", duration.Seconds())))
		return d.Summary(), execErr
	}

	fmt.Println(styleSuccess.Render("✓ Done") + styleDim.Render(fmt.Sprintf(" (%.1fs, %d steps)", duration.Seconds(), len(d.Nodes))))
	return d.Summary(), nil
}

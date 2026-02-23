package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
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

// renderMarkdown renders markdown text for terminal display.
func renderMarkdown(text string) string {
	r, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(100))
	if err != nil {
		return text
	}
	out, err := r.Render(text)
	if err != nil {
		return text
	}
	return strings.TrimSpace(out)
}

// runInteractiveTask executes a single task. Simple tasks skip the planner/DAG
// pipeline and call the executor directly for faster response.
func runInteractiveTask(cfg *config.Config, task string, sessionContext string) (string, error) {
	risk := governance.Classify(task)

	if risk.ShouldConfirm() {
		fmt.Printf("%s Warning: %s risk. Proceed? (y/n): ", renderRisk(risk.String()), risk)
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			return "", fmt.Errorf("cancelled by user")
		}
	}
	if risk.ShouldReject() {
		return "", fmt.Errorf("task rejected (%s risk)", risk)
	}

	var sb sandbox.Sandbox
	level, _ := sandbox.ParseLevel(cfg.Sandbox.Level)
	sb, _ = sandbox.ForLevel(level)

	enrichedTask := task
	if sessionContext != "" {
		enrichedTask = "Context from previous tasks:\n" + sessionContext + "\n\nNew task: " + task
	}

	// Fast path: simple tasks skip planner+DAG, call executor directly
	if planner.IsSimpleTask(task) {
		return runSimpleTask(cfg, enrichedTask, sb)
	}

	// Complex path: planner → DAG → pool
	return runComplexTask(cfg, enrichedTask, sb)
}

// runSimpleTask calls the executor directly — no planner, no DAG, no pool.
func runSimpleTask(cfg *config.Config, task string, sb sandbox.Sandbox) (string, error) {
	exec := executor.New(executor.Options{
		Model:          cfg.Claude.Model,
		Effort:         "low",
		Timeout:        time.Duration(cfg.Claude.Timeout) * time.Second,
		Binary:         cfg.Claude.Binary,
		Sandbox:        sb,
		PermissionMode: "bypassPermissions",
	})

	start := time.Now()
	result, err := exec.Run(context.Background(), task)
	duration := time.Since(start)

	if err != nil {
		fmt.Println(styleError.Render("✗ Failed") + styleDim.Render(fmt.Sprintf(" (%.1fs)", duration.Seconds())))
		return result.Output, err
	}

	fmt.Println(renderMarkdown(result.Output))
	fmt.Println(styleDim.Render(fmt.Sprintf("(%.1fs)", duration.Seconds())))
	return result.Output, nil
}

// runComplexTask decomposes via planner, builds a DAG, and executes through the pool.
func runComplexTask(cfg *config.Config, task string, sb sandbox.Sandbox) (string, error) {
	fmt.Println(styleInfo.Render("Planning..."))

	planExec := executor.New(executor.Options{
		Model:          cfg.Planner.Model,
		Effort:         "low",
		Timeout:        time.Duration(cfg.Planner.Timeout) * time.Second,
		Binary:         cfg.Claude.Binary,
		Sandbox:        sb,
		PermissionMode: "plan",
	})

	nodes, err := planner.Plan(context.Background(), planExec, task, cfg.Planner.Model, cfg.Planner.Timeout)
	if err != nil {
		return "", fmt.Errorf("planning failed: %w", err)
	}

	d, err := dag.New(nodes)
	if err != nil {
		return "", fmt.Errorf("invalid DAG: %w", err)
	}

	fmt.Printf("%s %d steps\n", styleInfo.Render("Plan:"), len(d.Nodes))

	exec := executor.New(executor.Options{
		Model:          cfg.Claude.Model,
		Effort:         cfg.Claude.Effort,
		Timeout:        time.Duration(cfg.Claude.Timeout) * time.Second,
		Binary:         cfg.Claude.Binary,
		Sandbox:        sb,
		PermissionMode: "bypassPermissions",
		OnOutput: func(chunk string) {
			text := stripJSONEnvelope(chunk)
			fmt.Println(renderMarkdown(text))
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

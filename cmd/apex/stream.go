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
		fmt.Printf("  %s Warning: %s risk. Proceed? (y/n): ", renderRisk(risk.String()), risk)
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
		return runSimpleTask(cfg, enrichedTask, sb, risk.String())
	}

	// Complex path: planner → DAG → pool
	return runComplexTask(cfg, enrichedTask, sb, risk.String())
}

// runSimpleTask calls the executor directly — no planner, no DAG, no pool.
func runSimpleTask(cfg *config.Config, task string, sb sandbox.Sandbox, riskLevel string) (string, error) {
	spin := NewSpinnerWithDetail("Thinking...", cfg.Claude.Model)
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
	spin.Stop()

	if err != nil {
		fmt.Println(errorHeader())
		fmt.Println(styleError.Render(fmt.Sprintf("  %s", err.Error())))
		fmt.Println()
		fmt.Println(separator())
		fmt.Println(styleMeta.Render(fmt.Sprintf("  ✗ %.1fs · %s · %s", duration.Seconds(), cfg.Claude.Model, renderRisk(riskLevel))))
		return result.Output, err
	}

	fmt.Println(responseHeader())
	fmt.Println()
	fmt.Println(renderMarkdown(result.Output))
	fmt.Println()
	fmt.Println(separator())
	fmt.Println(styleMeta.Render(fmt.Sprintf("  ✓ %.1fs · %s · %s", duration.Seconds(), cfg.Claude.Model, renderRisk(riskLevel))))
	return result.Output, nil
}

// runComplexTask decomposes via planner, builds a DAG, and executes through the pool.
func runComplexTask(cfg *config.Config, task string, sb sandbox.Sandbox, riskLevel string) (string, error) {
	spin := NewSpinnerWithDetail("Planning...", cfg.Planner.Model)

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
		spin.Stop()
		return "", fmt.Errorf("planning failed: %w", err)
	}

	d, err := dag.New(nodes)
	if err != nil {
		spin.Stop()
		return "", fmt.Errorf("invalid DAG: %w", err)
	}

	spin.Stop()
	fmt.Printf("  %s %d steps\n\n", styleInfo.Render("Planning..."), len(d.Nodes))

	exec := executor.New(executor.Options{
		Model:          cfg.Claude.Model,
		Effort:         cfg.Claude.Effort,
		Timeout:        time.Duration(cfg.Claude.Timeout) * time.Second,
		Binary:         cfg.Claude.Binary,
		Sandbox:        sb,
		PermissionMode: "bypassPermissions",
	})

	runner := pool.NewClaudeRunner(exec)
	p := pool.New(cfg.Pool.MaxConcurrent, runner)

	start := time.Now()

	// Run pool in a goroutine and poll DAG for step progress
	doneCh := make(chan error, 1)
	go func() {
		doneCh <- p.Execute(context.Background(), d)
	}()

	// Poll and display step progress
	execErr := displayStepProgress(d, doneCh)

	duration := time.Since(start)

	// Count completed/failed
	dagNodes := d.NodeSlice()
	completed := 0
	for _, n := range dagNodes {
		if n.Status == dag.Completed {
			completed++
		}
	}
	total := len(dagNodes)

	fmt.Println()
	fmt.Println(separator())
	if execErr != nil {
		fmt.Println(styleMeta.Render(fmt.Sprintf("  %s %d/%d steps · %.1fs · %s · %s",
			styleError.Render("✗"), completed, total, duration.Seconds(), cfg.Claude.Model, renderRisk(riskLevel))))
		return d.Summary(), execErr
	}

	fmt.Println(styleMeta.Render(fmt.Sprintf("  %s %d/%d steps · %.1fs · %s · %s",
		styleSuccess.Render("✓"), completed, total, duration.Seconds(), cfg.Claude.Model, renderRisk(riskLevel))))
	return d.Summary(), nil
}

// displayStepProgress polls the DAG and displays per-step progress with box-drawing characters.
// Returns the execution error from the pool.
func displayStepProgress(d *dag.DAG, doneCh chan error) error {
	displayed := make(map[string]bool)
	finalized := make(map[string]bool)
	stepIndex := make(map[string]int)
	nodes := d.NodeSlice()
	for i, n := range nodes {
		stepIndex[n.ID] = i + 1
	}
	total := len(nodes)

	// Active spinners for running steps
	spinners := make(map[string]*Spinner)

	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case execErr := <-doneCh:
			// Final sweep: close any remaining spinners and display final state
			for id, spin := range spinners {
				spin.Stop()
				delete(spinners, id)
			}
			finalNodes := d.NodeSlice()
			for _, n := range finalNodes {
				if !finalized[n.ID] {
					renderStepFinal(n, stepIndex[n.ID], total)
					finalized[n.ID] = true
				}
			}
			return execErr
		case <-tick.C:
			for _, n := range d.NodeSlice() {
				if finalized[n.ID] {
					continue
				}
				switch n.Status {
				case dag.Running:
					if !displayed[n.ID] {
						fmt.Printf("  %s [%d/%d] %s\n",
							styleStepBorder.Render("┌"),
							stepIndex[n.ID], total,
							styleStepName.Render(n.Task))
						spin := NewSpinner(fmt.Sprintf("Running step %d/%d...", stepIndex[n.ID], total))
						spinners[n.ID] = spin
						displayed[n.ID] = true
					}
				case dag.Completed:
					if spin, ok := spinners[n.ID]; ok {
						spin.Stop()
						delete(spinners, n.ID)
					}
					if !finalized[n.ID] {
						fmt.Printf("  %s %s\n\n",
							styleStepBorder.Render("└"),
							styleSuccess.Render("✓ Done"))
						finalized[n.ID] = true
					}
				case dag.Failed:
					if spin, ok := spinners[n.ID]; ok {
						spin.Stop()
						delete(spinners, n.ID)
					}
					if !finalized[n.ID] {
						fmt.Printf("  %s %s\n\n",
							styleStepBorder.Render("└"),
							styleError.Render("✗ Failed"))
						finalized[n.ID] = true
					}
				}
			}
		}
	}
}

// renderStepFinal renders a completed/failed step that wasn't displayed during polling.
func renderStepFinal(n *dag.Node, idx, total int) {
	fmt.Printf("  %s [%d/%d] %s\n",
		styleStepBorder.Render("┌"),
		idx, total,
		styleStepName.Render(n.Task))
	switch n.Status {
	case dag.Completed:
		fmt.Printf("  %s %s\n\n",
			styleStepBorder.Render("└"),
			styleSuccess.Render("✓ Done"))
	case dag.Failed:
		fmt.Printf("  %s %s\n\n",
			styleStepBorder.Render("└"),
			styleError.Render("✗ Failed"))
	default:
		fmt.Printf("  %s %s\n\n",
			styleStepBorder.Render("└"),
			styleDim.Render("– Skipped"))
	}
}

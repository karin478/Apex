package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/lyndonlyu/apex/internal/audit"
	"github.com/lyndonlyu/apex/internal/config"
	apexctx "github.com/lyndonlyu/apex/internal/context"
	"github.com/lyndonlyu/apex/internal/dag"
	"github.com/lyndonlyu/apex/internal/executor"
	"github.com/lyndonlyu/apex/internal/governance"
	"github.com/lyndonlyu/apex/internal/killswitch"
	"github.com/lyndonlyu/apex/internal/manifest"
	"github.com/lyndonlyu/apex/internal/memory"
	"github.com/lyndonlyu/apex/internal/planner"
	"github.com/lyndonlyu/apex/internal/pool"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [task]",
	Short: "Execute a task via Claude Code",
	Long:  "Classify risk, decompose into DAG, then execute concurrently via Claude Code CLI.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runTask,
}

func runTask(cmd *cobra.Command, args []string) error {
	task := args[0]

	// Load config
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".apex", "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	// Ensure directories
	if err := cfg.EnsureDirs(); err != nil {
		return fmt.Errorf("failed to create dirs: %w", err)
	}

	// Kill switch pre-flight check
	ks := killswitch.New(killSwitchPath())
	if ks.IsActive() {
		return fmt.Errorf("kill switch is active at %s — use 'apex resume' to deactivate", ks.Path())
	}

	// Classify risk
	risk := governance.Classify(task)
	fmt.Printf("[%s] Risk level: %s\n", task, risk)

	// Gate by risk level
	if risk.ShouldReject() {
		fmt.Printf("Task rejected (%s risk). Break it into smaller, safer steps.\n", risk)
		return nil
	}

	if risk.ShouldConfirm() {
		fmt.Printf("Warning: %s risk detected. Proceed? (y/n): ", risk)
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Plan: decompose task into DAG
	planExec := executor.New(executor.Options{
		Model:   cfg.Planner.Model,
		Effort:  "high",
		Timeout: time.Duration(cfg.Planner.Timeout) * time.Second,
	})

	fmt.Println("Planning task...")
	nodes, err := planner.Plan(context.Background(), planExec, task, cfg.Planner.Model, cfg.Planner.Timeout)
	if err != nil {
		return fmt.Errorf("planning failed: %w", err)
	}

	d, err := dag.New(nodes)
	if err != nil {
		return fmt.Errorf("invalid DAG: %w", err)
	}

	fmt.Printf("Plan: %d steps\n", len(d.Nodes))

	// Build enriched prompts for each DAG node (keep original Task for display/audit)
	ctxBuilder := apexctx.NewBuilder(apexctx.Options{
		TokenBudget: cfg.Context.TokenBudget,
	})

	enrichedTasks := make(map[string]string)
	for _, node := range d.Nodes {
		enriched, buildErr := ctxBuilder.Build(context.Background(), node.Task)
		if buildErr == nil {
			enrichedTasks[node.ID] = enriched
		}
	}

	// Swap in enriched prompts for execution, then restore originals
	origTasks := make(map[string]string)
	for id, enriched := range enrichedTasks {
		origTasks[id] = d.Nodes[id].Task
		d.Nodes[id].Task = enriched
	}

	// Execute DAG
	exec := executor.New(executor.Options{
		Model:   cfg.Claude.Model,
		Effort:  cfg.Claude.Effort,
		Timeout: time.Duration(cfg.Claude.Timeout) * time.Second,
	})

	runner := pool.NewClaudeRunner(exec)
	p := pool.New(cfg.Pool.MaxConcurrent, runner)

	killCtx, killCancel := ks.Watch(context.Background())
	defer killCancel()

	fmt.Println("Executing...")
	start := time.Now()
	execErr := p.Execute(killCtx, d)
	duration := time.Since(start)

	// Detect kill switch interruption
	killedBySwitch := ks.IsActive() && killCtx.Err() != nil

	// Restore original task names for display/audit
	for id, orig := range origTasks {
		d.Nodes[id].Task = orig
	}

	if killedBySwitch {
		fmt.Println("\n[KILL SWITCH] Execution halted by kill switch.")
	}

	// Audit
	auditDir := filepath.Join(cfg.BaseDir, "audit")
	logger, auditInitErr := audit.NewLogger(auditDir)
	if auditInitErr != nil {
		fmt.Fprintf(os.Stderr, "warning: audit init failed: %v\n", auditInitErr)
	}

	// Log each node
	if logger != nil {
		for _, n := range d.Nodes {
			nodeOutcome := "success"
			nodeErr := ""
			if n.Status == dag.Failed {
				nodeOutcome = "failure"
				nodeErr = n.Error
			}
			logger.Log(audit.Entry{
				Task:      fmt.Sprintf("[%s] %s", n.ID, n.Task),
				RiskLevel: risk.String(),
				Outcome:   nodeOutcome,
				Duration:  duration,
				Model:     cfg.Claude.Model,
				Error:     nodeErr,
			})
		}
	}

	// Save run manifest
	runsDir := filepath.Join(cfg.BaseDir, "runs")
	manifestStore := manifest.NewStore(runsDir)

	outcome := "success"
	if killedBySwitch {
		outcome = "killed"
	} else if d.HasFailure() {
		if execErr != nil {
			outcome = "failure"
		} else {
			outcome = "partial_failure"
		}
	}

	var nodeResults []manifest.NodeResult
	for _, n := range d.Nodes {
		nr := manifest.NodeResult{
			ID:     n.ID,
			Task:   n.Task,
			Status: n.Status.String(),
		}
		if n.Status == dag.Failed {
			nr.Error = n.Error
		}
		nodeResults = append(nodeResults, nr)
	}

	runManifest := &manifest.Manifest{
		RunID:      uuid.New().String(),
		Task:       task,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Model:      cfg.Claude.Model,
		Effort:     cfg.Claude.Effort,
		RiskLevel:  risk.String(),
		NodeCount:  len(d.Nodes),
		DurationMs: duration.Milliseconds(),
		Outcome:    outcome,
		Nodes:      nodeResults,
	}

	if saveErr := manifestStore.Save(runManifest); saveErr != nil {
		fmt.Fprintf(os.Stderr, "warning: manifest save failed: %v\n", saveErr)
	}

	// Print results
	fmt.Println("\n--- Results ---")
	fmt.Println(d.Summary())

	if execErr != nil {
		return fmt.Errorf("execution error: %w", execErr)
	}

	// Save to memory
	memDir := filepath.Join(cfg.BaseDir, "memory")
	store, memErr := memory.NewStore(memDir)
	if memErr != nil {
		fmt.Fprintf(os.Stderr, "warning: memory init failed: %v\n", memErr)
	} else {
		store.SaveSession("run", task, d.Summary())
	}

	fmt.Printf("\nDone (%.1fs, %s risk, %d steps)\n", duration.Seconds(), risk, len(d.Nodes))

	if d.HasFailure() {
		return fmt.Errorf("some steps failed, check audit log for details")
	}
	return nil
}

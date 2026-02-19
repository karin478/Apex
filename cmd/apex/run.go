package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/lyndonlyu/apex/internal/approval"
	"github.com/lyndonlyu/apex/internal/audit"
	"github.com/lyndonlyu/apex/internal/config"
	"github.com/lyndonlyu/apex/internal/cost"
	apexctx "github.com/lyndonlyu/apex/internal/context"
	"github.com/lyndonlyu/apex/internal/dag"
	"github.com/lyndonlyu/apex/internal/executor"
	"github.com/lyndonlyu/apex/internal/governance"
	"github.com/lyndonlyu/apex/internal/killswitch"
	"github.com/lyndonlyu/apex/internal/manifest"
	"github.com/lyndonlyu/apex/internal/memory"
	"github.com/lyndonlyu/apex/internal/planner"
	"github.com/lyndonlyu/apex/internal/pool"
	"github.com/lyndonlyu/apex/internal/snapshot"
	"github.com/spf13/cobra"
)

var dryRun bool

func init() {
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show execution plan and cost estimate without executing tasks (planning step still runs)")
}

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

	// Gate by risk level (CRITICAL always rejected, even in dry-run)
	if risk.ShouldReject() {
		fmt.Printf("Task rejected (%s risk). Break it into smaller, safer steps.\n", risk)
		return nil
	}

	if !dryRun && risk.ShouldConfirm() {
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

	// Approval gate for HIGH risk tasks (skipped in dry-run)
	if !dryRun && risk.ShouldRequireApproval() {
		reviewer := approval.NewReviewer(os.Stdin, os.Stdout)
		result, reviewErr := reviewer.Review(d.NodeSlice(), governance.Classify)
		if reviewErr != nil {
			return fmt.Errorf("approval review failed: %w", reviewErr)
		}
		if !result.Approved {
			fmt.Println("Approval rejected. Aborting.")
			return nil
		}
		// Remove skipped nodes
		for _, nd := range result.Nodes {
			if nd.Decision == approval.Skipped {
				d.RemoveNode(nd.NodeID)
			}
		}
		if len(d.Nodes) == 0 {
			fmt.Println("All nodes skipped. Nothing to execute.")
			return nil
		}
		fmt.Printf("Approved: %d steps to execute\n", len(d.Nodes))
	}

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

	// Dry-run: print report and exit
	if dryRun {
		fmt.Printf("\n[DRY RUN] %s\n", task)
		fmt.Printf("Risk: %s", risk)
		if risk.ShouldRequireApproval() {
			fmt.Print(" (approval required for execution)")
		}
		fmt.Println()

		fmt.Printf("\nPlan: %d steps\n", len(d.Nodes))
		for i, n := range d.NodeSlice() {
			nodeRisk := governance.Classify(n.Task)
			fmt.Fprintf(os.Stdout, "  [%d] %-40s %s\n", i+1, n.Task, nodeRisk)
		}

		fmt.Println("\nContext:")
		totalTokens := 0
		for i, n := range d.NodeSlice() {
			if enriched, ok := enrichedTasks[n.ID]; ok {
				tokens := cost.EstimateTokens(enriched)
				totalTokens += tokens
				fmt.Fprintf(os.Stdout, "  [%d] %d tokens\n", i+1, tokens)
			}
		}
		fmt.Fprintf(os.Stdout, "  Budget: %d/%d (%d%%)\n", totalTokens, cfg.Context.TokenBudget,
			totalTokens*100/max(cfg.Context.TokenBudget, 1))

		est := cost.EstimateRun(enrichedTasks, cfg.Claude.Model)
		fmt.Printf("\nCost estimate: %s (%d calls, %s)\n", cost.FormatCost(est.TotalCost), est.NodeCount, est.Model)

		fmt.Println("\nNo changes made. Run without --dry-run to execute.")
		return nil
	}

	// Swap in enriched prompts for execution, then restore originals
	origTasks := make(map[string]string)
	for id, enriched := range enrichedTasks {
		origTasks[id] = d.Nodes[id].Task
		d.Nodes[id].Task = enriched
	}

	runID := uuid.New().String()

	// Execute DAG
	exec := executor.New(executor.Options{
		Model:   cfg.Claude.Model,
		Effort:  cfg.Claude.Effort,
		Timeout: time.Duration(cfg.Claude.Timeout) * time.Second,
	})

	runner := pool.NewClaudeRunner(exec)
	p := pool.New(cfg.Pool.MaxConcurrent, runner)

	// Second kill switch check right before execution
	if ks.IsActive() {
		return fmt.Errorf("kill switch activated during planning — use 'apex resume' to deactivate")
	}

	// Create snapshot after all pre-checks pass (so early returns don't stash away user edits)
	cwd, _ := os.Getwd()
	snapMgr := snapshot.New(cwd)
	snap, snapErr := snapMgr.Create(runID)
	if snapErr != nil {
		fmt.Fprintf(os.Stderr, "warning: snapshot creation failed: %v\n", snapErr)
	} else if snap != nil {
		// Restore working tree — snapshot is a transparent backup, not destructive
		if applyErr := snapMgr.Apply(runID); applyErr != nil {
			fmt.Fprintf(os.Stderr, "warning: snapshot apply failed: %v\n", applyErr)
		}
		fmt.Printf("Snapshot saved (%s)\n", snap.Message)
	}

	killCtx, killCancel := ks.Watch(context.Background())
	defer killCancel()

	fmt.Println("Executing...")
	start := time.Now()
	execErr := p.Execute(killCtx, d)
	duration := time.Since(start)

	// Detect kill switch interruption (reliable: doesn't depend on file still existing)
	killedBySwitch := ks.WasTriggered()

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
			} else if killedBySwitch && (n.Status == dag.Pending || n.Status == dag.Running) {
				nodeOutcome = "interrupted"
				nodeErr = "kill switch activated"
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

	// Handle snapshot based on outcome
	if snap != nil {
		if outcome == "success" {
			if dropErr := snapMgr.Drop(runID); dropErr != nil {
				fmt.Fprintf(os.Stderr, "warning: snapshot cleanup failed: %v\n", dropErr)
			}
		} else {
			fmt.Printf("\nSnapshot available. Restore with: apex snapshot restore %s\n", runID)
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
		RunID:      runID,
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

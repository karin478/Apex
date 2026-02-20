package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	"github.com/lyndonlyu/apex/internal/health"
	"github.com/lyndonlyu/apex/internal/killswitch"
	"github.com/lyndonlyu/apex/internal/manifest"
	"github.com/lyndonlyu/apex/internal/memory"
	"github.com/lyndonlyu/apex/internal/planner"
	"github.com/lyndonlyu/apex/internal/pool"
	"github.com/lyndonlyu/apex/internal/redact"
	"github.com/lyndonlyu/apex/internal/retry"
	"github.com/lyndonlyu/apex/internal/sandbox"
	"github.com/lyndonlyu/apex/internal/snapshot"
	"github.com/lyndonlyu/apex/internal/trace"
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

	// Health pre-flight gate
	report := health.Evaluate(cfg.BaseDir)
	if report.Level == health.CRITICAL {
		fmt.Println("[HEALTH] System health CRITICAL — run 'apex doctor' to diagnose")
		return nil
	}

	// Create root trace context
	tc := trace.NewTrace()
	fmt.Printf("[trace: %s]\n", tc.TraceID[:8])

	// Classify risk
	risk := governance.Classify(task)
	fmt.Printf("[%s] Risk level: %s\n", task, risk)

	if report.Level == health.RED && !risk.ShouldAutoApprove() {
		fmt.Printf("[HEALTH] System health RED — only LOW-risk tasks allowed (task risk: %s)\n", risk)
		return nil
	}

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

	// Resolve sandbox
	var sb sandbox.Sandbox
	if cfg.Sandbox.Level == "auto" {
		sb = sandbox.Detect()
	} else {
		level, parseErr := sandbox.ParseLevel(cfg.Sandbox.Level)
		if parseErr != nil {
			return fmt.Errorf("invalid sandbox level: %w", parseErr)
		}
		var levelErr error
		sb, levelErr = sandbox.ForLevel(level)
		if levelErr != nil {
			return fmt.Errorf("sandbox unavailable: %w", levelErr)
		}
	}

	// Configure sandbox from config
	switch s := sb.(type) {
	case *sandbox.DockerSandbox:
		s.Image = cfg.Sandbox.DockerImage
		s.MemoryLimit = cfg.Sandbox.MemoryLimit
		s.CPULimit = cfg.Sandbox.CPULimit
	case *sandbox.UlimitSandbox:
		s.MaxCPUSec = cfg.Sandbox.MaxCPUSeconds
		s.MaxFileSizeMB = cfg.Sandbox.MaxFileSizeMB
	}

	// Fail-Closed: check if risk level requires higher sandbox
	for _, req := range cfg.Sandbox.RequireFor {
		if strings.EqualFold(req, risk.String()) {
			var requiredLevel sandbox.Level
			if strings.EqualFold(req, "CRITICAL") || strings.EqualFold(req, "HIGH") {
				requiredLevel = sandbox.Docker
			} else {
				requiredLevel = sandbox.Ulimit
			}
			if sb.Level() < requiredLevel {
				return fmt.Errorf("fail-closed: task risk %s requires %s isolation but only %s available",
					risk, requiredLevel, sb.Level())
			}
			break
		}
	}

	fmt.Printf("Sandbox: %s\n", sb.Level())

	// Plan: decompose task into DAG
	planExec := executor.New(executor.Options{
		Model:   cfg.Planner.Model,
		Effort:  "high",
		Timeout: time.Duration(cfg.Planner.Timeout) * time.Second,
		Binary:  cfg.Claude.Binary,
		Sandbox: sb,
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
		Binary:  cfg.Claude.Binary,
		Sandbox: sb,
	})

	runner := pool.NewClaudeRunner(exec)
	p := pool.New(cfg.Pool.MaxConcurrent, runner)
	retryPolicy := retry.Policy{
		MaxAttempts: cfg.Retry.MaxAttempts,
		InitDelay:   time.Duration(cfg.Retry.InitDelaySeconds) * time.Second,
		Multiplier:  cfg.Retry.Multiplier,
		MaxDelay:    time.Duration(cfg.Retry.MaxDelaySeconds) * time.Second,
	}
	p.RetryPolicy = &retryPolicy

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
	if logger != nil {
		logger.SetRedactor(redact.New(cfg.Redaction))
	}

	// Pre-generate action IDs for each node so we can build causal links
	nodeActionIDs := make(map[string]string)
	for id := range d.Nodes {
		nodeActionIDs[id] = uuid.New().String()
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
			// Determine parent: first DAG dependency's action ID, else empty (root)
			parentActionID := ""
			if len(n.Depends) > 0 {
				parentActionID = nodeActionIDs[n.Depends[0]]
			}
			logger.Log(audit.Entry{
				Task:           fmt.Sprintf("[%s] %s", n.ID, n.Task),
				RiskLevel:      risk.String(),
				Outcome:        nodeOutcome,
				Duration:       duration,
				Model:          cfg.Claude.Model,
				Error:          nodeErr,
				SandboxLevel:   sb.Level().String(),
				TraceID:        tc.TraceID,
				ParentActionID: parentActionID,
				ActionID:       nodeActionIDs[n.ID],
			})
		}
	}

	// Daily anchor — create/update after audit entries are written
	if logger != nil {
		cwd, _ := os.Getwd()
		if created, anchorErr := audit.MaybeCreateAnchor(logger, cwd); anchorErr != nil {
			fmt.Fprintf(os.Stderr, "warning: anchor creation failed: %v\n", anchorErr)
		} else if created {
			fmt.Println("Daily audit anchor updated.")
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
			ID:       n.ID,
			Task:     n.Task,
			Status:   n.Status.String(),
			ActionID: nodeActionIDs[n.ID],
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
		TraceID:    tc.TraceID,
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

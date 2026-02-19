package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lyndonlyu/apex/internal/audit"
	"github.com/lyndonlyu/apex/internal/config"
	"github.com/lyndonlyu/apex/internal/executor"
	"github.com/lyndonlyu/apex/internal/reasoning"
	"github.com/spf13/cobra"
)

var reviewCmd = &cobra.Command{
	Use:   "review [proposal]",
	Short: "Run adversarial review on a technical proposal",
	Long:  "Subject a technical proposal to a structured Advocate/Critic/Judge debate.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runReview,
}

// executorRunner adapts executor.Executor to reasoning.Runner.
type executorRunner struct {
	exec *executor.Executor
}

func (r *executorRunner) RunTask(ctx context.Context, task string) (string, error) {
	result, err := r.exec.Run(ctx, task)
	if err != nil {
		return "", err
	}
	return result.Output, nil
}

func runReview(cmd *cobra.Command, args []string) error {
	proposal := args[0]

	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".apex", "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	exec := executor.New(executor.Options{
		Model:   cfg.Claude.Model,
		Effort:  cfg.Claude.Effort,
		Timeout: time.Duration(cfg.Claude.Timeout) * time.Second,
		Binary:  cfg.Claude.Binary,
	})

	runner := &executorRunner{exec: exec}

	fmt.Println("Adversarial Review")
	fmt.Println("==================")
	fmt.Println()

	roles := []string{"Advocate", "Critic", "Response", "Judge"}
	dots := []string{"...", ".....", "...", "......"}

	ctx := context.Background()
	start := time.Now()

	result, err := reasoning.RunReviewWithProgress(ctx, runner, proposal, func(step int, dur time.Duration) {
		fmt.Printf("%-11s%s done (%.1fs)\n", roles[step], dots[step], dur.Seconds())
	})
	if err != nil {
		return fmt.Errorf("review failed: %w", err)
	}

	totalDur := time.Since(start)

	fmt.Println()
	fmt.Printf("Verdict: %s\n", formatDecision(result.Verdict.Decision))
	fmt.Printf("Summary: %s\n", result.Verdict.Summary)

	if len(result.Verdict.Risks) > 0 {
		fmt.Println()
		fmt.Println("Risks:")
		for i, r := range result.Verdict.Risks {
			fmt.Printf("  %d. %s\n", i+1, r)
		}
	}

	if len(result.Verdict.Actions) > 0 {
		fmt.Println()
		fmt.Println("Suggested Actions:")
		for i, a := range result.Verdict.Actions {
			fmt.Printf("  %d. %s\n", i+1, a)
		}
	}

	// Save review
	reviewsDir := filepath.Join(home, ".apex", "reviews")
	if err := reasoning.SaveReview(reviewsDir, result); err != nil {
		fmt.Fprintf(os.Stderr, "warning: save review failed: %v\n", err)
	} else {
		fmt.Printf("\nFull report: %s/%s.json\n", reviewsDir, result.ID)
	}

	// Audit log
	auditDir := filepath.Join(home, ".apex", "audit")
	logger, logErr := audit.NewLogger(auditDir)
	if logErr == nil {
		truncated := proposal
		if len(truncated) > 80 {
			truncated = truncated[:80]
		}
		// Map verdict decision to audit outcome (history.go expects "rejected")
		outcome := result.Verdict.Decision
		if outcome == "reject" {
			outcome = "rejected"
		}
		logger.Log(audit.Entry{
			Task:      "review: " + truncated,
			RiskLevel: "LOW",
			Outcome:   outcome,
			Duration:  totalDur,
			Model:     cfg.Claude.Model,
		})

		// Refresh daily anchor after audit entry
		cwd, _ := os.Getwd()
		audit.MaybeCreateAnchor(logger, cwd)
	}

	return nil
}

func formatDecision(d string) string {
	switch d {
	case "approve":
		return "APPROVE"
	case "reject":
		return "REJECT"
	case "revise":
		return "REVISE"
	default:
		return d
	}
}

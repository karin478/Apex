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
	"github.com/lyndonlyu/apex/internal/governance"
	"github.com/lyndonlyu/apex/internal/memory"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [task]",
	Short: "Execute a task via Claude Code",
	Long:  "Classify risk, get approval if needed, then execute the task via Claude Code CLI.",
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

	// Execute
	exec := executor.New(executor.Options{
		Model:   cfg.Claude.Model,
		Effort:  cfg.Claude.Effort,
		Timeout: time.Duration(cfg.Claude.Timeout) * time.Second,
	})

	fmt.Println("Executing task...")
	start := time.Now()
	result, err := exec.Run(context.Background(), task)
	duration := time.Since(start)

	// Audit
	auditDir := filepath.Join(cfg.BaseDir, "audit")
	logger, auditErr := audit.NewLogger(auditDir)
	if auditErr != nil {
		fmt.Fprintf(os.Stderr, "warning: audit init failed: %v\n", auditErr)
	}

	outcome := "success"
	errMsg := ""
	if err != nil {
		outcome = "failure"
		errMsg = err.Error()
		if result.TimedOut {
			outcome = "timeout"
		}
	}

	if logger != nil {
		logger.Log(audit.Entry{
			Task:      task,
			RiskLevel: risk.String(),
			Outcome:   outcome,
			Duration:  duration,
			Model:     cfg.Claude.Model,
			Error:     errMsg,
		})
	}

	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	// Output result
	fmt.Println("\n--- Result ---")
	fmt.Println(result.Output)

	// Save to memory
	memDir := filepath.Join(cfg.BaseDir, "memory")
	store, memErr := memory.NewStore(memDir)
	if memErr != nil {
		fmt.Fprintf(os.Stderr, "warning: memory init failed: %v\n", memErr)
	} else {
		store.SaveSession("run", task, result.Output)
	}

	fmt.Printf("\nDone (%.1fs, %s risk)\n", duration.Seconds(), risk)
	return nil
}

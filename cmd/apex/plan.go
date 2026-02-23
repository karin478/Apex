package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/lyndonlyu/apex/internal/config"
	"github.com/lyndonlyu/apex/internal/dag"
	"github.com/lyndonlyu/apex/internal/executor"
	"github.com/lyndonlyu/apex/internal/planner"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan [task]",
	Short: "Preview task decomposition without executing",
	Long:  "Decompose a task into a DAG and display the execution plan without running it.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  planTask,
}

func planTask(cmd *cobra.Command, args []string) error {
	task := args[0]

	// Load config
	home, err := homeDir()
	if err != nil {
		return err
	}
	configPath := filepath.Join(home, ".apex", "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	// Create executor for planning
	exec := executor.New(executor.Options{
		Model:   cfg.Planner.Model,
		Effort:  "high",
		Timeout: time.Duration(cfg.Planner.Timeout) * time.Second,
		Binary:  cfg.Claude.Binary,
	})

	fmt.Println("Analyzing task...")
	nodes, err := planner.Plan(context.Background(), exec, task, cfg.Planner.Model, cfg.Planner.Timeout)
	if err != nil {
		return fmt.Errorf("planning failed: %w", err)
	}

	// Validate the DAG structure
	d, err := dag.New(nodes)
	if err != nil {
		return fmt.Errorf("invalid DAG: %w", err)
	}

	fmt.Printf("\nExecution Plan (%d steps):\n\n", len(d.Nodes))
	for _, n := range nodes {
		deps := "none"
		if len(n.Depends) > 0 {
			deps = fmt.Sprintf("%v", n.Depends)
		}
		fmt.Printf("  [%s] %s\n", n.ID, n.Task)
		fmt.Printf("        depends: %s\n\n", deps)
	}

	return nil
}

package main

import (
	"fmt"

	"github.com/lyndonlyu/apex/internal/progress"
	"github.com/spf13/cobra"
)

var progressFormat string

// progressTracker is a package-level tracker for CLI demo purposes.
var progressTracker = progress.NewTracker()

var progressCmd = &cobra.Command{
	Use:   "progress",
	Short: "Task progress tracking",
}

var progressListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tracked tasks",
	RunE:  runProgressList,
}

var progressShowCmd = &cobra.Command{
	Use:   "show <task-id>",
	Short: "Show progress for a specific task",
	Args:  cobra.ExactArgs(1),
	RunE:  runProgressShow,
}

var progressStartCmd = &cobra.Command{
	Use:   "start <task-id>",
	Short: "Start tracking a task",
	Args:  cobra.ExactArgs(1),
	RunE:  runProgressStart,
}

var progressPhase string

func init() {
	progressListCmd.Flags().StringVar(&progressFormat, "format", "", "Output format (json)")
	progressStartCmd.Flags().StringVar(&progressPhase, "phase", "default", "Phase name")
	progressCmd.AddCommand(progressListCmd, progressShowCmd, progressStartCmd)
}

func runProgressList(cmd *cobra.Command, args []string) error {
	list := progressTracker.List()

	if progressFormat == "json" {
		out, err := progress.FormatProgressListJSON(list)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(progress.FormatProgressList(list))
	}
	return nil
}

func runProgressShow(cmd *cobra.Command, args []string) error {
	report, err := progressTracker.Get(args[0])
	if err != nil {
		return err
	}
	fmt.Print(progress.FormatProgressReport(report))
	return nil
}

func runProgressStart(cmd *cobra.Command, args []string) error {
	report := progressTracker.Start(args[0], progressPhase)
	fmt.Print(progress.FormatProgressReport(*report))
	return nil
}

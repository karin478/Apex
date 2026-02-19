package main

import (
	"fmt"
	"os"

	"github.com/lyndonlyu/apex/internal/snapshot"
	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Manage execution snapshots",
}

var snapshotListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all apex snapshots",
	RunE:  listSnapshots,
}

var snapshotRestoreCmd = &cobra.Command{
	Use:   "restore [run-id]",
	Short: "Restore working tree from a snapshot",
	Args:  cobra.ExactArgs(1),
	RunE:  restoreSnapshot,
}

func init() {
	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotCmd.AddCommand(snapshotRestoreCmd)
}

func listSnapshots(cmd *cobra.Command, args []string) error {
	cwd, _ := os.Getwd()
	m := snapshot.New(cwd)

	snaps, err := m.List()
	if err != nil {
		return fmt.Errorf("failed to list snapshots: %w", err)
	}

	if len(snaps) == 0 {
		fmt.Println("No snapshots found.")
		return nil
	}

	fmt.Printf("%-12s %-50s\n", "RUN_ID", "MESSAGE")
	fmt.Println("------------ --------------------------------------------------")
	for _, s := range snaps {
		runID := s.RunID
		if len(runID) > 12 {
			runID = runID[:12]
		}
		fmt.Printf("%-12s %-50s\n", runID, s.Message)
	}
	return nil
}

func restoreSnapshot(cmd *cobra.Command, args []string) error {
	runID := args[0]
	cwd, _ := os.Getwd()
	m := snapshot.New(cwd)

	fmt.Printf("Restoring snapshot for run %s...\n", runID)
	if err := m.Restore(runID); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}
	fmt.Println("Snapshot restored successfully.")
	return nil
}

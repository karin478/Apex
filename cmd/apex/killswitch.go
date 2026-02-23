package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lyndonlyu/apex/internal/killswitch"
	"github.com/spf13/cobra"
)

var killSwitchCmd = &cobra.Command{
	Use:   "kill-switch [reason]",
	Short: "Activate emergency kill switch to halt all execution",
	RunE:  activateKillSwitch,
}

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Deactivate kill switch and allow execution to continue",
	RunE:  deactivateKillSwitch,
}

func killSwitchPath() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "KILL_SWITCH"), nil
}

func activateKillSwitch(cmd *cobra.Command, args []string) error {
	ksPath, err := killSwitchPath()
	if err != nil {
		return err
	}
	w := killswitch.New(ksPath)

	if w.IsActive() {
		fmt.Printf("Kill switch already active at %s\n", w.Path())
		return nil
	}

	reason := "manual activation"
	if len(args) > 0 {
		reason = strings.Join(args, " ")
	}

	if err := w.Activate(reason); err != nil {
		return fmt.Errorf("failed to activate kill switch: %w", err)
	}

	fmt.Printf("Kill switch ACTIVATED at %s\n", w.Path())
	fmt.Printf("Reason: %s\n", reason)
	fmt.Println("All running executions will be stopped.")
	fmt.Println("Use 'apex resume' to deactivate.")
	return nil
}

func deactivateKillSwitch(cmd *cobra.Command, args []string) error {
	ksPath, err := killSwitchPath()
	if err != nil {
		return err
	}
	w := killswitch.New(ksPath)

	if !w.IsActive() {
		fmt.Println("No kill switch active.")
		return nil
	}

	if err := w.Clear(); err != nil {
		return fmt.Errorf("failed to deactivate kill switch: %w", err)
	}

	fmt.Println("Kill switch DEACTIVATED. Execution may resume.")
	return nil
}

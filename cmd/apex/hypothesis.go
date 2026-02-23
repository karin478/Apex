package main

import (
	"fmt"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/hypothesis"
	"github.com/lyndonlyu/apex/internal/manifest"
	"github.com/spf13/cobra"
)

var hypothesisCmd = &cobra.Command{
	Use:   "hypothesis",
	Short: "Manage hypothesis board",
	Long:  "Track and manage hypotheses for debugging and analysis with propose/challenge/confirm/reject lifecycle.",
}

var hypothesisListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all hypotheses",
	RunE:  listHypotheses,
}

var hypothesisProposeCmd = &cobra.Command{
	Use:   "propose [statement]",
	Short: "Propose a new hypothesis",
	Args:  cobra.ExactArgs(1),
	RunE:  proposeHypothesis,
}

var hypothesisConfirmCmd = &cobra.Command{
	Use:   "confirm [id] [evidence]",
	Short: "Confirm a hypothesis with evidence",
	Args:  cobra.ExactArgs(2),
	RunE:  confirmHypothesis,
}

var hypothesisRejectCmd = &cobra.Command{
	Use:   "reject [id] [reason]",
	Short: "Reject a hypothesis",
	Args:  cobra.ExactArgs(2),
	RunE:  rejectHypothesis,
}

func init() {
	hypothesisCmd.AddCommand(hypothesisListCmd)
	hypothesisCmd.AddCommand(hypothesisProposeCmd)
	hypothesisCmd.AddCommand(hypothesisConfirmCmd)
	hypothesisCmd.AddCommand(hypothesisRejectCmd)
}

func boardPath() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	baseDir := filepath.Join(home, ".apex")

	// Use most recent run's ID as session, or "default"
	store := manifest.NewStore(filepath.Join(baseDir, "runs"))
	recent, recentErr := store.Recent(1)
	sessionID := "default"
	if recentErr == nil && len(recent) > 0 {
		sessionID = recent[0].RunID
	}

	return filepath.Join(baseDir, "sessions", sessionID, "hypothesis_board.json"), nil
}

func loadOrCreateBoard() (*hypothesis.Board, string, error) {
	path, err := boardPath()
	if err != nil {
		return nil, "", err
	}
	b, loadErr := hypothesis.LoadBoard(path)
	if loadErr != nil {
		// Extract session ID from path
		sessionID := filepath.Base(filepath.Dir(path))
		b = hypothesis.NewBoard(sessionID)
	}
	return b, path, nil
}

func listHypotheses(cmd *cobra.Command, args []string) error {
	b, _, err := loadOrCreateBoard()
	if err != nil {
		return err
	}
	hypotheses := b.List()

	if len(hypotheses) == 0 {
		fmt.Println("No hypotheses on the board.")
		return nil
	}

	for _, h := range hypotheses {
		score := hypothesis.Score(&h)
		fmt.Printf("[%s] %-12s  %.1f  %s\n", h.ID, h.Status, score, h.Statement)
	}
	return nil
}

func proposeHypothesis(cmd *cobra.Command, args []string) error {
	b, path, err := loadOrCreateBoard()
	if err != nil {
		return err
	}
	h := b.Propose(args[0])
	if err := b.Save(path); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}
	fmt.Printf("Proposed: [%s] %s\n", h.ID, h.Statement)
	return nil
}

func confirmHypothesis(cmd *cobra.Command, args []string) error {
	b, path, err := loadOrCreateBoard()
	if err != nil {
		return err
	}
	ev := hypothesis.Evidence{Type: "user_confirmation", Content: args[1], Confidence: 0.9}
	if err := b.Confirm(args[0], ev); err != nil {
		return err
	}
	if err := b.Save(path); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}
	fmt.Printf("Confirmed: [%s]\n", args[0])
	return nil
}

func rejectHypothesis(cmd *cobra.Command, args []string) error {
	b, path, err := loadOrCreateBoard()
	if err != nil {
		return err
	}
	if err := b.Reject(args[0], args[1]); err != nil {
		return err
	}
	if err := b.Save(path); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}
	fmt.Printf("Rejected: [%s]\n", args[0])
	return nil
}

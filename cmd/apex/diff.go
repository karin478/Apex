package main

import (
	"fmt"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/manifest"
	"github.com/spf13/cobra"
)

var diffFormat string

var diffCmd = &cobra.Command{
	Use:   "diff <run-id-1> <run-id-2>",
	Short: "Compare two run manifests",
	Long:  "Load two run manifests by ID and display their differences side-by-side.",
	Args:  cobra.ExactArgs(2),
	RunE:  runDiff,
}

func init() {
	diffCmd.Flags().StringVar(&diffFormat, "format", "human", "Output format: human or json")
}

func runDiff(cmd *cobra.Command, args []string) error {
	home, err := homeDir()
	if err != nil {
		return err
	}
	runsDir := filepath.Join(home, ".apex", "runs")
	store := manifest.NewStore(runsDir)

	left, err := store.Load(args[0])
	if err != nil {
		return fmt.Errorf("failed to load left manifest %s: %w", args[0], err)
	}
	right, err := store.Load(args[1])
	if err != nil {
		return fmt.Errorf("failed to load right manifest %s: %w", args[1], err)
	}

	d := manifest.Diff(left, right)

	switch diffFormat {
	case "json":
		out, fmtErr := manifest.FormatDiffJSON(d)
		if fmtErr != nil {
			return fmt.Errorf("format error: %w", fmtErr)
		}
		fmt.Print(out)
	default:
		fmt.Print(manifest.FormatDiffHuman(d))
	}
	return nil
}

package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/lyndonlyu/apex/internal/artifact"
	"github.com/lyndonlyu/apex/internal/manifest"
	"github.com/spf13/cobra"
)

var artifactRunFilter string
var artifactGCDryRun bool
var artifactImpactFormat string

var artifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Manage content-addressed artifacts",
}

var artifactListCmd = &cobra.Command{
	Use:   "list",
	Short: "List stored artifacts",
	RunE:  listArtifacts,
}

var artifactInfoCmd = &cobra.Command{
	Use:   "info <hash>",
	Short: "Show artifact details",
	Args:  cobra.ExactArgs(1),
	RunE:  infoArtifact,
}

var artifactGCCmd = &cobra.Command{
	Use:   "gc",
	Short: "Remove orphan artifacts",
	RunE:  gcArtifacts,
}

var artifactImpactCmd = &cobra.Command{
	Use:   "impact <hash>",
	Short: "Show downstream impact of an artifact",
	Args:  cobra.ExactArgs(1),
	RunE:  impactArtifact,
}

var artifactDepsCmd = &cobra.Command{
	Use:   "deps <hash>",
	Short: "Show direct dependencies of an artifact",
	Args:  cobra.ExactArgs(1),
	RunE:  depsArtifact,
}

func init() {
	artifactListCmd.Flags().StringVar(&artifactRunFilter, "run", "", "Filter by run ID")
	artifactGCCmd.Flags().BoolVar(&artifactGCDryRun, "dry-run", false, "Preview without deleting")
	artifactImpactCmd.Flags().StringVar(&artifactImpactFormat, "format", "", "Output format (json)")
	artifactCmd.AddCommand(artifactListCmd, artifactInfoCmd, artifactGCCmd, artifactImpactCmd, artifactDepsCmd)
}

func listArtifacts(cmd *cobra.Command, args []string) error {
	home, err := homeDir()
	if err != nil {
		return err
	}
	store := artifact.NewStore(filepath.Join(home, ".apex", "artifacts"))

	var arts []*artifact.Artifact
	var listErr error

	if artifactRunFilter != "" {
		arts, listErr = store.ListByRun(artifactRunFilter)
	} else {
		arts, listErr = store.List()
	}
	if listErr != nil {
		return fmt.Errorf("artifact list: %w", listErr)
	}

	if len(arts) == 0 {
		fmt.Println("No artifacts stored.")
		return nil
	}

	fmt.Printf("%-12s  %-20s  %-12s  %s\n", "HASH", "NAME", "RUN", "SIZE")
	for _, a := range arts {
		short := a.Hash
		if len(short) > 12 {
			short = short[:12]
		}
		runShort := a.RunID
		if len(runShort) > 12 {
			runShort = runShort[:12]
		}
		fmt.Printf("%-12s  %-20s  %-12s  %s\n", short, a.Name, runShort, humanSize(a.Size))
	}
	return nil
}

func infoArtifact(cmd *cobra.Command, args []string) error {
	home, err := homeDir()
	if err != nil {
		return err
	}
	store := artifact.NewStore(filepath.Join(home, ".apex", "artifacts"))

	a, err := store.Get(args[0])
	if err != nil {
		return fmt.Errorf("artifact info: %w", err)
	}

	fmt.Printf("Hash:      %s\n", a.Hash)
	fmt.Printf("Name:      %s\n", a.Name)
	fmt.Printf("RunID:     %s\n", a.RunID)
	fmt.Printf("NodeID:    %s\n", a.NodeID)
	fmt.Printf("Size:      %s\n", humanSize(a.Size))
	fmt.Printf("CreatedAt: %s\n", a.CreatedAt)
	return nil
}

func gcArtifacts(cmd *cobra.Command, args []string) error {
	home, err := homeDir()
	if err != nil {
		return err
	}
	artStore := artifact.NewStore(filepath.Join(home, ".apex", "artifacts"))
	manStore := manifest.NewStore(filepath.Join(home, ".apex", "runs"))

	// Collect valid run IDs from manifest store.
	manifests, err := manStore.Recent(math.MaxInt32)
	if err != nil {
		return fmt.Errorf("artifact gc: load manifests: %w", err)
	}
	validIDs := make(map[string]bool, len(manifests))
	for _, m := range manifests {
		validIDs[m.RunID] = true
	}

	orphans, err := artStore.FindOrphans(validIDs)
	if err != nil {
		return fmt.Errorf("artifact gc: find orphans: %w", err)
	}

	if len(orphans) == 0 {
		fmt.Println("No orphan artifacts found.")
		return nil
	}

	if artifactGCDryRun {
		fmt.Printf("[dry-run] Would remove %d orphan artifact(s):\n", len(orphans))
		for _, o := range orphans {
			fmt.Printf("  %s  %s\n", o.Hash[:12], o.Name)
		}
		return nil
	}

	removed := 0
	for _, o := range orphans {
		if err := artStore.Remove(o.Hash); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to remove %s: %v\n", o.Hash[:12], err)
			continue
		}
		removed++
	}
	fmt.Printf("Removed %d orphan artifact(s).\n", removed)
	return nil
}

func impactArtifact(cmd *cobra.Command, args []string) error {
	home, err := homeDir()
	if err != nil {
		return err
	}
	artDir := filepath.Join(home, ".apex", "artifacts")

	lg, err := artifact.NewLineageGraph(artDir)
	if err != nil {
		return fmt.Errorf("artifact impact: %w", err)
	}

	result := lg.Impact(args[0])

	if artifactImpactFormat == "json" {
		fmt.Println(artifact.FormatImpactJSON(result))
	} else {
		fmt.Println(artifact.FormatImpact(result))
	}
	return nil
}

func depsArtifact(cmd *cobra.Command, args []string) error {
	home, err := homeDir()
	if err != nil {
		return err
	}
	artDir := filepath.Join(home, ".apex", "artifacts")

	lg, err := artifact.NewLineageGraph(artDir)
	if err != nil {
		return fmt.Errorf("artifact deps: %w", err)
	}

	deps := lg.DirectDeps(args[0])
	if len(deps) == 0 {
		fmt.Printf("No dependencies for %s\n", args[0])
		return nil
	}
	for _, d := range deps {
		fmt.Println(d)
	}
	return nil
}

func humanSize(b int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	if b == 0 {
		return "0 B"
	}
	i := int(math.Log(float64(b)) / math.Log(1024))
	if i >= len(units) {
		i = len(units) - 1
	}
	val := float64(b) / math.Pow(1024, float64(i))
	s := fmt.Sprintf("%.1f %s", val, units[i])
	if strings.HasSuffix(s, ".0 "+units[i]) {
		s = fmt.Sprintf("%d %s", int(val), units[i])
	}
	return s
}

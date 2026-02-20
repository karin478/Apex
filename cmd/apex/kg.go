package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/kg"
	"github.com/spf13/cobra"
)

var (
	kgListType    string
	kgListProject string
	kgQueryDepth  int
	kgQueryFormat string
)

var kgCmd = &cobra.Command{
	Use:   "kg",
	Short: "Knowledge graph operations",
}

var kgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List entities in the knowledge graph",
	RunE:  runKGList,
}

var kgQueryCmd = &cobra.Command{
	Use:   "query <name>",
	Short: "Query entities by name and show related nodes",
	Args:  cobra.ExactArgs(1),
	RunE:  runKGQuery,
}

var kgStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show knowledge graph statistics",
	RunE:  runKGStats,
}

func init() {
	kgListCmd.Flags().StringVar(&kgListType, "type", "", "Filter by entity type")
	kgListCmd.Flags().StringVar(&kgListProject, "project", "", "Filter by project")
	kgQueryCmd.Flags().IntVar(&kgQueryDepth, "depth", 2, "BFS traversal depth")
	kgQueryCmd.Flags().StringVar(&kgQueryFormat, "format", "", "Output format (json)")
	kgCmd.AddCommand(kgListCmd, kgQueryCmd, kgStatsCmd)
}

func openGraph() (*kg.Graph, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("kg: home dir: %w", err)
	}
	dir := filepath.Join(home, ".claude", "kg")
	g, err := kg.New(dir)
	if err != nil {
		return nil, fmt.Errorf("kg: open graph: %w", err)
	}
	return g, nil
}

func runKGList(cmd *cobra.Command, args []string) error {
	g, err := openGraph()
	if err != nil {
		return err
	}

	entities := g.List(kg.EntityType(kgListType), kgListProject)
	fmt.Print(kg.FormatEntitiesTable(entities))
	return nil
}

func runKGQuery(cmd *cobra.Command, args []string) error {
	g, err := openGraph()
	if err != nil {
		return err
	}

	name := args[0]
	matches := g.QueryByName(name)
	if len(matches) == 0 {
		fmt.Printf("No entities matching %q found.\n", name)
		return nil
	}

	for _, center := range matches {
		related, rels := g.QueryRelated(center.ID, kgQueryDepth, 0)

		if kgQueryFormat == "json" {
			result := struct {
				Center   *kg.Entity         `json:"center"`
				Related  []*kg.Entity       `json:"related"`
				Rels     []*kg.Relationship `json:"relationships"`
			}{
				Center:  center,
				Related: related,
				Rels:    rels,
			}
			fmt.Println(kg.FormatJSON(result))
		} else {
			fmt.Print(kg.FormatQueryResult(center, related, rels))
			fmt.Println()
		}
	}
	return nil
}

func runKGStats(cmd *cobra.Command, args []string) error {
	g, err := openGraph()
	if err != nil {
		return err
	}

	stats := g.Stats()
	fmt.Printf("Entities:      %d\n", stats.TotalEntities)
	fmt.Printf("Relationships: %d\n", stats.TotalRelationships)

	if len(stats.EntitiesByType) > 0 {
		fmt.Println("\nEntities by type:")
		for t, count := range stats.EntitiesByType {
			fmt.Printf("  %-16s %d\n", t, count)
		}
	}

	if len(stats.RelsByType) > 0 {
		fmt.Println("\nRelationships by type:")
		for t, count := range stats.RelsByType {
			fmt.Printf("  %-16s %d\n", t, count)
		}
	}

	return nil
}

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/memory"
	"github.com/spf13/cobra"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage memory",
}

var memorySearchCmd = &cobra.Command{
	Use:   "search [keyword]",
	Short: "Search memory for a keyword",
	Args:  cobra.ExactArgs(1),
	RunE:  searchMemory,
}

func init() {
	memoryCmd.AddCommand(memorySearchCmd)
}

func searchMemory(cmd *cobra.Command, args []string) error {
	keyword := args[0]
	home, _ := os.UserHomeDir()
	memDir := filepath.Join(home, ".apex", "memory")

	store, err := memory.NewStore(memDir)
	if err != nil {
		return fmt.Errorf("memory error: %w", err)
	}

	results, err := store.Search(keyword)
	if err != nil {
		return fmt.Errorf("search error: %w", err)
	}

	if len(results) == 0 {
		fmt.Printf("No memories found for '%s'\n", keyword)
		return nil
	}

	fmt.Printf("Found %d result(s) for '%s':\n\n", len(results), keyword)
	for _, r := range results {
		fmt.Printf("  [%s] %s\n", r.Type, r.Path)
		if r.Snippet != "" {
			fmt.Printf("         %s\n", r.Snippet)
		}
		fmt.Println()
	}
	return nil
}

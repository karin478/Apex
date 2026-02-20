package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/memport"
	"github.com/spf13/cobra"
)

var (
	memExportCategory string
	memExportOutput   string
	memImportStrategy string
)

var memoryExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export memory files to JSON",
	RunE:  runMemoryExport,
}

var memoryImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import memory files from a JSON export",
	Args:  cobra.ExactArgs(1),
	RunE:  runMemoryImport,
}

func init() {
	memoryExportCmd.Flags().StringVar(&memExportCategory, "category", "", "Filter by category (decisions|facts|sessions)")
	memoryExportCmd.Flags().StringVar(&memExportOutput, "output", "", "Write JSON to file instead of stdout")
	memoryImportCmd.Flags().StringVar(&memImportStrategy, "strategy", "skip", "Merge strategy for existing files (skip|overwrite)")
	memoryCmd.AddCommand(memoryExportCmd)
	memoryCmd.AddCommand(memoryImportCmd)
}

func memoryDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("memory: home dir: %w", err)
	}
	return filepath.Join(home, ".claude", "memory"), nil
}

func runMemoryExport(cmd *cobra.Command, args []string) error {
	memDir, err := memoryDir()
	if err != nil {
		return err
	}

	data, err := memport.Export(memDir, memExportCategory)
	if err != nil {
		return fmt.Errorf("memory export: %w", err)
	}

	if memExportOutput != "" {
		if err := memport.WriteFile(memExportOutput, data); err != nil {
			return fmt.Errorf("memory export: write file: %w", err)
		}
		fmt.Printf("Exported %d entries to %s\n", data.Count, memExportOutput)
		return nil
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("memory export: marshal: %w", err)
	}
	fmt.Println(string(b))
	return nil
}

func runMemoryImport(cmd *cobra.Command, args []string) error {
	memDir, err := memoryDir()
	if err != nil {
		return err
	}

	strategy := memport.MergeStrategy(memImportStrategy)
	if strategy != memport.MergeSkip && strategy != memport.MergeOverwrite {
		return fmt.Errorf("memory import: invalid strategy %q (use skip or overwrite)", memImportStrategy)
	}

	data, err := memport.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("memory import: read file: %w", err)
	}

	result, err := memport.Import(memDir, data, strategy)
	if err != nil {
		return fmt.Errorf("memory import: %w", err)
	}

	fmt.Printf("Import complete: Added=%d  Skipped=%d  Overwritten=%d\n",
		result.Added, result.Skipped, result.Overwritten)
	return nil
}

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lyndonlyu/apex/internal/config"
	"github.com/lyndonlyu/apex/internal/embedding"
	"github.com/lyndonlyu/apex/internal/memory"
	"github.com/lyndonlyu/apex/internal/search"
	"github.com/lyndonlyu/apex/internal/vectordb"
	"github.com/spf13/cobra"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage memory",
}

var memorySearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search memory using hybrid search (vector + keyword)",
	Args:  cobra.ExactArgs(1),
	RunE:  searchMemory,
}

var memoryIndexCmd = &cobra.Command{
	Use:   "index",
	Short: "Build vector index for existing memory files",
	RunE:  indexMemory,
}

func init() {
	memoryCmd.AddCommand(memorySearchCmd)
	memoryCmd.AddCommand(memoryIndexCmd)
}

func loadSearchDeps() (*config.Config, *memory.Store, *vectordb.VectorDB, *embedding.Client, error) {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".apex", "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("config error: %w", err)
	}

	if err := cfg.EnsureDirs(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create dirs: %w", err)
	}

	memDir := filepath.Join(cfg.BaseDir, "memory")
	store, err := memory.NewStore(memDir)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("memory error: %w", err)
	}

	vdbPath := filepath.Join(cfg.BaseDir, "vectors.db")
	vdb, vdbErr := vectordb.Open(vdbPath, cfg.Embedding.Dimensions)
	if vdbErr != nil {
		fmt.Fprintf(os.Stderr, "warning: vector DB unavailable: %v\n", vdbErr)
	}

	apiKey := os.Getenv(cfg.Embedding.APIKeyEnv)
	embedder := embedding.NewClient(apiKey, cfg.Embedding.Model, cfg.Embedding.Dimensions)
	if !embedder.Available() {
		fmt.Fprintf(os.Stderr, "warning: embedding unavailable (set %s for vector search)\n", cfg.Embedding.APIKeyEnv)
	}

	return cfg, store, vdb, embedder, nil
}

func searchMemory(cmd *cobra.Command, args []string) error {
	query := args[0]

	_, store, vdb, embedder, err := loadSearchDeps()
	if err != nil {
		return err
	}
	if vdb != nil {
		defer vdb.Close()
	}

	engine := search.New(vdb, store, embedder)
	results, err := engine.Hybrid(context.Background(), query, 20)
	if err != nil {
		return fmt.Errorf("search error: %w", err)
	}

	if len(results) == 0 {
		fmt.Printf("No memories found for '%s'\n", query)
		return nil
	}

	fmt.Printf("Found %d result(s) for '%s':\n\n", len(results), query)
	for _, r := range results {
		source := r.Source
		if r.Type != "" {
			source = r.Type + "/" + source
		}
		fmt.Printf("  [%s] %.2f  %s\n", source, r.Score, r.ID)
		if r.Text != "" {
			text := r.Text
			if len(text) > 120 {
				text = text[:120] + "..."
			}
			fmt.Printf("         %s\n", text)
		}
		fmt.Println()
	}
	return nil
}

func indexMemory(cmd *cobra.Command, args []string) error {
	cfg, _, vdb, embedder, err := loadSearchDeps()
	if err != nil {
		return err
	}
	if vdb != nil {
		defer vdb.Close()
	}

	if vdb == nil {
		return fmt.Errorf("vector DB unavailable, cannot index")
	}
	if !embedder.Available() {
		return fmt.Errorf("embedding client unavailable (set %s)", cfg.Embedding.APIKeyEnv)
	}

	memDir := filepath.Join(cfg.BaseDir, "memory")
	var files []string
	filepath.Walk(memDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".jsonl") {
			files = append(files, path)
		}
		return nil
	})

	fmt.Printf("Indexing %d memory files...\n", len(files))

	indexed := 0
	for _, path := range files {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "warning: skip %s: %v\n", path, readErr)
			continue
		}

		text := string(data)
		if len(text) > 8000 {
			text = text[:8000]
		}

		vec, embedErr := embedder.Embed(context.Background(), text)
		if embedErr != nil {
			fmt.Fprintf(os.Stderr, "warning: embed failed for %s: %v\n", path, embedErr)
			continue
		}

		rel, _ := filepath.Rel(memDir, path)
		if idxErr := vdb.Index(context.Background(), rel, text, vec); idxErr != nil {
			fmt.Fprintf(os.Stderr, "warning: index failed for %s: %v\n", path, idxErr)
			continue
		}

		indexed++
		fmt.Printf("  indexed: %s\n", rel)
	}

	fmt.Printf("\nDone. Indexed %d/%d files.\n", indexed, len(files))
	return nil
}

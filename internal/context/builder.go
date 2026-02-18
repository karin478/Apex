package context

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SearchResult represents a single result from a memory search.
type SearchResult struct {
	ID    string
	Text  string
	Score float32
	Type  string
}

// Searcher is an interface for searching memory/knowledge stores.
type Searcher interface {
	Search(ctx context.Context, query string, topK int) ([]SearchResult, error)
}

// Options configures the context Builder.
type Options struct {
	TokenBudget int
	Searcher    Searcher
	Files       []string
}

// Builder assembles optimized prompts within a token budget.
type Builder struct {
	opts Options
}

// NewBuilder creates a new Builder with the given options.
func NewBuilder(opts Options) *Builder {
	return &Builder{opts: opts}
}

// ContentBlock represents a block of content to include in the prompt.
type ContentBlock struct {
	ID       string
	Source   string
	Path     string
	Text     string
	Policy   CompressionPolicy
	Priority int
}

// Build assembles an optimized prompt for the given task within the token budget.
// It gathers content from the task, memory search results, and files, then
// compresses and degrades content as needed to fit the budget.
func (b *Builder) Build(ctx context.Context, task string) (string, error) {
	var blocks []ContentBlock

	// 1. Create task block (highest priority, exact policy).
	blocks = append(blocks, ContentBlock{
		ID:       "task",
		Source:   "task",
		Path:     "",
		Text:     task,
		Policy:   PolicyExact,
		Priority: 100,
	})

	// 2. Search for memory if a Searcher is configured.
	if b.opts.Searcher != nil {
		results, err := b.opts.Searcher.Search(ctx, task, 10)
		if err == nil {
			for _, r := range results {
				blocks = append(blocks, ContentBlock{
					ID:       r.ID,
					Source:   "memory",
					Path:     r.ID,
					Text:     r.Text,
					Policy:   PolicySummarizable,
					Priority: 80,
				})
			}
		}
		// On search error, we skip memory silently.
	}

	// 3. Read files and classify them.
	for _, path := range b.opts.Files {
		data, err := os.ReadFile(path)
		if err != nil {
			// Skip files that can't be read.
			continue
		}
		blocks = append(blocks, ContentBlock{
			ID:       path,
			Source:   "file",
			Path:     path,
			Text:     string(data),
			Policy:   classifyFile(path),
			Priority: 60,
		})
	}

	// 4. Sort blocks by priority descending.
	sort.SliceStable(blocks, func(i, j int) bool {
		return blocks[i].Priority > blocks[j].Priority
	})

	// 5. Apply compression to fit within budget.
	blocks = fitBudget(blocks, b.opts.TokenBudget)

	// 6. Assemble the final prompt.
	return assemble(blocks), nil
}

// classifyFile returns the appropriate CompressionPolicy for a file based on
// its extension.
func classifyFile(path string) CompressionPolicy {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".py", ".js", ".ts", ".java", ".rs", ".c", ".cpp":
		return PolicyStructural
	case ".md", ".txt", ".rst":
		return PolicySummarizable
	case ".json", ".yaml", ".yml", ".toml":
		return PolicyExact
	default:
		return PolicySummarizable
	}
}

// fitBudget compresses and degrades blocks to fit within the token budget.
// It preserves original text for each block so that compression can be
// re-applied at increasing levels of aggressiveness.
func fitBudget(blocks []ContentBlock, budget int) []ContentBlock {
	// Save original text so we can re-compress from source at each level.
	type blockState struct {
		original string
		applied  bool // whether compression has been applied
	}
	state := make([]blockState, len(blocks))
	for i := range blocks {
		state[i] = blockState{original: blocks[i].Text, applied: false}
	}

	// Iteratively compress, degrade, and remove blocks until we fit.
	for {
		total := totalTokens(blocks)
		if total <= budget {
			break
		}

		// First, try to compress a block that hasn't been compressed yet
		// (lowest priority first).
		compressed := false
		for i := len(blocks) - 1; i >= 0; i-- {
			if blocks[i].Source == "task" {
				continue
			}
			if !state[i].applied {
				blocks[i].Text = Compress(blocks[i].Policy, blocks[i].Path, state[i].original)
				state[i].applied = true
				compressed = true
				break
			}
		}
		if compressed {
			continue
		}

		// All blocks are compressed. Try to degrade the lowest-priority
		// non-task block.
		degraded := false
		for i := len(blocks) - 1; i >= 0; i-- {
			if blocks[i].Source == "task" {
				continue
			}
			newPolicy := Degrade(blocks[i].Policy)
			if newPolicy != blocks[i].Policy {
				blocks[i].Policy = newPolicy
				blocks[i].Text = Compress(newPolicy, blocks[i].Path, state[i].original)
				degraded = true
				break
			}
		}

		if degraded {
			continue
		}

		// Nothing could be degraded further. Remove the lowest-priority
		// non-task block.
		removed := false
		for i := len(blocks) - 1; i >= 0; i-- {
			if blocks[i].Source != "task" {
				blocks = append(blocks[:i], blocks[i+1:]...)
				state = append(state[:i], state[i+1:]...)
				removed = true
				break
			}
		}

		if !removed {
			// Only the task block remains; nothing more we can do.
			break
		}
	}

	return blocks
}

// totalTokens returns the estimated total token count across all blocks.
func totalTokens(blocks []ContentBlock) int {
	total := 0
	for _, b := range blocks {
		total += EstimateTokens(b.Text)
	}
	return total
}

// assemble renders the final prompt from the content blocks.
func assemble(blocks []ContentBlock) string {
	var sb strings.Builder

	// Collect blocks by source type.
	var taskText string
	var memoryBlocks []ContentBlock
	var fileBlocks []ContentBlock

	for _, b := range blocks {
		switch b.Source {
		case "task":
			taskText = b.Text
		case "memory":
			memoryBlocks = append(memoryBlocks, b)
		case "file":
			fileBlocks = append(fileBlocks, b)
		}
	}

	// Task section (always present).
	sb.WriteString("## Task\n\n")
	sb.WriteString(taskText)

	// Memory section.
	if len(memoryBlocks) > 0 {
		sb.WriteString("\n\n## Relevant Memory\n\n")
		for _, m := range memoryBlocks {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", m.Path, m.Text))
		}
	}

	// File sections.
	for _, f := range fileBlocks {
		sb.WriteString(fmt.Sprintf("\n\n## File: %s\n\n%s", f.Path, f.Text))
	}

	return sb.String()
}

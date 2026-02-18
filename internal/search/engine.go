package search

import (
	"context"
	"sort"

	"github.com/lyndonlyu/apex/internal/memory"
	"github.com/lyndonlyu/apex/internal/vectordb"
)

const (
	vectorWeight  = 0.6
	keywordWeight = 0.4
)

// Embedder generates vector embeddings from text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Available() bool
}

// Result represents a single hybrid search result.
type Result struct {
	ID     string
	Text   string
	Score  float32
	Source string // "vector" | "keyword" | "both"
	Type   string // "decision" | "fact" | "session"
}

// Engine performs hybrid search combining vector and keyword results.
type Engine struct {
	vectorDB *vectordb.VectorDB
	memStore *memory.Store
	embedder Embedder
}

// New creates a new hybrid search engine. Any parameter can be nil for graceful degradation.
func New(vdb *vectordb.VectorDB, store *memory.Store, embedder Embedder) *Engine {
	return &Engine{
		vectorDB: vdb,
		memStore: store,
		embedder: embedder,
	}
}

// Hybrid runs vector + keyword search in parallel and merges results.
func (e *Engine) Hybrid(ctx context.Context, query string, topK int) ([]Result, error) {
	merged := make(map[string]*Result)

	// Vector search (if available)
	vectorOK := false
	if e.vectorDB != nil && e.embedder != nil && e.embedder.Available() {
		queryVec, err := e.embedder.Embed(ctx, query)
		if err == nil {
			vecResults, err := e.vectorDB.Search(ctx, queryVec, topK*2)
			if err == nil {
				vectorOK = true
				for _, vr := range vecResults {
					score := 1.0 / (1.0 + vr.Distance)
					merged[vr.MemoryID] = &Result{
						ID:     vr.MemoryID,
						Text:   vr.Text,
						Score:  float32(score) * vectorWeight,
						Source: "vector",
					}
				}
			}
		}
	}

	// Keyword search (always available)
	if e.memStore != nil {
		kwResults, err := e.memStore.Search(query)
		if err == nil {
			for _, kr := range kwResults {
				if existing, ok := merged[kr.Path]; ok {
					existing.Score += keywordWeight
					existing.Source = "both"
					existing.Type = kr.Type
				} else {
					merged[kr.Path] = &Result{
						ID:     kr.Path,
						Text:   kr.Snippet,
						Score:  keywordWeight,
						Source: "keyword",
						Type:   kr.Type,
					}
				}
			}
		}
	}

	// If no vector search was done, keyword results get full score
	if !vectorOK {
		for _, r := range merged {
			if r.Source == "keyword" {
				r.Score = 1.0
			}
		}
	}

	// Sort by score descending
	results := make([]Result, 0, len(merged))
	for _, r := range merged {
		results = append(results, *r)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

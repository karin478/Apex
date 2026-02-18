package context

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSearchEngine struct {
	results []SearchResult
	err     error
}

func (m *mockSearchEngine) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	return m.results, m.err
}

func TestBuildBasicPrompt(t *testing.T) {
	b := NewBuilder(Options{TokenBudget: 60000})
	result, err := b.Build(context.Background(), "Write a hello world program")
	require.NoError(t, err)
	assert.Contains(t, result, "Write a hello world program")
}

func TestBuildWithMemory(t *testing.T) {
	engine := &mockSearchEngine{
		results: []SearchResult{
			{ID: "facts/go.md", Text: "We use Go 1.25", Score: 0.9, Type: "fact"},
		},
	}
	b := NewBuilder(Options{TokenBudget: 60000, Searcher: engine})
	result, err := b.Build(context.Background(), "Write a Go function")
	require.NoError(t, err)
	assert.Contains(t, result, "Write a Go function")
	assert.Contains(t, result, "We use Go 1.25")
}

func TestBuildWithFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(dir+"/main.go", []byte("package main\n\nfunc main() {}\n"), 0644))

	b := NewBuilder(Options{
		TokenBudget: 60000,
		Files:       []string{dir + "/main.go"},
	})
	result, err := b.Build(context.Background(), "Modify the main function")
	require.NoError(t, err)
	assert.Contains(t, result, "package main")
}

func TestBuildBudgetOverflow(t *testing.T) {
	b := NewBuilder(Options{TokenBudget: 50})
	result, err := b.Build(context.Background(), "short task")
	require.NoError(t, err)
	assert.Contains(t, result, "short task")
}

func TestBuildExactExceedsBudget(t *testing.T) {
	b := NewBuilder(Options{TokenBudget: 1})
	result, err := b.Build(context.Background(), "task")
	require.NoError(t, err)
	assert.Contains(t, result, "task")
}

func TestBuildFileReadError(t *testing.T) {
	b := NewBuilder(Options{
		TokenBudget: 60000,
		Files:       []string{"/nonexistent/file.go"},
	})
	result, err := b.Build(context.Background(), "do something")
	require.NoError(t, err)
	assert.Contains(t, result, "do something")
}

func TestBuildSearchError(t *testing.T) {
	engine := &mockSearchEngine{err: assert.AnError}
	b := NewBuilder(Options{TokenBudget: 60000, Searcher: engine})
	result, err := b.Build(context.Background(), "do something")
	require.NoError(t, err)
	assert.Contains(t, result, "do something")
}

func TestBuildDegradation(t *testing.T) {
	engine := &mockSearchEngine{
		results: []SearchResult{
			{ID: "facts/a.md", Text: "fact A content that is reasonably long to consume tokens", Score: 0.9, Type: "fact"},
			{ID: "facts/b.md", Text: "fact B content that is also reasonably long to consume tokens", Score: 0.5, Type: "fact"},
		},
	}
	b := NewBuilder(Options{TokenBudget: 80, Searcher: engine})
	result, err := b.Build(context.Background(), "task")
	require.NoError(t, err)
	assert.Contains(t, result, "task")
}

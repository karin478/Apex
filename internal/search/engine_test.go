package search

import (
	"context"
	"testing"

	"github.com/lyndonlyu/apex/internal/memory"
	"github.com/lyndonlyu/apex/internal/vectordb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEmbedder struct {
	available bool
	vec       []float32
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return m.vec, nil
}

func (m *mockEmbedder) Available() bool {
	return m.available
}

func TestNewEngine(t *testing.T) {
	e := New(nil, nil, nil)
	assert.NotNil(t, e)
}

func TestHybridKeywordOnly(t *testing.T) {
	dir := t.TempDir()
	store, _ := memory.NewStore(dir)
	store.SaveFact("golang", "Go is a programming language")

	e := New(nil, store, nil)
	results, err := e.Hybrid(context.Background(), "programming", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.Equal(t, "keyword", results[0].Source)
}

func TestHybridVectorOnly(t *testing.T) {
	dir := t.TempDir()
	store, _ := memory.NewStore(dir)

	dbPath := dir + "/vectors.db"
	vdb, err := vectordb.Open(dbPath, 4)
	require.NoError(t, err)
	defer vdb.Close()

	vec := make([]float32, 4)
	for i := range vec {
		vec[i] = 0.5
	}
	vdb.Index(context.Background(), "fact-1", "Go is a programming language", vec)

	embedder := &mockEmbedder{available: true, vec: vec}

	e := New(vdb, store, embedder)
	results, err := e.Hybrid(context.Background(), "programming", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestHybridEmbedderUnavailable(t *testing.T) {
	dir := t.TempDir()
	store, _ := memory.NewStore(dir)
	store.SaveFact("golang", "Go is a programming language")

	embedder := &mockEmbedder{available: false}

	e := New(nil, store, embedder)
	results, err := e.Hybrid(context.Background(), "programming", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.Equal(t, "keyword", results[0].Source)
}

func TestHybridNoResults(t *testing.T) {
	dir := t.TempDir()
	store, _ := memory.NewStore(dir)

	e := New(nil, store, nil)
	results, err := e.Hybrid(context.Background(), "nonexistent_xyz_12345", 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestHybridMerge(t *testing.T) {
	dir := t.TempDir()
	store, _ := memory.NewStore(dir)
	store.SaveFact("golang", "Go is a programming language")

	dbPath := dir + "/vectors.db"
	vdb, err := vectordb.Open(dbPath, 4)
	require.NoError(t, err)
	defer vdb.Close()

	vec := make([]float32, 4)
	for i := range vec {
		vec[i] = 0.5
	}
	vdb.Index(context.Background(), "vector-entry", "Go programming language", vec)

	embedder := &mockEmbedder{available: true, vec: vec}

	e := New(vdb, store, embedder)
	results, err := e.Hybrid(context.Background(), "programming", 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1)
}

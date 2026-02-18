package vectordb

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestVector(dim int, val float32) []float32 {
	v := make([]float32, dim)
	for i := range v {
		v[i] = val
	}
	return v
}

func TestOpenAndClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	vdb, err := Open(dbPath, 4)
	require.NoError(t, err)
	require.NotNil(t, vdb)
	assert.NoError(t, vdb.Close())
}

func TestOpenCreatesFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	vdb, err := Open(dbPath, 4)
	require.NoError(t, err)
	defer vdb.Close()
	assert.FileExists(t, dbPath)
}

func TestIndex(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	vdb, err := Open(dbPath, 4)
	require.NoError(t, err)
	defer vdb.Close()

	vec := makeTestVector(4, 0.5)
	err = vdb.Index(context.Background(), "mem-1", "test memory content", vec)
	assert.NoError(t, err)
}

func TestCount(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	vdb, err := Open(dbPath, 4)
	require.NoError(t, err)
	defer vdb.Close()

	count, err := vdb.Count()
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	vdb.Index(context.Background(), "mem-1", "one", makeTestVector(4, 0.1))
	vdb.Index(context.Background(), "mem-2", "two", makeTestVector(4, 0.2))

	count, err = vdb.Count()
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestSearch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	vdb, err := Open(dbPath, 4)
	require.NoError(t, err)
	defer vdb.Close()

	vdb.Index(context.Background(), "close", "close match", makeTestVector(4, 0.9))
	vdb.Index(context.Background(), "medium", "medium match", makeTestVector(4, 0.5))
	vdb.Index(context.Background(), "far", "far match", makeTestVector(4, 0.1))

	query := makeTestVector(4, 0.85)
	results, err := vdb.Search(context.Background(), query, 2)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "close", results[0].MemoryID)
}

func TestDelete(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	vdb, err := Open(dbPath, 4)
	require.NoError(t, err)
	defer vdb.Close()

	vdb.Index(context.Background(), "mem-1", "one", makeTestVector(4, 0.5))

	count, _ := vdb.Count()
	assert.Equal(t, 1, count)

	err = vdb.Delete("mem-1")
	assert.NoError(t, err)

	count, _ = vdb.Count()
	assert.Equal(t, 0, count)
}

func TestSearchEmpty(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	vdb, err := Open(dbPath, 4)
	require.NoError(t, err)
	defer vdb.Close()

	results, err := vdb.Search(context.Background(), makeTestVector(4, 0.5), 5)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestIndexDuplicate(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	vdb, err := Open(dbPath, 4)
	require.NoError(t, err)
	defer vdb.Close()

	vec := makeTestVector(4, 0.5)
	require.NoError(t, vdb.Index(context.Background(), "mem-1", "original", vec))

	vec2 := makeTestVector(4, 0.9)
	require.NoError(t, vdb.Index(context.Background(), "mem-1", "updated", vec2))

	count, _ := vdb.Count()
	assert.Equal(t, 1, count)
}

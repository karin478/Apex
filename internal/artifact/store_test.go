package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(t.TempDir())
}

func TestSaveAndGet(t *testing.T) {
	s := testStore(t)
	data := []byte("hello world")

	art, err := s.Save("greeting.txt", data, "run-1", "node-a")
	require.NoError(t, err)

	// Verify fields.
	h := sha256.Sum256(data)
	wantHash := hex.EncodeToString(h[:])
	assert.Equal(t, wantHash, art.Hash)
	assert.Equal(t, "greeting.txt", art.Name)
	assert.Equal(t, int64(len(data)), art.Size)

	// Get back by hash.
	got, err := s.Get(art.Hash)
	require.NoError(t, err)
	assert.Equal(t, art.Hash, got.Hash)
	assert.Equal(t, art.Name, got.Name)
}

func TestSaveDeduplicates(t *testing.T) {
	s := testStore(t)
	data := []byte("duplicate content")

	a1, err := s.Save("file1.txt", data, "run-1", "node-a")
	require.NoError(t, err)

	a2, err := s.Save("file2.txt", data, "run-2", "node-b")
	require.NoError(t, err)

	// Same hash returned.
	assert.Equal(t, a1.Hash, a2.Hash)

	// Only one entry in the index.
	list, err := s.List()
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestData(t *testing.T) {
	s := testStore(t)
	content := []byte("binary payload \x00\xff")

	art, err := s.Save("payload.bin", content, "run-1", "node-a")
	require.NoError(t, err)

	got, err := s.Data(art.Hash)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestListEmpty(t *testing.T) {
	s := testStore(t)

	list, err := s.List()
	require.NoError(t, err)
	assert.Nil(t, list)
}

func TestListByRun(t *testing.T) {
	s := testStore(t)

	_, err := s.Save("a.txt", []byte("aaa"), "run-1", "node-a")
	require.NoError(t, err)
	_, err = s.Save("b.txt", []byte("bbb"), "run-1", "node-b")
	require.NoError(t, err)
	_, err = s.Save("c.txt", []byte("ccc"), "run-2", "node-c")
	require.NoError(t, err)

	result, err := s.ListByRun("run-1")
	require.NoError(t, err)
	assert.Len(t, result, 2)
	for _, a := range result {
		assert.Equal(t, "run-1", a.RunID)
	}
}

func TestRemove(t *testing.T) {
	s := testStore(t)

	art, err := s.Save("tmp.txt", []byte("temporary"), "run-1", "node-a")
	require.NoError(t, err)

	err = s.Remove(art.Hash)
	require.NoError(t, err)

	_, err = s.Get(art.Hash)
	assert.Error(t, err)
}

func TestRemoveNotFound(t *testing.T) {
	s := testStore(t)

	err := s.Remove("0000000000000000000000000000000000000000000000000000000000000000")
	assert.Error(t, err)
}

func TestFindOrphans(t *testing.T) {
	s := testStore(t)

	_, err := s.Save("kept.txt", []byte("kept"), "run-valid", "node-a")
	require.NoError(t, err)
	_, err = s.Save("orphan.txt", []byte("orphan"), "run-gone", "node-b")
	require.NoError(t, err)

	orphans, err := s.FindOrphans(map[string]bool{"run-valid": true})
	require.NoError(t, err)
	require.Len(t, orphans, 1)
	assert.Equal(t, "run-gone", orphans[0].RunID)
}

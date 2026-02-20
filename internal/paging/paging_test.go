package paging

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultBudget(t *testing.T) {
	b := DefaultBudget()
	assert.Equal(t, 10, b.MaxPages)
	assert.Equal(t, 8000, b.MaxTokens)
	assert.Equal(t, 0, b.PagesUsed)
	assert.Equal(t, 0, b.TokensUsed)
}

func TestNewBudget(t *testing.T) {
	b := NewBudget(5, 4000)
	assert.Equal(t, 5, b.MaxPages)
	assert.Equal(t, 4000, b.MaxTokens)
}

func TestBudgetCanPage(t *testing.T) {
	t.Run("fresh budget", func(t *testing.T) {
		b := DefaultBudget()
		assert.True(t, b.CanPage())
	})

	t.Run("pages exhausted", func(t *testing.T) {
		b := NewBudget(1, 8000)
		b.Record(100)
		assert.False(t, b.CanPage())
	})

	t.Run("tokens exhausted", func(t *testing.T) {
		b := NewBudget(10, 100)
		b.Record(100)
		assert.False(t, b.CanPage())
	})
}

func TestBudgetRecord(t *testing.T) {
	b := DefaultBudget()
	b.Record(500)
	assert.Equal(t, 1, b.PagesUsed)
	assert.Equal(t, 500, b.TokensUsed)

	b.Record(300)
	assert.Equal(t, 2, b.PagesUsed)
	assert.Equal(t, 800, b.TokensUsed)

	pages, tokens := b.Remaining()
	assert.Equal(t, 8, pages)
	assert.Equal(t, 7200, tokens)
}

func TestEstimateTokens(t *testing.T) {
	assert.Equal(t, 0, EstimateTokens(""))
	assert.Equal(t, 1, EstimateTokens("abcd"))
	assert.Equal(t, 3, EstimateTokens("hello world!"))
	assert.Equal(t, 25, EstimateTokens(strings.Repeat("x", 100)))
}

func TestPagerPage(t *testing.T) {
	store := &mockStore{
		data: map[string]string{
			"art-1": "line1\nline2\nline3\nline4\nline5",
		},
	}
	pager := NewPager(store, DefaultBudget())

	t.Run("full content", func(t *testing.T) {
		result, err := pager.Page(PageRequest{ArtifactID: "art-1"})
		require.NoError(t, err)
		assert.Equal(t, "art-1", result.ArtifactID)
		assert.Equal(t, 5, result.Lines)
		assert.Contains(t, result.Content, "line1")
		assert.Contains(t, result.Content, "line5")
		assert.Greater(t, result.Tokens, 0)
	})

	t.Run("line range", func(t *testing.T) {
		result, err := pager.Page(PageRequest{
			ArtifactID: "art-1",
			StartLine:  2,
			EndLine:    4,
		})
		require.NoError(t, err)
		assert.Equal(t, 3, result.Lines)
		assert.Contains(t, result.Content, "line2")
		assert.Contains(t, result.Content, "line4")
		assert.NotContains(t, result.Content, "line1")
		assert.NotContains(t, result.Content, "line5")
	})

	t.Run("not found", func(t *testing.T) {
		_, err := pager.Page(PageRequest{ArtifactID: "nope"})
		assert.Error(t, err)
	})
}

func TestPagerPageBudgetExhausted(t *testing.T) {
	store := &mockStore{
		data: map[string]string{
			"art-1": "content",
		},
	}
	budget := NewBudget(1, 8000)

	pager := NewPager(store, budget)

	// First page succeeds.
	_, err := pager.Page(PageRequest{ArtifactID: "art-1"})
	require.NoError(t, err)

	// Second page fails — budget exhausted.
	_, err = pager.Page(PageRequest{ArtifactID: "art-1"})
	assert.ErrorIs(t, err, ErrBudgetExhausted)
}

// mockStore is a simple in-memory ContentStore for testing.
type mockStore struct {
	data map[string]string
}

func (m *mockStore) GetContent(artifactID string) (string, error) {
	content, ok := m.data[artifactID]
	if !ok {
		return "", fmt.Errorf("artifact %q not found", artifactID)
	}
	return content, nil
}

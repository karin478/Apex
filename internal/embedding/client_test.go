package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	c := NewClient("test-key", "text-embedding-3-small", 1536)
	assert.NotNil(t, c)
}

func TestNewClientMissingKey(t *testing.T) {
	c := NewClient("", "text-embedding-3-small", 1536)
	assert.NotNil(t, c)
	assert.False(t, c.Available())
}

func TestClientAvailable(t *testing.T) {
	c := NewClient("test-key", "text-embedding-3-small", 1536)
	assert.True(t, c.Available())
}

func TestEmbed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/embeddings", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		var req embeddingRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "text-embedding-3-small", req.Model)
		assert.Equal(t, "hello world", req.Input)

		resp := embeddingResponse{
			Data: []embeddingData{
				{Embedding: make([]float32, 1536)},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient("test-key", "text-embedding-3-small", 1536)
	c.baseURL = server.URL

	vec, err := c.Embed(context.Background(), "hello world")
	require.NoError(t, err)
	assert.Len(t, vec, 1536)
}

func TestEmbedBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := embeddingResponse{
			Data: []embeddingData{
				{Embedding: make([]float32, 1536)},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient("test-key", "text-embedding-3-small", 1536)
	c.baseURL = server.URL

	vecs, err := c.EmbedBatch(context.Background(), []string{"one", "two"})
	require.NoError(t, err)
	assert.Len(t, vecs, 2)
	assert.Len(t, vecs[0], 1536)
}

func TestEmbedUnavailable(t *testing.T) {
	c := NewClient("", "text-embedding-3-small", 1536)
	_, err := c.Embed(context.Background(), "hello")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key")
}

func TestEmbedAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": {"message": "rate limited"}}`))
	}))
	defer server.Close()

	c := NewClient("test-key", "text-embedding-3-small", 1536)
	c.baseURL = server.URL

	_, err := c.Embed(context.Background(), "hello")
	assert.Error(t, err)
}

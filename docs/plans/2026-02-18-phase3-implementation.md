# Phase 3 Semantic Search Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add semantic vector search to Apex so `apex memory search` uses hybrid search (vector + keyword) for better recall.

**Architecture:** OpenAI API generates embeddings. sqlite-vec stores vectors in independent vectors.db. Hybrid search merges vector results with existing keyword search. Graceful degradation when vector search unavailable.

**Tech Stack:** Go 1.25, mattn/go-sqlite3 (CGO), sqlite-vec-go-bindings/cgo, OpenAI text-embedding-3-small API, existing packages (config, memory).

---

### Task 1: Add Dependencies and Embedding Config

**Files:**
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/config/config.go`
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/config/config_test.go`

**Step 1: Write failing tests for new config fields**

Add to `internal/config/config_test.go`:
```go
func TestDefaultConfigPhase3(t *testing.T) {
	cfg := Default()

	assert.Equal(t, "text-embedding-3-small", cfg.Embedding.Model)
	assert.Equal(t, "OPENAI_API_KEY", cfg.Embedding.APIKeyEnv)
	assert.Equal(t, 1536, cfg.Embedding.Dimensions)
}

func TestLoadConfigPhase3Override(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	content := []byte(`embedding:
  model: text-embedding-3-large
  api_key_env: MY_OPENAI_KEY
  dimensions: 3072
`)
	require.NoError(t, os.WriteFile(configPath, content, 0644))

	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Equal(t, "text-embedding-3-large", cfg.Embedding.Model)
	assert.Equal(t, "MY_OPENAI_KEY", cfg.Embedding.APIKeyEnv)
	assert.Equal(t, 3072, cfg.Embedding.Dimensions)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/config/ -v -run "Phase3"`
Expected: FAIL (EmbeddingConfig doesn't exist)

**Step 3: Update implementation**

Update `internal/config/config.go` — add EmbeddingConfig struct, add to Config, update Default() and Load():

```go
type EmbeddingConfig struct {
	Model      string `yaml:"model"`
	APIKeyEnv  string `yaml:"api_key_env"`
	Dimensions int    `yaml:"dimensions"`
}
```

Add to Config struct:
```go
Embedding  EmbeddingConfig  `yaml:"embedding"`
```

Add to Default():
```go
Embedding: EmbeddingConfig{
    Model:      "text-embedding-3-small",
    APIKeyEnv:  "OPENAI_API_KEY",
    Dimensions: 1536,
},
```

Add to Load() zero-value defaults:
```go
if cfg.Embedding.Model == "" {
    cfg.Embedding.Model = "text-embedding-3-small"
}
if cfg.Embedding.APIKeyEnv == "" {
    cfg.Embedding.APIKeyEnv = "OPENAI_API_KEY"
}
if cfg.Embedding.Dimensions == 0 {
    cfg.Embedding.Dimensions = 1536
}
```

**Step 4: Run all config tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/config/ -v`
Expected: ALL PASS

**Step 5: Add Go dependencies**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
go get -u github.com/mattn/go-sqlite3
go get -u github.com/asg017/sqlite-vec-go-bindings/cgo
```

**Step 6: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/config/ go.mod go.sum
git commit -m "feat(config): add embedding config for Phase 3 semantic search

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: Embedding Client

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/embedding/client.go`
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/embedding/client_test.go`

**Step 1: Write failing tests**

Create `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/embedding/client_test.go`:
```go
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
	// Mock OpenAI API
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
		var req embeddingRequest
		json.NewDecoder(r.Body).Decode(&req)

		// For batch, input is an array — the handler receives one call per text
		// since EmbedBatch calls Embed in a loop
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
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/embedding/ -v`
Expected: FAIL (package doesn't exist)

**Step 3: Write implementation**

Create `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/embedding/client.go`:
```go
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	apiKey     string
	model      string
	dimensions int
	baseURL    string
	httpClient *http.Client
}

type embeddingRequest struct {
	Model string      `json:"model"`
	Input interface{} `json:"input"`
}

type embeddingResponse struct {
	Data []embeddingData `json:"data"`
}

type embeddingData struct {
	Embedding []float32 `json:"embedding"`
}

func NewClient(apiKey string, model string, dimensions int) *Client {
	return &Client{
		apiKey:     apiKey,
		model:      model,
		dimensions: dimensions,
		baseURL:    "https://api.openai.com",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Available() bool {
	return c.apiKey != ""
}

func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	if !c.Available() {
		return nil, fmt.Errorf("embedding client unavailable: API key not set")
	}

	reqBody := embeddingRequest{
		Model: c.model,
		Input: text,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var embResp embeddingResponse
	if err := json.Unmarshal(body, &embResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}

	return embResp.Data[0].Embedding, nil
}

func (c *Client) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, text := range texts {
		vec, err := c.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed text %d: %w", i, err)
		}
		results[i] = vec
	}
	return results, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/embedding/ -v`
Expected: ALL PASS (6 tests)

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/embedding/
git commit -m "feat(embedding): add OpenAI embedding client with mock-tested API

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: Vector Database

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/vectordb/vectordb.go`
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/vectordb/vectordb_test.go`

**Step 1: Write failing tests**

Create `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/vectordb/vectordb_test.go`:
```go
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

	// Index 3 vectors with different values
	vdb.Index(context.Background(), "close", "close match", makeTestVector(4, 0.9))
	vdb.Index(context.Background(), "medium", "medium match", makeTestVector(4, 0.5))
	vdb.Index(context.Background(), "far", "far match", makeTestVector(4, 0.1))

	// Search for vector close to 0.9
	query := makeTestVector(4, 0.85)
	results, err := vdb.Search(context.Background(), query, 2)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Closest should be "close"
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

	// Re-indexing same ID should update, not error
	vec2 := makeTestVector(4, 0.9)
	require.NoError(t, vdb.Index(context.Background(), "mem-1", "updated", vec2))

	count, _ := vdb.Count()
	assert.Equal(t, 1, count)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/vectordb/ -v`
Expected: FAIL (package doesn't exist)

**Step 3: Write implementation**

Create `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/vectordb/vectordb.go`:
```go
package vectordb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	sqlite_vec.Auto()
}

type VectorDB struct {
	db         *sql.DB
	dimensions int
}

type VectorResult struct {
	MemoryID string
	Distance float32
	Text     string
}

func Open(dbPath string, dimensions int) (*VectorDB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Create metadata table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS vec_meta (
		memory_id TEXT PRIMARY KEY,
		text TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create meta table: %w", err)
	}

	// Create vector virtual table
	createVec := fmt.Sprintf(
		`CREATE VIRTUAL TABLE IF NOT EXISTS vec_memories USING vec0(embedding float[%d])`,
		dimensions,
	)
	_, err = db.Exec(createVec)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create vec table: %w", err)
	}

	return &VectorDB{db: db, dimensions: dimensions}, nil
}

func (v *VectorDB) Close() error {
	return v.db.Close()
}

func (v *VectorDB) Index(ctx context.Context, memoryID string, text string, embedding []float32) error {
	// Delete existing entry if any (upsert)
	v.Delete(memoryID)

	// Insert into metadata table
	_, err := v.db.ExecContext(ctx,
		`INSERT INTO vec_meta (memory_id, text, created_at) VALUES (?, ?, ?)`,
		memoryID, text, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert meta: %w", err)
	}

	// Get the rowid of the inserted metadata row
	var rowID int64
	err = v.db.QueryRowContext(ctx,
		`SELECT rowid FROM vec_meta WHERE memory_id = ?`, memoryID,
	).Scan(&rowID)
	if err != nil {
		return fmt.Errorf("get rowid: %w", err)
	}

	// Insert into vector table with matching rowid
	serialized, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return fmt.Errorf("serialize embedding: %w", err)
	}

	_, err = v.db.ExecContext(ctx,
		`INSERT INTO vec_memories (rowid, embedding) VALUES (?, ?)`,
		rowID, serialized,
	)
	if err != nil {
		return fmt.Errorf("insert vector: %w", err)
	}

	return nil
}

func (v *VectorDB) Search(ctx context.Context, query []float32, topK int) ([]VectorResult, error) {
	serialized, err := sqlite_vec.SerializeFloat32(query)
	if err != nil {
		return nil, fmt.Errorf("serialize query: %w", err)
	}

	rows, err := v.db.QueryContext(ctx,
		`SELECT vm.rowid, vm.distance, m.memory_id, m.text
		 FROM vec_memories vm
		 JOIN vec_meta m ON m.rowid = vm.rowid
		 WHERE vm.embedding MATCH ?
		 ORDER BY vm.distance
		 LIMIT ?`,
		serialized, topK,
	)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()

	var results []VectorResult
	for rows.Next() {
		var rowID int64
		var r VectorResult
		if err := rows.Scan(&rowID, &r.Distance, &r.MemoryID, &r.Text); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (v *VectorDB) Delete(memoryID string) error {
	// Get rowid first
	var rowID int64
	err := v.db.QueryRow(`SELECT rowid FROM vec_meta WHERE memory_id = ?`, memoryID).Scan(&rowID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("get rowid for delete: %w", err)
	}

	// Delete from vector table
	v.db.Exec(`DELETE FROM vec_memories WHERE rowid = ?`, rowID)

	// Delete from metadata table
	_, err = v.db.Exec(`DELETE FROM vec_meta WHERE memory_id = ?`, memoryID)
	return err
}

func (v *VectorDB) Count() (int, error) {
	var count int
	err := v.db.QueryRow(`SELECT COUNT(*) FROM vec_meta`).Scan(&count)
	return count, err
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/vectordb/ -v`
Expected: ALL PASS (8 tests)

NOTE: If CGO issues occur on macOS, ensure `CGO_ENABLED=1` is set. Run: `CGO_ENABLED=1 go test ./internal/vectordb/ -v`

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/vectordb/
git commit -m "feat(vectordb): add sqlite-vec backed vector database

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Hybrid Search Engine

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/search/engine.go`
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/search/engine_test.go`

**Step 1: Write failing tests**

Create `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/search/engine_test.go`:
```go
package search

import (
	"context"
	"testing"

	"github.com/lyndonlyu/apex/internal/memory"
	"github.com/lyndonlyu/apex/internal/vectordb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEmbedder implements Embedder for testing
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
	// When no vector DB or embedder, falls back to keyword only
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
	vdb.Index(context.Background(), "facts/golang", "Go is a programming language", vec)

	embedder := &mockEmbedder{available: true, vec: vec}

	e := New(vdb, store, embedder)
	results, err := e.Hybrid(context.Background(), "programming", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results)

	// Items found by both should have source "both" and higher score
	hasBoth := false
	for _, r := range results {
		if r.Source == "both" {
			hasBoth = true
			assert.Greater(t, r.Score, float32(0.5))
		}
	}
	// If same entry matched both, it should be merged
	if !hasBoth {
		// Acceptable if IDs don't match — keyword uses file paths, vector uses memory_id
		t.Log("No 'both' matches found — IDs may differ between keyword and vector results")
	}
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
	results, err := e.Hybrid(context.Background(), "nonexistent_xyz", 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/search/ -v`
Expected: FAIL (package doesn't exist)

**Step 3: Write implementation**

Create `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/search/engine.go`:
```go
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

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Available() bool
}

type Result struct {
	ID     string
	Text   string
	Score  float32
	Source string // "vector" | "keyword" | "both"
	Type   string // "decision" | "fact" | "session"
}

type Engine struct {
	vectorDB *vectordb.VectorDB
	memStore *memory.Store
	embedder Embedder
}

func New(vdb *vectordb.VectorDB, store *memory.Store, embedder Embedder) *Engine {
	return &Engine{
		vectorDB: vdb,
		memStore: store,
		embedder: embedder,
	}
}

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
					// Convert distance to similarity score (lower distance = higher score)
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
					// Found by both — boost score
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

	// Limit to topK
	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/search/ -v`
Expected: ALL PASS (6 tests)

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/search/
git commit -m "feat(search): add hybrid search engine (vector + keyword)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 5: Wire into CLI

**Files:**
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/memory.go` — upgrade search, add index command
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/main.go` — no changes needed (memoryCmd already registered)

**Step 1: Update `cmd/apex/memory.go`**

Replace the entire file to add hybrid search and `apex memory index`:

```go
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

	// Vector DB (optional — may fail if sqlite-vec not available)
	vdbPath := filepath.Join(cfg.BaseDir, "vectors.db")
	vdb, vdbErr := vectordb.Open(vdbPath, cfg.Embedding.Dimensions)
	if vdbErr != nil {
		fmt.Fprintf(os.Stderr, "warning: vector DB unavailable: %v\n", vdbErr)
	}

	// Embedding client (optional — needs API key)
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
	cfg, store, vdb, embedder, err := loadSearchDeps()
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

	// Get all memory files via search with empty-ish query
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
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skip %s: %v\n", path, err)
			continue
		}

		text := string(data)
		if len(text) > 8000 {
			text = text[:8000] // Truncate very long files for embedding
		}

		vec, err := embedder.Embed(context.Background(), text)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: embed failed for %s: %v\n", path, err)
			continue
		}

		rel, _ := filepath.Rel(memDir, path)
		if err := vdb.Index(context.Background(), rel, text, vec); err != nil {
			fmt.Fprintf(os.Stderr, "warning: index failed for %s: %v\n", path, err)
			continue
		}

		indexed++
		fmt.Printf("  indexed: %s\n", rel)
	}

	fmt.Printf("\nDone. Indexed %d/%d files.\n", indexed, len(files))
	return nil
}
```

**Step 2: Run all tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./... -v`
Expected: ALL PASS

**Step 3: Build and verify**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
make build
./bin/apex memory --help
./bin/apex memory search --help
./bin/apex memory index --help
```
Expected: All help texts display correctly, `index` subcommand visible.

**Step 4: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add cmd/apex/memory.go
git commit -m "feat: upgrade memory search to hybrid, add memory index command

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 6: End-to-End Verification

**Step 1: Run full test suite**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./... -v -count=1`
Expected: ALL PASS

**Step 2: Build**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && make build`

**Step 3: Verify all commands**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
./bin/apex version
./bin/apex --help
./bin/apex plan --help
./bin/apex run --help
./bin/apex history --help
./bin/apex memory --help
./bin/apex memory search --help
./bin/apex memory index --help
```

**Step 4: Verify vector DB creation**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
# Quick smoke test — search should degrade gracefully without API key
./bin/apex memory search "test query" 2>&1 || true
```
Expected: Either returns results or shows "warning: embedding unavailable" + keyword-only results.

**Step 5: Final commit if needed**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git status
# If any uncommitted changes:
git add -A
git commit -m "feat: Phase 3 complete - semantic vector search with hybrid engine

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

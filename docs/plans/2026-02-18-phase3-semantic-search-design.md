# Phase 3 Semantic Search Design

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scope | MVP — semantic search core only | Reasoning Protocols and Aggregation deferred to Phase 3b/3c |
| Embedding | OpenAI text-embedding-3-small API | Simplest, high quality, user already has API key |
| Storage migration | Add vector layer, keep file layer intact | Minimal invasion, vectors.db is enhancement not replacement |
| SQLite binding | CGO — mattn/go-sqlite3 + sqlite-vec-go-bindings/cgo | Most mature Go SQLite integration |

## Architecture

```
Existing file-system memory          New vectors.db (SQLite + sqlite-vec)
┌──────────────────┐                ┌──────────────────────┐
│ decisions/*.md   │                │ vec_memories          │
│ facts/*.md       │◄───sync───────│   memory_id (FK)      │
│ sessions/*.json  │                │   embedding (BLOB)    │
└──────────────────┘                │   entity_text         │
        │                           │   sync_status         │
        ▼                           └──────────────────────┘
  Keyword search (existing)          Vector search (new, sqlite-vec)
        │                                   │
        └─────────┬─────────────────────────┘
                  ▼
          Hybrid search (new)
     Score = Vector × 0.6 + Keyword × 0.4
```

### Data Flow

1. **Write**: `memory.SaveXxx()` → file write succeeds → `vectordb.Index()` generates embedding via OpenAI API and stores in vectors.db
2. **Search**: `search.Hybrid(query)` → parallel vector search + keyword search → weighted merge → sorted results
3. **Degradation**: If vectors.db unavailable or embedding API fails → automatic fallback to keyword-only search

## New Packages

### 1. `internal/embedding` — OpenAI Embedding Client

```go
type Client struct {
    apiKey string
    model  string  // "text-embedding-3-small"
}

func NewClient(apiKey string) *Client
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error)
func (c *Client) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
```

- Calls OpenAI `/v1/embeddings` endpoint
- Model: `text-embedding-3-small` (1536 dimensions, low cost)
- API key from config (reads env var specified in config)
- 30s timeout, returns error on failure (caller decides degradation)

### 2. `internal/vectordb` — Vector Database

```go
type VectorDB struct {
    db *sql.DB  // SQLite + sqlite-vec
}

func Open(dbPath string) (*VectorDB, error)
func (v *VectorDB) Close() error
func (v *VectorDB) Index(ctx context.Context, id string, text string, embedding []float32) error
func (v *VectorDB) Search(ctx context.Context, query []float32, topK int) ([]VectorResult, error)
func (v *VectorDB) Delete(id string) error
func (v *VectorDB) Count() (int, error)

type VectorResult struct {
    MemoryID string
    Distance float32
    Text     string
}
```

- SQLite file: `~/.apex/vectors.db`
- Table `vec_memories`: id, memory_id, text, embedding (sqlite-vec BLOB), created_at, sync_status
- sqlite-vec maintains HNSW index automatically
- Search returns cosine distance sorted results

### 3. `internal/search` — Hybrid Search Engine

```go
type Engine struct {
    vectorDB  *vectordb.VectorDB
    memStore  *memory.Store
    embedder  *embedding.Client
}

func New(vdb *vectordb.VectorDB, store *memory.Store, embedder *embedding.Client) *Engine
func (e *Engine) Hybrid(ctx context.Context, query string, topK int) ([]Result, error)

type Result struct {
    ID       string
    Text     string
    Score    float32  // 0-1, higher is better
    Source   string   // "vector" | "keyword" | "both"
    Type     string   // "decision" | "fact" | "session"
}
```

- `Hybrid()` runs in parallel:
  - Vector: query → embed → vectordb.Search(topK*2)
  - Keyword: memory.Search(keyword)
- Merge: `Score = Vector × 0.6 + Keyword × 0.4` (overlapping results get boosted)
- Vector search failure → automatic degradation to keyword-only

## Config Changes

New `EmbeddingConfig` added to Config:

```yaml
embedding:
  model: text-embedding-3-small
  api_key_env: OPENAI_API_KEY  # read from this environment variable
  dimensions: 1536
```

Defaults: model=text-embedding-3-small, api_key_env=OPENAI_API_KEY, dimensions=1536

## CLI Changes

- `apex memory search <query>` — upgraded to hybrid search (vector + keyword)
- `apex memory index` — new command, batch-indexes existing memory files into vectors.db

## Performance Targets

| Metric | Target |
|--------|--------|
| Semantic search | < 500ms |
| Keyword search | < 100ms (unchanged) |
| Hybrid merge | < 50ms |
| Single embedding API call | < 2s |
| Batch index (100 entries) | < 30s |

## Degradation Strategy

| Failure | Behavior |
|---------|----------|
| OpenAI API down | Keyword-only search, warn user |
| vectors.db corrupt | Keyword-only search, suggest `apex memory index` to rebuild |
| OPENAI_API_KEY missing | Keyword-only search, warn user to set key |
| vectors.db missing | Auto-create on first write |

## Dependencies

- `github.com/mattn/go-sqlite3` — CGO SQLite driver
- `github.com/asg017/sqlite-vec-go-bindings/cgo` — sqlite-vec extension
- OpenAI API (text-embedding-3-small) — via HTTP, no SDK dependency

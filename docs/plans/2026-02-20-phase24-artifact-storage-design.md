# Phase 24: Content-Addressed Artifact Storage — Design Document

> Date: 2026-02-20
> Status: Approved
> Architecture Ref: v11.0 §2.13 Maintenance Subsystem / Content Addressing

## 1. Goal

Content-addressed (SHA-256) artifact storage with save, query, lineage tracking, and orphan cleanup.

## 2. New Package: `internal/artifact`

### 2.1 Types

```go
type Artifact struct {
    Hash      string `json:"hash"`
    Name      string `json:"name"`
    RunID     string `json:"run_id"`
    NodeID    string `json:"node_id"`
    Size      int64  `json:"size"`
    CreatedAt string `json:"created_at"`
}

type Store struct {
    dir string
}
```

### 2.2 Functions

```go
func NewStore(dir string) *Store
func (s *Store) Save(name string, data []byte, runID, nodeID string) (*Artifact, error)
func (s *Store) Get(hash string) (*Artifact, error)
func (s *Store) Data(hash string) ([]byte, error)
func (s *Store) List() ([]*Artifact, error)
func (s *Store) ListByRun(runID string) ([]*Artifact, error)
func (s *Store) Remove(hash string) error
func (s *Store) FindOrphans(validRunIDs map[string]bool) ([]*Artifact, error)
```

### 2.3 Storage Layout

```
~/.apex/artifacts/
  index.json                    # artifact metadata array
  blobs/
    ab/abcdef1234...            # content blob (first 2 chars as subdir)
```

- Save computes SHA-256, deduplicates (same content stored once), writes blob + updates index.
- Blob path: `blobs/{hash[:2]}/{hash}`

## 3. CLI

```
apex artifact list [--run <run-id>]   # list artifacts
apex artifact info <hash>             # show artifact details
apex artifact gc [--dry-run]          # remove orphan artifacts
```

## 4. Non-Goals

- No remote storage (S3, etc.)
- No artifact version chains
- No artifact content diffing

package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Artifact represents a content-addressed artifact stored by the Store.
type Artifact struct {
	Hash      string `json:"hash"`
	Name      string `json:"name"`
	RunID     string `json:"run_id"`
	NodeID    string `json:"node_id"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
}

// Store provides content-addressed artifact storage backed by the filesystem.
type Store struct {
	dir string
}

// NewStore creates a new Store rooted at dir.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// Save persists data as a named artifact. If content with the same SHA-256 hash
// already exists the existing artifact is returned (deduplication).
func (s *Store) Save(name string, data []byte, runID, nodeID string) (*Artifact, error) {
	hash := sha256sum(data)

	// Deduplication: return existing artifact if hash already indexed.
	index, err := s.loadIndex()
	if err != nil {
		return nil, err
	}
	for _, a := range index {
		if a.Hash == hash {
			return a, nil
		}
	}

	// Write blob.
	bp := s.blobPath(hash)
	if err := os.MkdirAll(filepath.Dir(bp), 0o755); err != nil {
		return nil, fmt.Errorf("artifact: mkdir blob dir: %w", err)
	}
	if err := os.WriteFile(bp, data, 0o644); err != nil {
		return nil, fmt.Errorf("artifact: write blob: %w", err)
	}

	art := &Artifact{
		Hash:      hash,
		Name:      name,
		RunID:     runID,
		NodeID:    nodeID,
		Size:      int64(len(data)),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	index = append(index, art)
	if err := s.saveIndex(index); err != nil {
		return nil, err
	}
	return art, nil
}

// Get returns the artifact with the given hash, or an error if not found.
func (s *Store) Get(hash string) (*Artifact, error) {
	index, err := s.loadIndex()
	if err != nil {
		return nil, err
	}
	for _, a := range index {
		if a.Hash == hash {
			return a, nil
		}
	}
	return nil, fmt.Errorf("artifact: not found: %s", hash)
}

// Data reads and returns the raw blob content for the given hash.
func (s *Store) Data(hash string) ([]byte, error) {
	bp := s.blobPath(hash)
	data, err := os.ReadFile(bp)
	if err != nil {
		return nil, fmt.Errorf("artifact: read blob: %w", err)
	}
	return data, nil
}

// List returns all indexed artifacts. Returns nil (not an empty slice) when the
// store is empty.
func (s *Store) List() ([]*Artifact, error) {
	index, err := s.loadIndex()
	if err != nil {
		return nil, err
	}
	if len(index) == 0 {
		return nil, nil
	}
	return index, nil
}

// ListByRun returns all artifacts belonging to the given runID.
func (s *Store) ListByRun(runID string) ([]*Artifact, error) {
	index, err := s.loadIndex()
	if err != nil {
		return nil, err
	}
	var result []*Artifact
	for _, a := range index {
		if a.RunID == runID {
			result = append(result, a)
		}
	}
	return result, nil
}

// Remove deletes the blob and removes the artifact from the index. Returns an
// error if the hash is not found in the index.
func (s *Store) Remove(hash string) error {
	index, err := s.loadIndex()
	if err != nil {
		return err
	}
	found := -1
	for i, a := range index {
		if a.Hash == hash {
			found = i
			break
		}
	}
	if found == -1 {
		return fmt.Errorf("artifact: not found: %s", hash)
	}

	// Remove blob (best-effort; may already be gone).
	_ = os.Remove(s.blobPath(hash))

	index = append(index[:found], index[found+1:]...)
	return s.saveIndex(index)
}

// FindOrphans returns artifacts whose RunID is not in the provided set of valid
// run IDs.
func (s *Store) FindOrphans(validRunIDs map[string]bool) ([]*Artifact, error) {
	index, err := s.loadIndex()
	if err != nil {
		return nil, err
	}
	var orphans []*Artifact
	for _, a := range index {
		if !validRunIDs[a.RunID] {
			orphans = append(orphans, a)
		}
	}
	return orphans, nil
}

// --- private helpers --------------------------------------------------------

func (s *Store) indexPath() string {
	return filepath.Join(s.dir, "index.json")
}

func (s *Store) blobPath(hash string) string {
	return filepath.Join(s.dir, "blobs", hash[:2], hash)
}

func (s *Store) loadIndex() ([]*Artifact, error) {
	data, err := os.ReadFile(s.indexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("artifact: read index: %w", err)
	}
	var index []*Artifact
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("artifact: unmarshal index: %w", err)
	}
	return index, nil
}

func (s *Store) saveIndex(index []*Artifact) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("artifact: mkdir store dir: %w", err)
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("artifact: marshal index: %w", err)
	}
	return os.WriteFile(s.indexPath(), data, 0o644)
}

func sha256sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// Package manifest provides storage for run execution metadata.
// Each run produces a Manifest that is persisted as JSON inside a
// directory named after the run ID.
package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// NodeResult captures the outcome of a single node (step) in a run.
type NodeResult struct {
	ID       string `json:"id"`
	Task     string `json:"task"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
	ActionID string `json:"action_id,omitempty"`
}

// Manifest holds the complete metadata for one execution run.
type Manifest struct {
	RunID      string       `json:"run_id"`
	Task       string       `json:"task"`
	Timestamp  string       `json:"timestamp"`
	Model      string       `json:"model"`
	Effort     string       `json:"effort"`
	RiskLevel  string       `json:"risk_level"`
	NodeCount  int          `json:"node_count"`
	DurationMs int64        `json:"duration_ms"`
	Outcome    string       `json:"outcome"`
	TraceID         string       `json:"trace_id,omitempty"`
	RollbackQuality string       `json:"rollback_quality,omitempty"`
	Nodes           []NodeResult `json:"nodes"`
}

// Store manages manifest persistence under a root directory.
type Store struct {
	dir string
}

// NewStore creates a Store that reads and writes manifests under dir.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// Save writes a manifest to {dir}/{run_id}/manifest.json.
// It creates intermediate directories as needed.
func (s *Store) Save(m *Manifest) error {
	runDir := filepath.Join(s.dir, m.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(runDir, "manifest.json"), data, 0o644)
}

// Load reads the manifest for the given run ID from disk.
func (s *Store) Load(runID string) (*Manifest, error) {
	path := filepath.Join(s.dir, runID, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Recent returns up to n manifests sorted by Timestamp descending.
// It handles an empty or nonexistent directory gracefully by returning
// an empty slice.
func (s *Store) Recent(n int) ([]*Manifest, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var manifests []*Manifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		m, err := s.Load(entry.Name())
		if err != nil {
			// Skip directories that don't contain a valid manifest.
			continue
		}
		manifests = append(manifests, m)
	}

	// Sort by Timestamp descending (most recent first).
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].Timestamp > manifests[j].Timestamp
	})

	if n > len(manifests) {
		n = len(manifests)
	}
	return manifests[:n], nil
}

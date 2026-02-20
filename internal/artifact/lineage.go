package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Dependency represents that FromHash depends on ToHash.
type Dependency struct {
	FromHash string `json:"from_hash"`
	ToHash   string `json:"to_hash"`
}

// ImpactResult holds the result of an impact analysis.
type ImpactResult struct {
	RootHash string   `json:"root_hash"`
	Affected []string `json:"affected"` // hashes of affected artifacts
	Depth    int      `json:"depth"`    // max BFS depth reached
}

// LineageGraph tracks dependencies between artifacts.
type LineageGraph struct {
	dir  string
	deps []Dependency
}

// NewLineageGraph loads deps.json from dir, or returns an empty graph if the
// file does not exist.
func NewLineageGraph(dir string) (*LineageGraph, error) {
	lg := &LineageGraph{dir: dir}

	data, err := os.ReadFile(lg.depsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return lg, nil
		}
		return nil, fmt.Errorf("lineage: read deps: %w", err)
	}

	if err := json.Unmarshal(data, &lg.deps); err != nil {
		return nil, fmt.Errorf("lineage: unmarshal deps: %w", err)
	}
	return lg, nil
}

// AddDependency appends a dependency edge. If the same (fromHash, toHash) pair
// already exists, the call is a no-op.
func (lg *LineageGraph) AddDependency(fromHash, toHash string) error {
	for _, d := range lg.deps {
		if d.FromHash == fromHash && d.ToHash == toHash {
			return nil // dedup
		}
	}
	lg.deps = append(lg.deps, Dependency{FromHash: fromHash, ToHash: toHash})
	return nil
}

// RemoveDependency removes the matching (fromHash, toHash) pair if found.
func (lg *LineageGraph) RemoveDependency(fromHash, toHash string) {
	for i, d := range lg.deps {
		if d.FromHash == fromHash && d.ToHash == toHash {
			lg.deps = append(lg.deps[:i], lg.deps[i+1:]...)
			return
		}
	}
}

// DirectDeps returns all toHash values where FromHash == hash.
func (lg *LineageGraph) DirectDeps(hash string) []string {
	var result []string
	for _, d := range lg.deps {
		if d.FromHash == hash {
			result = append(result, d.ToHash)
		}
	}
	return result
}

// DirectDependents returns all fromHash values where ToHash == hash.
func (lg *LineageGraph) DirectDependents(hash string) []string {
	var result []string
	for _, d := range lg.deps {
		if d.ToHash == hash {
			result = append(result, d.FromHash)
		}
	}
	return result
}

// Impact performs BFS starting from hash, finding all artifacts that depend on
// it (direct and transitive). Uses a visited set to avoid cycles. Depth is the
// max BFS level reached.
func (lg *LineageGraph) Impact(hash string) *ImpactResult {
	result := &ImpactResult{RootHash: hash}

	visited := map[string]bool{hash: true}
	queue := []string{hash}
	depth := 0

	for len(queue) > 0 {
		nextQueue := []string{}
		for _, current := range queue {
			dependents := lg.DirectDependents(current)
			for _, dep := range dependents {
				if !visited[dep] {
					visited[dep] = true
					result.Affected = append(result.Affected, dep)
					nextQueue = append(nextQueue, dep)
				}
			}
		}
		if len(nextQueue) > 0 {
			depth++
		}
		queue = nextQueue
	}

	result.Depth = depth
	return result
}

// Save writes the current dependency list to deps.json in the graph directory.
func (lg *LineageGraph) Save() error {
	if err := os.MkdirAll(lg.dir, 0o755); err != nil {
		return fmt.Errorf("lineage: mkdir dir: %w", err)
	}
	data, err := json.MarshalIndent(lg.deps, "", "  ")
	if err != nil {
		return fmt.Errorf("lineage: marshal deps: %w", err)
	}
	return os.WriteFile(lg.depsPath(), data, 0o644)
}

// --- private helpers --------------------------------------------------------

func (lg *LineageGraph) depsPath() string {
	return filepath.Join(lg.dir, "deps.json")
}

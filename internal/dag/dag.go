package dag

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Status represents the execution state of a DAG node.
type Status int

const (
	Pending Status = iota
	Running
	Completed
	Failed
)

func (s Status) String() string {
	switch s {
	case Pending:
		return "PENDING"
	case Running:
		return "RUNNING"
	case Completed:
		return "COMPLETED"
	case Failed:
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}

// NodeSpec is the input specification for creating a DAG node.
type NodeSpec struct {
	ID      string   `json:"id"`
	Task    string   `json:"task"`
	Depends []string `json:"depends"`
}

// Node represents a single task in the DAG with its current execution state.
type Node struct {
	ID      string
	Task    string
	Depends []string
	Status  Status
	Result  string
	Error   string
}

// DAG is a directed acyclic graph of task nodes with thread-safe operations.
type DAG struct {
	Nodes map[string]*Node
	mu    sync.Mutex
}

// New creates a new DAG from a list of node specifications.
// It validates that all dependencies exist and that the graph contains no cycles.
// Returns an error if the spec list is empty, contains missing dependencies, or has cycles.
func New(specs []NodeSpec) (*DAG, error) {
	if len(specs) == 0 {
		return nil, fmt.Errorf("cannot create DAG from empty node list")
	}

	nodes := make(map[string]*Node, len(specs))
	for _, s := range specs {
		nodes[s.ID] = &Node{
			ID:      s.ID,
			Task:    s.Task,
			Depends: s.Depends,
			Status:  Pending,
		}
	}

	// Validate that all dependencies reference existing nodes.
	for _, n := range nodes {
		for _, dep := range n.Depends {
			if _, ok := nodes[dep]; !ok {
				return nil, fmt.Errorf("node %q depends on %q which does not exist", n.ID, dep)
			}
		}
	}

	d := &DAG{Nodes: nodes}

	if err := d.detectCycles(); err != nil {
		return nil, err
	}

	return d, nil
}

// detectCycles uses DFS with a tri-color marking scheme to find cycles.
func (d *DAG) detectCycles() error {
	// 0 = unvisited, 1 = visiting (on current stack), 2 = done
	visited := make(map[string]int, len(d.Nodes))

	var dfs func(id string) error
	dfs = func(id string) error {
		visited[id] = 1
		for _, dep := range d.Nodes[id].Depends {
			switch visited[dep] {
			case 1:
				return fmt.Errorf("cycle detected involving %q and %q", id, dep)
			case 0:
				if err := dfs(dep); err != nil {
					return err
				}
			}
		}
		visited[id] = 2
		return nil
	}

	for id := range d.Nodes {
		if visited[id] == 0 {
			if err := dfs(id); err != nil {
				return err
			}
		}
	}
	return nil
}

// ReadyNodes returns all nodes that are in Pending status and have all
// dependencies in Completed status. Thread-safe.
func (d *DAG) ReadyNodes() []*Node {
	d.mu.Lock()
	defer d.mu.Unlock()

	var ready []*Node
	for _, n := range d.Nodes {
		if n.Status != Pending {
			continue
		}
		allDepsComplete := true
		for _, dep := range n.Depends {
			if d.Nodes[dep].Status != Completed {
				allDepsComplete = false
				break
			}
		}
		if allDepsComplete {
			ready = append(ready, n)
		}
	}
	return ready
}

// MarkRunning transitions a node to Running status. Thread-safe.
func (d *DAG) MarkRunning(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if n, ok := d.Nodes[id]; ok {
		n.Status = Running
	}
}

// MarkCompleted transitions a node to Completed status and stores the result. Thread-safe.
func (d *DAG) MarkCompleted(id string, result string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if n, ok := d.Nodes[id]; ok {
		n.Status = Completed
		n.Result = result
	}
}

// MarkFailed transitions a node to Failed status, stores the error message,
// and cascades the failure to all transitive dependents. Thread-safe.
func (d *DAG) MarkFailed(id string, errMsg string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if n, ok := d.Nodes[id]; ok {
		n.Status = Failed
		n.Error = errMsg
	}
	d.cascadeFail(id)
}

// cascadeFail recursively marks all pending nodes that depend (directly or
// transitively) on the failed node as Failed. Must be called with mu held.
func (d *DAG) cascadeFail(failedID string) {
	for _, n := range d.Nodes {
		if n.Status != Pending {
			continue
		}
		for _, dep := range n.Depends {
			if dep == failedID || (d.Nodes[dep] != nil && d.Nodes[dep].Status == Failed) {
				n.Status = Failed
				n.Error = fmt.Sprintf("dependency %q failed", dep)
				d.cascadeFail(n.ID)
				break
			}
		}
	}
}

// IsComplete returns true if all nodes are in a terminal state (Completed or Failed). Thread-safe.
func (d *DAG) IsComplete() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, n := range d.Nodes {
		if n.Status != Completed && n.Status != Failed {
			return false
		}
	}
	return true
}

// HasFailure returns true if any node is in Failed status. Thread-safe.
func (d *DAG) HasFailure() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, n := range d.Nodes {
		if n.Status == Failed {
			return true
		}
	}
	return false
}

// Summary returns a human-readable summary of all node statuses. Thread-safe.
func (d *DAG) Summary() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var lines []string
	for _, n := range d.Nodes {
		line := fmt.Sprintf("  [%s] task: %s", n.Status, n.Task)
		if n.Error != "" {
			line += fmt.Sprintf(" (error: %s)", n.Error)
		}
		if n.Result != "" {
			// Truncate long results for readability.
			result := n.Result
			if len(result) > 500 {
				result = result[:500] + "..."
			}
			line += "\n    → " + strings.ReplaceAll(result, "\n", "\n      ")
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// NodeSlice returns nodes in topological order (dependencies before dependents).
// Thread-safe.
func (d *DAG) NodeSlice() []*Node {
	d.mu.Lock()
	defer d.mu.Unlock()

	visited := make(map[string]bool, len(d.Nodes))
	var order []*Node

	var visit func(id string)
	visit = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true
		n := d.Nodes[id]
		for _, dep := range n.Depends {
			visit(dep)
		}
		order = append(order, n)
	}

	// Sort keys for deterministic output
	ids := make([]string, 0, len(d.Nodes))
	for id := range d.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		visit(id)
	}

	return order
}

// RemoveNode removes a node from the DAG and strips it from all dependency lists.
// Thread-safe. No-op if the node does not exist.
func (d *DAG) RemoveNode(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.Nodes[id]; !ok {
		return
	}
	delete(d.Nodes, id)
	for _, n := range d.Nodes {
		filtered := n.Depends[:0]
		for _, dep := range n.Depends {
			if dep != id {
				filtered = append(filtered, dep)
			}
		}
		n.Depends = filtered
	}
}

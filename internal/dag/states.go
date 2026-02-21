package dag

import "fmt"

// Extended states (additive — does not modify existing iota values)
const (
	Blocked   Status = 4 // waiting on unresolved dependencies
	Suspended Status = 5 // paused by user/system
	Cancelled Status = 6 // cancelled by user/system
	Skipped   Status = 7 // skipped due to dependency cancel/skip
)

// IsTerminal returns true if the status is a terminal state.
func IsTerminal(s Status) bool {
	return s == Completed || s == Failed || s == Cancelled || s == Skipped
}

// MarkBlocked transitions a node from Pending to Blocked.
func (d *DAG) MarkBlocked(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.Nodes[id]
	if !ok {
		return fmt.Errorf("dag: node %q not found", id)
	}
	if n.Status != Pending {
		return fmt.Errorf("dag: cannot block node %q: current status is %s", id, n.Status)
	}
	n.Status = Blocked
	return nil
}

// Unblock transitions a node from Blocked back to Pending.
func (d *DAG) Unblock(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.Nodes[id]
	if !ok {
		return fmt.Errorf("dag: node %q not found", id)
	}
	if n.Status != Blocked {
		return fmt.Errorf("dag: cannot unblock node %q: current status is %s", id, n.Status)
	}
	n.Status = Pending
	return nil
}

// Suspend transitions a node from Pending, Blocked, or Running to Suspended.
func (d *DAG) Suspend(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.Nodes[id]
	if !ok {
		return fmt.Errorf("dag: node %q not found", id)
	}
	if n.Status != Pending && n.Status != Blocked && n.Status != Running {
		return fmt.Errorf("dag: cannot suspend node %q: current status is %s", id, n.Status)
	}
	n.Status = Suspended
	return nil
}

// Resume transitions a node from Suspended to Pending.
func (d *DAG) Resume(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.Nodes[id]
	if !ok {
		return fmt.Errorf("dag: node %q not found", id)
	}
	if n.Status != Suspended {
		return fmt.Errorf("dag: cannot resume node %q: current status is %s", id, n.Status)
	}
	n.Status = Pending
	return nil
}

// Cancel transitions a node from any non-terminal state to Cancelled,
// then cascades skip to all transitive dependents in Pending/Blocked state.
func (d *DAG) Cancel(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.Nodes[id]
	if !ok {
		return fmt.Errorf("dag: node %q not found", id)
	}
	if IsTerminal(n.Status) {
		return fmt.Errorf("dag: cannot cancel node %q: current status is %s", id, n.Status)
	}
	n.Status = Cancelled
	d.cascadeSkip(id)
	return nil
}

// Skip transitions a node from Pending or Blocked to Skipped.
func (d *DAG) Skip(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.Nodes[id]
	if !ok {
		return fmt.Errorf("dag: node %q not found", id)
	}
	if n.Status != Pending && n.Status != Blocked {
		return fmt.Errorf("dag: cannot skip node %q: current status is %s", id, n.Status)
	}
	n.Status = Skipped
	return nil
}

// cascadeSkip recursively marks all Pending/Blocked dependents as Skipped.
// Must be called with mu held.
func (d *DAG) cascadeSkip(cancelledID string) {
	for _, n := range d.Nodes {
		if n.Status != Pending && n.Status != Blocked {
			continue
		}
		for _, dep := range n.Depends {
			if dep == cancelledID || (d.Nodes[dep] != nil && (d.Nodes[dep].Status == Cancelled || d.Nodes[dep].Status == Skipped)) {
				n.Status = Skipped
				n.Error = fmt.Sprintf("dependency %q was cancelled", dep)
				d.cascadeSkip(n.ID)
				break
			}
		}
	}
}

// StatusCounts returns a count of nodes per status name.
func (d *DAG) StatusCounts() map[string]int {
	d.mu.Lock()
	defer d.mu.Unlock()
	counts := make(map[string]int)
	for _, n := range d.Nodes {
		counts[n.Status.String()]++
	}
	return counts
}

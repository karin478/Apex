package dag

import "fmt"

// Extended states (additive — does not modify existing iota values)
const (
	Blocked   Status = 4 // waiting on unresolved dependencies
	Suspended Status = 5 // paused by user/system
	Cancelled Status = 6 // cancelled by user/system
	Skipped   Status = 7 // skipped due to dependency cancel/skip

	// Extended lifecycle states (additive — architecture v11.0 §2.2 F1)
	Ready       Status = 8  // dependencies resolved, waiting for execution slot
	Retrying    Status = 9  // in exponential backoff, waiting for retry
	Resuming    Status = 10 // resuming from Suspended (30s timeout)
	Replanning  Status = 11 // change weight ≥1.5, needs re-plan (60s timeout)
	Invalidated Status = 12 // artifact changed, needs re-execution
	Escalated   Status = 13 // requires human intervention (terminal)
	NeedsHuman  Status = 14 // explicit human approval required (terminal)
)

// IsTerminal returns true if the status is a terminal state.
func IsTerminal(s Status) bool {
	return s == Completed || s == Failed || s == Cancelled || s == Skipped ||
		s == Escalated || s == NeedsHuman
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

// cascadeSkip marks all Pending/Blocked dependents as Skipped.
// Must be called with mu held.
func (d *DAG) cascadeSkip(cancelledID string) {
	visited := make(map[string]bool)
	d.cascadeSkipImpl(cancelledID, visited)
}

func (d *DAG) cascadeSkipImpl(cancelledID string, visited map[string]bool) {
	for _, n := range d.Nodes {
		if visited[n.ID] {
			continue
		}
		if n.Status != Pending && n.Status != Blocked {
			continue
		}
		for _, dep := range n.Depends {
			if dep == cancelledID || (d.Nodes[dep] != nil && (d.Nodes[dep].Status == Cancelled || d.Nodes[dep].Status == Skipped)) {
				n.Status = Skipped
				n.Error = fmt.Sprintf("dependency %q was cancelled", dep)
				visited[n.ID] = true
				d.cascadeSkipImpl(n.ID, visited)
				break
			}
		}
	}
}

// MarkReady transitions a node from Pending or Blocked to Ready.
func (d *DAG) MarkReady(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.Nodes[id]
	if !ok {
		return fmt.Errorf("dag: node %q not found", id)
	}
	if n.Status != Pending && n.Status != Blocked {
		return fmt.Errorf("dag: cannot mark node %q ready: current status is %s", id, n.Status)
	}
	n.Status = Ready
	return nil
}

// MarkRetrying transitions a node from Failed to Retrying.
func (d *DAG) MarkRetrying(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.Nodes[id]
	if !ok {
		return fmt.Errorf("dag: node %q not found", id)
	}
	if n.Status != Failed {
		return fmt.Errorf("dag: cannot retry node %q: current status is %s", id, n.Status)
	}
	n.Status = Retrying
	return nil
}

// MarkResuming transitions a node from Suspended to Resuming.
func (d *DAG) MarkResuming(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.Nodes[id]
	if !ok {
		return fmt.Errorf("dag: node %q not found", id)
	}
	if n.Status != Suspended {
		return fmt.Errorf("dag: cannot resume node %q: current status is %s", id, n.Status)
	}
	n.Status = Resuming
	return nil
}

// MarkReplanning transitions a node from Suspended to Replanning.
func (d *DAG) MarkReplanning(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.Nodes[id]
	if !ok {
		return fmt.Errorf("dag: node %q not found", id)
	}
	if n.Status != Suspended {
		return fmt.Errorf("dag: cannot replan node %q: current status is %s", id, n.Status)
	}
	n.Status = Replanning
	return nil
}

// Invalidate transitions a node from Completed to Invalidated.
func (d *DAG) Invalidate(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.Nodes[id]
	if !ok {
		return fmt.Errorf("dag: node %q not found", id)
	}
	if n.Status != Completed {
		return fmt.Errorf("dag: cannot invalidate node %q: current status is %s", id, n.Status)
	}
	n.Status = Invalidated
	return nil
}

// Requeue transitions a node from Invalidated back to Pending.
func (d *DAG) Requeue(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.Nodes[id]
	if !ok {
		return fmt.Errorf("dag: node %q not found", id)
	}
	if n.Status != Invalidated {
		return fmt.Errorf("dag: cannot requeue node %q: current status is %s", id, n.Status)
	}
	n.Status = Pending
	return nil
}

// Escalate transitions a node from Retrying, Resuming, or Replanning to Escalated.
func (d *DAG) Escalate(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.Nodes[id]
	if !ok {
		return fmt.Errorf("dag: node %q not found", id)
	}
	if n.Status != Retrying && n.Status != Resuming && n.Status != Replanning {
		return fmt.Errorf("dag: cannot escalate node %q: current status is %s", id, n.Status)
	}
	n.Status = Escalated
	return nil
}

// MarkNeedsHuman transitions a node from Failed to NeedsHuman.
func (d *DAG) MarkNeedsHuman(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.Nodes[id]
	if !ok {
		return fmt.Errorf("dag: node %q not found", id)
	}
	if n.Status != Failed {
		return fmt.Errorf("dag: cannot mark node %q needs-human: current status is %s", id, n.Status)
	}
	n.Status = NeedsHuman
	return nil
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

// Package qos provides resource Quality-of-Service with priority-based slot
// management.  A SlotPool holds a fixed number of slots that can be reserved
// per priority level.  Higher-priority consumers may borrow unused reserved
// capacity from lower-priority tiers when the shared (unreserved) pool is
// exhausted.
package qos

import (
	"fmt"
	"sort"
	"sync"
)

// ---------------------------------------------------------------------------
// Priority helpers
// ---------------------------------------------------------------------------

// PriorityValue maps a priority name to a numeric value.  Lower numbers mean
// higher priority.  Unknown names return 99.
func PriorityValue(priority string) int {
	switch priority {
	case "URGENT":
		return 0
	case "HIGH":
		return 1
	case "NORMAL":
		return 2
	case "LOW":
		return 3
	default:
		return 99
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// Reservation describes the number of slots reserved for a given priority.
type Reservation struct {
	Priority string `json:"priority"`
	Reserved int    `json:"reserved"`
}

// PoolUsage is a point-in-time snapshot of a SlotPool.
type PoolUsage struct {
	Total      int            `json:"total"`
	Used       int            `json:"used"`
	Available  int            `json:"available"`
	ByPriority map[string]int `json:"by_priority"`
}

// SlotPool is a concurrency-safe pool of execution slots with per-priority
// reservations and borrowing semantics.
type SlotPool struct {
	mu           sync.Mutex
	total        int
	used         int
	reservations map[string]Reservation
	allocated    map[string]int
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

// NewSlotPool creates a pool with the given total capacity.
func NewSlotPool(total int) *SlotPool {
	return &SlotPool{
		total:        total,
		reservations: make(map[string]Reservation),
		allocated:    make(map[string]int),
	}
}

// ---------------------------------------------------------------------------
// Reservation management
// ---------------------------------------------------------------------------

// AddReservation adds or updates a reservation for the given priority.  It
// returns an error if the sum of all reservations would exceed the pool total.
func (p *SlotPool) AddReservation(r Reservation) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Compute what total reserved would be after this add/update.
	sum := 0
	for pri, existing := range p.reservations {
		if pri == r.Priority {
			continue // will be replaced
		}
		sum += existing.Reserved
	}
	sum += r.Reserved

	if sum > p.total {
		return fmt.Errorf("total reserved (%d) exceeds pool capacity (%d)", sum, p.total)
	}

	p.reservations[r.Priority] = r
	return nil
}

// ---------------------------------------------------------------------------
// Allocation / release
// ---------------------------------------------------------------------------

// Allocate tries to allocate one slot for the given priority.  Returns true on
// success.
//
// Allocation logic:
//  1. If pool is full, return false.
//  2. If the priority has a reservation with unused capacity, consume it.
//  3. Otherwise, if the shared (unreserved) pool has room, use that.
//  4. Otherwise, if the caller's priority is higher (lower numeric value) than
//     the lowest-priority tier that still has unused reserved capacity, borrow
//     from that tier.
//  5. Otherwise, return false.
func (p *SlotPool) Allocate(priority string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.used >= p.total {
		return false
	}

	// Step 2 -- use own reservation.
	if res, ok := p.reservations[priority]; ok {
		if p.allocated[priority] < res.Reserved {
			p.allocated[priority]++
			p.used++
			return true
		}
	}

	// Step 3 -- use unreserved (shared) pool.
	totalReserved := 0
	usedInReserved := 0
	for pri, res := range p.reservations {
		totalReserved += res.Reserved
		alloc := p.allocated[pri]
		if alloc > res.Reserved {
			alloc = res.Reserved
		}
		usedInReserved += alloc
	}

	unreservedTotal := p.total - totalReserved
	unreservedUsed := p.used - usedInReserved

	if unreservedUsed < unreservedTotal {
		p.allocated[priority]++
		p.used++
		return true
	}

	// Step 4 -- borrow from lower-priority unused reservations.
	// Find the lowest-priority tier (highest PriorityValue) that still has
	// unused reserved capacity.
	lowestUnused := -1
	for pri, res := range p.reservations {
		if p.allocated[pri] < res.Reserved {
			pv := PriorityValue(pri)
			if pv > lowestUnused {
				lowestUnused = pv
			}
		}
	}

	if lowestUnused >= 0 && PriorityValue(priority) < lowestUnused {
		p.allocated[priority]++
		p.used++
		return true
	}

	return false
}

// Release releases one slot for the given priority.  It is a no-op if nothing
// is allocated for that priority.
func (p *SlotPool) Release(priority string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.allocated[priority] == 0 {
		return
	}
	p.allocated[priority]--
	p.used--
}

// ---------------------------------------------------------------------------
// Queries
// ---------------------------------------------------------------------------

// Available returns the number of free slots (total - used).
func (p *SlotPool) Available() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.total - p.used
}

// Usage returns a point-in-time snapshot of pool utilisation.
func (p *SlotPool) Usage() PoolUsage {
	p.mu.Lock()
	defer p.mu.Unlock()

	byPriority := make(map[string]int, len(p.allocated))
	for k, v := range p.allocated {
		byPriority[k] = v
	}

	return PoolUsage{
		Total:      p.total,
		Used:       p.used,
		Available:  p.total - p.used,
		ByPriority: byPriority,
	}
}

// Total returns the pool's total capacity.
func (p *SlotPool) Total() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.total
}

// Reservations returns all reservations sorted by PriorityValue (ascending).
func (p *SlotPool) Reservations() []Reservation {
	p.mu.Lock()
	defer p.mu.Unlock()

	out := make([]Reservation, 0, len(p.reservations))
	for _, r := range p.reservations {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		return PriorityValue(out[i].Priority) < PriorityValue(out[j].Priority)
	})
	return out
}

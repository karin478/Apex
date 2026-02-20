// Package event implements an async event runtime with a priority queue and
// type-based handler router. Events are classified into three priority levels
// (urgent, normal, long-running) and dispatched to registered handlers.
package event

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// idCounter is an atomic counter appended to event IDs to guarantee uniqueness
// even when multiple goroutines create events within the same nanosecond.
var idCounter uint64

// Priority represents the urgency level of an event.
type Priority int

const (
	// PriorityURGENT is the highest priority — processed first.
	PriorityURGENT Priority = 0
	// PriorityNORMAL is the default priority.
	PriorityNORMAL Priority = 1
	// PriorityLONG_RUNNING is the lowest priority — processed last.
	PriorityLONG_RUNNING Priority = 2
)

// Event represents a single event flowing through the system.
type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Priority  Priority  `json:"priority"`
	Payload   string    `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}

// Handler is a function that processes an event.
type Handler func(Event) error

// ---------------------------------------------------------------------------
// Queue — priority-based event queue
// ---------------------------------------------------------------------------

// Queue holds events in three priority buckets and pops them in strict
// priority order (urgent → normal → long-running).
type Queue struct {
	urgent      []Event
	normal      []Event
	longRunning []Event
	mu          sync.Mutex
	notify      chan struct{}
}

// QueueStats is a snapshot of the queue's current occupancy.
type QueueStats struct {
	Urgent      int `json:"urgent"`
	Normal      int `json:"normal"`
	LongRunning int `json:"long_running"`
	Total       int `json:"total"`
}

// NewEvent creates an Event with a unique ID and the current timestamp.
// The ID combines the current nanosecond timestamp with an atomic counter
// to avoid collisions under concurrent usage.
func NewEvent(eventType string, priority Priority, payload string) Event {
	now := time.Now()
	seq := atomic.AddUint64(&idCounter, 1)
	return Event{
		ID:        fmt.Sprintf("%d-%d", now.UnixNano(), seq),
		Type:      eventType,
		Priority:  priority,
		Payload:   payload,
		CreatedAt: now,
	}
}

// NewQueue returns an empty, ready-to-use Queue.
func NewQueue() *Queue {
	return &Queue{
		notify: make(chan struct{}, 1),
	}
}

// Push adds an event to the appropriate priority bucket and sends a
// non-blocking signal on the notify channel.
func (q *Queue) Push(e Event) {
	q.mu.Lock()
	switch e.Priority {
	case PriorityURGENT:
		q.urgent = append(q.urgent, e)
	case PriorityNORMAL:
		q.normal = append(q.normal, e)
	case PriorityLONG_RUNNING:
		q.longRunning = append(q.longRunning, e)
	default:
		q.normal = append(q.normal, e)
	}
	q.mu.Unlock()

	// Non-blocking signal so consumers can wake up.
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

// Pop removes and returns the highest-priority event. It checks urgent first,
// then normal, then longRunning. Returns (Event{}, false) when the queue is
// empty.
func (q *Queue) Pop() (Event, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.urgent) > 0 {
		e := q.urgent[0]
		q.urgent = q.urgent[1:]
		return e, true
	}
	if len(q.normal) > 0 {
		e := q.normal[0]
		q.normal = q.normal[1:]
		return e, true
	}
	if len(q.longRunning) > 0 {
		e := q.longRunning[0]
		q.longRunning = q.longRunning[1:]
		return e, true
	}
	return Event{}, false
}

// Len returns the total number of events across all priority buckets.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.urgent) + len(q.normal) + len(q.longRunning)
}

// Stats returns a QueueStats snapshot of the current queue occupancy.
func (q *Queue) Stats() QueueStats {
	q.mu.Lock()
	defer q.mu.Unlock()
	u := len(q.urgent)
	n := len(q.normal)
	l := len(q.longRunning)
	return QueueStats{
		Urgent:      u,
		Normal:      n,
		LongRunning: l,
		Total:       u + n + l,
	}
}

// ---------------------------------------------------------------------------
// Router — type-based event handler registry
// ---------------------------------------------------------------------------

// Router dispatches events to registered handlers based on event type.
type Router struct {
	handlers map[string][]Handler
	mu       sync.RWMutex
}

// NewRouter creates an empty Router ready for handler registration.
func NewRouter() *Router {
	return &Router{
		handlers: make(map[string][]Handler),
	}
}

// Register appends a handler for the given event type. Multiple handlers
// can be registered for the same type and will be called in registration
// order.
func (r *Router) Register(eventType string, h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[eventType] = append(r.handlers[eventType], h)
}

// Dispatch calls all registered handlers for the event's Type in order.
// If any handler returns an error, Dispatch stops and returns that error.
// If no handlers are registered for the type, Dispatch returns nil.
func (r *Router) Dispatch(e Event) error {
	r.mu.RLock()
	hs := r.handlers[e.Type]
	r.mu.RUnlock()

	for _, h := range hs {
		if err := h(e); err != nil {
			return err
		}
	}
	return nil
}

// Types returns a sorted list of all event types that have registered
// handlers.
func (r *Router) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.handlers))
	for t := range r.handlers {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

package event

import (
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// TestNewEvent — Verify ID (non-empty), Type, Priority, Payload, CreatedAt
// ---------------------------------------------------------------------------

func TestNewEvent(t *testing.T) {
	before := time.Now()
	e := NewEvent("task.created", PriorityNORMAL, `{"key":"value"}`)
	after := time.Now()

	assert.NotEmpty(t, e.ID, "ID must not be empty")
	assert.Equal(t, "task.created", e.Type)
	assert.Equal(t, PriorityNORMAL, e.Priority)
	assert.Equal(t, `{"key":"value"}`, e.Payload)
	assert.False(t, e.CreatedAt.IsZero(), "CreatedAt must not be zero")
	assert.True(t, !e.CreatedAt.Before(before) && !e.CreatedAt.After(after),
		"CreatedAt should be between before and after")
}

// ---------------------------------------------------------------------------
// TestQueuePushPop — Push mixed priorities, Pop returns strict priority order
// ---------------------------------------------------------------------------

func TestQueuePushPop(t *testing.T) {
	q := NewQueue()

	// Push in mixed order: normal, long_running, urgent, normal, urgent
	q.Push(NewEvent("a", PriorityNORMAL, "n1"))
	q.Push(NewEvent("b", PriorityLONG_RUNNING, "l1"))
	q.Push(NewEvent("c", PriorityURGENT, "u1"))
	q.Push(NewEvent("d", PriorityNORMAL, "n2"))
	q.Push(NewEvent("e", PriorityURGENT, "u2"))

	// Expected pop order: u1, u2 (urgent), n1, n2 (normal), l1 (long_running)
	expected := []string{"u1", "u2", "n1", "n2", "l1"}
	for _, want := range expected {
		e, ok := q.Pop()
		assert.True(t, ok, "Pop should return true for payload %s", want)
		assert.Equal(t, want, e.Payload, "expected payload %s", want)
	}

	// Queue should now be empty
	_, ok := q.Pop()
	assert.False(t, ok, "Pop on empty queue should return false")
}

// ---------------------------------------------------------------------------
// TestQueueEmpty — Empty queue Pop returns (zero Event, false)
// ---------------------------------------------------------------------------

func TestQueueEmpty(t *testing.T) {
	q := NewQueue()

	e, ok := q.Pop()
	assert.False(t, ok, "Pop on empty queue must return false")
	assert.Equal(t, Event{}, e, "Pop on empty queue must return zero Event")
}

// ---------------------------------------------------------------------------
// TestQueueStats — Push events of different priorities, Stats() returns counts
// ---------------------------------------------------------------------------

func TestQueueStats(t *testing.T) {
	q := NewQueue()

	q.Push(NewEvent("x", PriorityURGENT, ""))
	q.Push(NewEvent("x", PriorityURGENT, ""))
	q.Push(NewEvent("x", PriorityNORMAL, ""))
	q.Push(NewEvent("x", PriorityNORMAL, ""))
	q.Push(NewEvent("x", PriorityNORMAL, ""))
	q.Push(NewEvent("x", PriorityLONG_RUNNING, ""))

	stats := q.Stats()
	assert.Equal(t, 2, stats.Urgent)
	assert.Equal(t, 3, stats.Normal)
	assert.Equal(t, 1, stats.LongRunning)
	assert.Equal(t, 6, stats.Total)

	// Pop one urgent and verify stats update
	q.Pop()
	stats = q.Stats()
	assert.Equal(t, 1, stats.Urgent)
	assert.Equal(t, 5, stats.Total)
}

// ---------------------------------------------------------------------------
// TestRouterRegisterDispatch — Register handler, dispatch, verify called
// ---------------------------------------------------------------------------

func TestRouterRegisterDispatch(t *testing.T) {
	r := NewRouter()

	var received Event
	var called int32

	r.Register("task.created", func(e Event) error {
		atomic.AddInt32(&called, 1)
		received = e
		return nil
	})

	evt := NewEvent("task.created", PriorityNORMAL, "hello")
	err := r.Dispatch(evt)

	assert.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&called), "handler should be called exactly once")
	assert.Equal(t, evt.ID, received.ID)
	assert.Equal(t, evt.Payload, received.Payload)
}

// ---------------------------------------------------------------------------
// TestRouterMultipleHandlers — Register 2 handlers for same type, both called
// ---------------------------------------------------------------------------

func TestRouterMultipleHandlers(t *testing.T) {
	r := NewRouter()

	var count int32

	r.Register("task.done", func(e Event) error {
		atomic.AddInt32(&count, 1)
		return nil
	})
	r.Register("task.done", func(e Event) error {
		atomic.AddInt32(&count, 1)
		return nil
	})

	evt := NewEvent("task.done", PriorityURGENT, "")
	err := r.Dispatch(evt)

	assert.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&count), "both handlers should be called")
}

// ---------------------------------------------------------------------------
// TestRouterUnknownType — Dispatch unregistered type returns nil (no-op)
// ---------------------------------------------------------------------------

func TestRouterUnknownType(t *testing.T) {
	r := NewRouter()

	evt := NewEvent("unknown.type", PriorityNORMAL, "data")
	err := r.Dispatch(evt)

	assert.NoError(t, err, "dispatching unknown type should return nil")

	// Also verify Types() returns sorted list
	r.Register("beta", func(e Event) error { return nil })
	r.Register("alpha", func(e Event) error { return nil })
	r.Register("gamma", func(e Event) error { return nil })

	types := r.Types()
	assert.True(t, sort.StringsAreSorted(types), "Types() should return sorted list")
	assert.Equal(t, []string{"alpha", "beta", "gamma"}, types)
}

package dag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusString(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{Pending, "PENDING"},
		{Running, "RUNNING"},
		{Completed, "COMPLETED"},
		{Failed, "FAILED"},
		{Blocked, "BLOCKED"},
		{Suspended, "SUSPENDED"},
		{Cancelled, "CANCELLED"},
		{Skipped, "SKIPPED"},
		{Status(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.status.String(), "Status(%d).String()", tt.status)
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := []Status{Completed, Failed, Cancelled, Skipped}
	for _, s := range terminal {
		assert.True(t, IsTerminal(s), "%s should be terminal", s)
	}

	nonTerminal := []Status{Pending, Running, Blocked, Suspended}
	for _, s := range nonTerminal {
		assert.False(t, IsTerminal(s), "%s should not be terminal", s)
	}
}

func TestMarkBlockedUnblock(t *testing.T) {
	d, err := New([]NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
	})
	require.NoError(t, err)

	// Pending -> Blocked
	err = d.MarkBlocked("a")
	require.NoError(t, err)
	assert.Equal(t, Blocked, d.Nodes["a"].Status)

	// Blocked -> Pending (unblock)
	err = d.Unblock("a")
	require.NoError(t, err)
	assert.Equal(t, Pending, d.Nodes["a"].Status)

	// Error: MarkBlocked on non-Pending (set to Running first)
	d.MarkRunning("a")
	err = d.MarkBlocked("a")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot block")

	// Error: Unblock on non-Blocked
	err = d.Unblock("a")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unblock")

	// Error: node not found
	err = d.MarkBlocked("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	err = d.Unblock("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSuspendResume(t *testing.T) {
	d, err := New([]NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{}},
		{ID: "c", Task: "task c", Depends: []string{}},
	})
	require.NoError(t, err)

	// Pending -> Suspended -> Pending
	err = d.Suspend("a")
	require.NoError(t, err)
	assert.Equal(t, Suspended, d.Nodes["a"].Status)

	err = d.Resume("a")
	require.NoError(t, err)
	assert.Equal(t, Pending, d.Nodes["a"].Status)

	// Blocked -> Suspended
	err = d.MarkBlocked("b")
	require.NoError(t, err)
	err = d.Suspend("b")
	require.NoError(t, err)
	assert.Equal(t, Suspended, d.Nodes["b"].Status)

	// Running -> Suspended
	d.MarkRunning("c")
	err = d.Suspend("c")
	require.NoError(t, err)
	assert.Equal(t, Suspended, d.Nodes["c"].Status)

	// Error: Suspend on terminal state
	d.Nodes["a"].Status = Completed // force terminal for testing
	err = d.Suspend("a")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot suspend")

	// Error: Resume on non-Suspended
	err = d.Resume("a")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot resume")

	// Error: node not found
	err = d.Suspend("nonexistent")
	assert.Error(t, err)
	err = d.Resume("nonexistent")
	assert.Error(t, err)
}

func TestCancel(t *testing.T) {
	d, err := New([]NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{}},
	})
	require.NoError(t, err)

	// Non-terminal -> Cancelled
	err = d.Cancel("a")
	require.NoError(t, err)
	assert.Equal(t, Cancelled, d.Nodes["a"].Status)

	// Error: already terminal (Cancelled)
	err = d.Cancel("a")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot cancel")

	// Error: node not found
	err = d.Cancel("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCancelCascade(t *testing.T) {
	// A -> B -> C chain
	d, err := New([]NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{"a"}},
		{ID: "c", Task: "task c", Depends: []string{"b"}},
	})
	require.NoError(t, err)

	// Cancel A should cascade skip to B and C
	err = d.Cancel("a")
	require.NoError(t, err)

	assert.Equal(t, Cancelled, d.Nodes["a"].Status)
	assert.Equal(t, Skipped, d.Nodes["b"].Status)
	assert.Equal(t, Skipped, d.Nodes["c"].Status)

	// Verify error messages on cascaded nodes
	assert.Contains(t, d.Nodes["b"].Error, "cancelled")
	assert.Contains(t, d.Nodes["c"].Error, "cancelled")

	// Verify IsComplete returns true (all terminal)
	assert.True(t, d.IsComplete())
}

func TestStatusCounts(t *testing.T) {
	d, err := New([]NodeSpec{
		{ID: "a", Task: "task a", Depends: []string{}},
		{ID: "b", Task: "task b", Depends: []string{}},
		{ID: "c", Task: "task c", Depends: []string{}},
		{ID: "d", Task: "task d", Depends: []string{}},
		{ID: "e", Task: "task e", Depends: []string{}},
	})
	require.NoError(t, err)

	// Set up mixed states
	d.MarkRunning("b")
	d.MarkCompleted("b", "done")

	d.MarkRunning("c")
	d.MarkFailed("c", "error")

	err = d.MarkBlocked("d")
	require.NoError(t, err)

	err = d.Suspend("e")
	require.NoError(t, err)

	counts := d.StatusCounts()

	assert.Equal(t, 1, counts["PENDING"])
	assert.Equal(t, 1, counts["COMPLETED"])
	assert.Equal(t, 1, counts["FAILED"])
	assert.Equal(t, 1, counts["BLOCKED"])
	assert.Equal(t, 1, counts["SUSPENDED"])
}

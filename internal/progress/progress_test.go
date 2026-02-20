package progress

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTracker(t *testing.T) {
	tr := NewTracker()
	list := tr.List()
	assert.Empty(t, list)
}

func TestTrackerStart(t *testing.T) {
	tr := NewTracker()
	report := tr.Start("task-1", "initialization")

	assert.Equal(t, "task-1", report.TaskID)
	assert.Equal(t, "initialization", report.Phase)
	assert.Equal(t, 0, report.Percent)
	assert.Equal(t, StatusRunning, report.Status)
	assert.False(t, report.StartedAt.IsZero())
	assert.False(t, report.UpdatedAt.IsZero())
}

func TestTrackerUpdate(t *testing.T) {
	tr := NewTracker()
	tr.Start("task-1", "build")

	t.Run("normal update", func(t *testing.T) {
		err := tr.Update("task-1", 50, "halfway done")
		require.NoError(t, err)

		report, err := tr.Get("task-1")
		require.NoError(t, err)
		assert.Equal(t, 50, report.Percent)
		assert.Equal(t, "halfway done", report.Message)
	})

	t.Run("clamp above 100", func(t *testing.T) {
		err := tr.Update("task-1", 150, "over")
		require.NoError(t, err)

		report, err := tr.Get("task-1")
		require.NoError(t, err)
		assert.Equal(t, 100, report.Percent)
	})

	t.Run("clamp below 0", func(t *testing.T) {
		err := tr.Update("task-1", -10, "under")
		require.NoError(t, err)

		report, err := tr.Get("task-1")
		require.NoError(t, err)
		assert.Equal(t, 0, report.Percent)
	})
}

func TestTrackerUpdateNotFound(t *testing.T) {
	tr := NewTracker()
	err := tr.Update("nonexistent", 50, "test")
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestTrackerComplete(t *testing.T) {
	tr := NewTracker()
	tr.Start("task-1", "deploy")
	err := tr.Update("task-1", 75, "almost")
	require.NoError(t, err)

	err = tr.Complete("task-1", "done successfully")
	require.NoError(t, err)

	report, err := tr.Get("task-1")
	require.NoError(t, err)
	assert.Equal(t, 100, report.Percent)
	assert.Equal(t, StatusCompleted, report.Status)
	assert.Equal(t, "done successfully", report.Message)
}

func TestTrackerFail(t *testing.T) {
	tr := NewTracker()
	tr.Start("task-1", "deploy")
	err := tr.Update("task-1", 40, "in progress")
	require.NoError(t, err)

	err = tr.Fail("task-1", "connection lost")
	require.NoError(t, err)

	report, err := tr.Get("task-1")
	require.NoError(t, err)
	assert.Equal(t, 40, report.Percent)
	assert.Equal(t, StatusFailed, report.Status)
	assert.Equal(t, "connection lost", report.Message)
}

func TestTrackerList(t *testing.T) {
	tr := NewTracker()
	tr.Start("task-a", "phase-a")
	time.Sleep(10 * time.Millisecond)
	tr.Start("task-b", "phase-b")
	time.Sleep(10 * time.Millisecond)
	err := tr.Update("task-a", 50, "updated last")
	require.NoError(t, err)

	list := tr.List()
	assert.Len(t, list, 2)
	// task-a was updated most recently, should be first.
	assert.Equal(t, "task-a", list[0].TaskID)
	assert.Equal(t, "task-b", list[1].TaskID)
}

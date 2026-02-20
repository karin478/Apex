// Package progress provides structured task progress tracking and reporting.
package progress

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ErrTaskNotFound is returned when a task is not being tracked.
var ErrTaskNotFound = errors.New("progress: task not found")

// Status represents the state of a tracked task.
type Status string

const (
	StatusRunning   Status = "RUNNING"
	StatusCompleted Status = "COMPLETED"
	StatusFailed    Status = "FAILED"
)

// ProgressReport holds the progress state for a single task.
type ProgressReport struct {
	TaskID    string    `json:"task_id"`
	Phase     string    `json:"phase"`
	Percent   int       `json:"percent"`
	Message   string    `json:"message"`
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Status    Status    `json:"status"`
}

// Tracker manages progress reports for multiple tasks.
type Tracker struct {
	mu      sync.RWMutex
	reports map[string]*ProgressReport
}

// NewTracker creates an empty Tracker.
func NewTracker() *Tracker {
	return &Tracker{reports: make(map[string]*ProgressReport)}
}

// Start begins tracking a task. Overwrites if the task already exists.
func (t *Tracker) Start(taskID, phase string) *ProgressReport {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	report := &ProgressReport{
		TaskID:    taskID,
		Phase:     phase,
		Percent:   0,
		Status:    StatusRunning,
		StartedAt: now,
		UpdatedAt: now,
	}
	t.reports[taskID] = report
	return report
}

// Update updates the progress percentage and message for a task.
// Percent is clamped to 0-100.
func (t *Tracker) Update(taskID string, percent int, message string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	report, ok := t.reports[taskID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}

	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	report.Percent = percent
	report.Message = message
	report.UpdatedAt = time.Now()
	return nil
}

// Complete marks a task as completed with Percent=100.
func (t *Tracker) Complete(taskID, message string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	report, ok := t.reports[taskID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}

	report.Percent = 100
	report.Status = StatusCompleted
	report.Message = message
	report.UpdatedAt = time.Now()
	return nil
}

// Fail marks a task as failed, keeping the current percent.
func (t *Tracker) Fail(taskID, message string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	report, ok := t.reports[taskID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}

	report.Status = StatusFailed
	report.Message = message
	report.UpdatedAt = time.Now()
	return nil
}

// Get returns a copy of the progress report for a task.
func (t *Tracker) Get(taskID string) (ProgressReport, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	report, ok := t.reports[taskID]
	if !ok {
		return ProgressReport{}, fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}
	return *report, nil
}

// List returns all reports sorted by UpdatedAt descending.
func (t *Tracker) List() []ProgressReport {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]ProgressReport, 0, len(t.reports))
	for _, r := range t.reports {
		result = append(result, *r)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result
}

package analytics

import (
	"testing"
	"time"

	"github.com/lyndonlyu/apex/internal/statedb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeRun builds a RunRecord for testing. startOffset shifts the base
// start time (2026-01-01T00:00:00Z) by the given number of seconds.
// For COMPLETED and FAILED runs, EndedAt is set to start + durationSecs.
func makeRun(id, status string, startOffset, durationSecs int) statedb.RunRecord {
	start := time.Date(2026, 1, 1, 0, 0, startOffset, 0, time.UTC)
	r := statedb.RunRecord{
		ID:        id,
		Status:    status,
		TaskCount: 3,
		StartedAt: start.Format(time.RFC3339),
	}
	if status == "COMPLETED" || status == "FAILED" {
		r.EndedAt = start.Add(time.Duration(durationSecs) * time.Second).Format(time.RFC3339)
	}
	return r
}

func TestSummarizeEmpty(t *testing.T) {
	s := Summarize(nil)
	assert.Equal(t, 0, s.TotalRuns)
	assert.Empty(t, s.ByStatus)
	assert.Equal(t, 0.0, s.SuccessRate)
	assert.Equal(t, 0.0, s.FailureRate)
}

func TestSummarize(t *testing.T) {
	runs := []statedb.RunRecord{
		makeRun("r1", "COMPLETED", 0, 10),
		makeRun("r2", "COMPLETED", 10, 20),
		makeRun("r3", "COMPLETED", 30, 60),
		makeRun("r4", "FAILED", 90, 5),
		makeRun("r5", "FAILED", 100, 3),
	}

	s := Summarize(runs)
	assert.Equal(t, 5, s.TotalRuns)
	assert.Equal(t, 3, s.ByStatus["COMPLETED"])
	assert.Equal(t, 2, s.ByStatus["FAILED"])
	assert.InDelta(t, 0.6, s.SuccessRate, 1e-9)
	assert.InDelta(t, 0.4, s.FailureRate, 1e-9)
}

func TestComputeDurationEmpty(t *testing.T) {
	// No completed runs at all.
	runs := []statedb.RunRecord{
		makeRun("r1", "FAILED", 0, 10),
		makeRun("r2", "PENDING", 10, 0),
	}

	d := ComputeDuration(runs)
	assert.Equal(t, 0, d.Count)
	assert.Equal(t, 0.0, d.MinSecs)
	assert.Equal(t, 0.0, d.MaxSecs)
	assert.Equal(t, 0.0, d.AvgSecs)
	assert.Equal(t, 0.0, d.P50Secs)
	assert.Equal(t, 0.0, d.P90Secs)
}

func TestComputeDuration(t *testing.T) {
	runs := []statedb.RunRecord{
		makeRun("r1", "COMPLETED", 0, 10),
		makeRun("r2", "COMPLETED", 10, 20),
		makeRun("r3", "COMPLETED", 30, 60),
	}

	d := ComputeDuration(runs)
	assert.Equal(t, 3, d.Count)
	assert.InDelta(t, 10.0, d.MinSecs, 1e-9)
	assert.InDelta(t, 60.0, d.MaxSecs, 1e-9)
	assert.InDelta(t, 30.0, d.AvgSecs, 1e-9)
	assert.InDelta(t, 20.0, d.P50Secs, 1e-9)
	assert.InDelta(t, 60.0, d.P90Secs, 1e-9)
}

func TestDetectFailures(t *testing.T) {
	runs := []statedb.RunRecord{
		makeRun("r1", "COMPLETED", 0, 10),
		makeRun("r2", "COMPLETED", 10, 20),
		makeRun("r3", "COMPLETED", 30, 60),
		makeRun("r4", "FAILED", 90, 5),
		makeRun("r5", "FAILED", 100, 3),
	}

	failures := DetectFailures(runs)
	require.Len(t, failures, 1)
	assert.Equal(t, "FAILED", failures[0].Status)
	assert.Equal(t, 2, failures[0].Count)
	assert.InDelta(t, 0.4, failures[0].Rate, 1e-9)
}

func TestGenerateReport(t *testing.T) {
	runs := []statedb.RunRecord{
		makeRun("r1", "COMPLETED", 0, 10),
		makeRun("r2", "COMPLETED", 10, 20),
		makeRun("r3", "COMPLETED", 30, 60),
		makeRun("r4", "FAILED", 90, 5),
		makeRun("r5", "FAILED", 100, 3),
	}

	report := GenerateReport(runs)

	// Summary
	assert.Equal(t, 5, report.Summary.TotalRuns)
	assert.InDelta(t, 0.6, report.Summary.SuccessRate, 1e-9)
	assert.InDelta(t, 0.4, report.Summary.FailureRate, 1e-9)

	// Duration (only 3 COMPLETED runs with durations 10, 20, 60)
	assert.Equal(t, 3, report.Duration.Count)
	assert.InDelta(t, 10.0, report.Duration.MinSecs, 1e-9)
	assert.InDelta(t, 60.0, report.Duration.MaxSecs, 1e-9)
	assert.InDelta(t, 30.0, report.Duration.AvgSecs, 1e-9)

	// Failures
	require.NotEmpty(t, report.Failures)
	assert.Equal(t, 2, report.Failures[0].Count)

	// Generated timestamp is non-empty and parseable
	assert.NotEmpty(t, report.Generated)
	_, err := time.Parse(time.RFC3339, report.Generated)
	assert.NoError(t, err)
}

func TestComputeDurationSkipsInvalid(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	runs := []statedb.RunRecord{
		// Valid COMPLETED run: 10s duration
		makeRun("r1", "COMPLETED", 0, 10),
		// COMPLETED with empty EndedAt — should be skipped
		{
			ID:        "r2",
			Status:    "COMPLETED",
			TaskCount: 3,
			StartedAt: base.Format(time.RFC3339),
			EndedAt:   "",
		},
		// COMPLETED with bad EndedAt — should be skipped
		{
			ID:        "r3",
			Status:    "COMPLETED",
			TaskCount: 3,
			StartedAt: base.Format(time.RFC3339),
			EndedAt:   "not-a-timestamp",
		},
		// COMPLETED with bad StartedAt — should be skipped
		{
			ID:        "r4",
			Status:    "COMPLETED",
			TaskCount: 3,
			StartedAt: "also-bad",
			EndedAt:   base.Add(20 * time.Second).Format(time.RFC3339),
		},
	}

	d := ComputeDuration(runs)
	// Only r1 should be counted.
	assert.Equal(t, 1, d.Count)
	assert.InDelta(t, 10.0, d.MinSecs, 1e-9)
	assert.InDelta(t, 10.0, d.MaxSecs, 1e-9)
	assert.InDelta(t, 10.0, d.AvgSecs, 1e-9)
	assert.InDelta(t, 10.0, d.P50Secs, 1e-9)
	assert.InDelta(t, 10.0, d.P90Secs, 1e-9)
}

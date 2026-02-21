package analytics

import (
	"math"
	"sort"
	"time"

	"github.com/lyndonlyu/apex/internal/statedb"
)

// RunSummary holds aggregate counts and rates for a set of runs.
type RunSummary struct {
	TotalRuns   int            `json:"total_runs"`
	ByStatus    map[string]int `json:"by_status"`
	SuccessRate float64        `json:"success_rate"`
	FailureRate float64        `json:"failure_rate"`
}

// DurationStats holds duration statistics for completed runs.
type DurationStats struct {
	Count   int     `json:"count"`
	MinSecs float64 `json:"min_secs"`
	MaxSecs float64 `json:"max_secs"`
	AvgSecs float64 `json:"avg_secs"`
	P50Secs float64 `json:"p50_secs"`
	P90Secs float64 `json:"p90_secs"`
}

// FailurePattern represents a failure status with its count and rate.
type FailurePattern struct {
	Status string  `json:"status"`
	Count  int     `json:"count"`
	Rate   float64 `json:"rate"`
}

// Report combines summary, duration, and failure analytics into a
// single structure.
type Report struct {
	Summary   RunSummary       `json:"summary"`
	Duration  DurationStats    `json:"duration"`
	Failures  []FailurePattern `json:"failures"`
	Generated string           `json:"generated"`
}

// Summarize counts runs by status and computes success/failure rates.
// An empty input produces TotalRuns=0 with both rates at 0.0.
func Summarize(runs []statedb.RunRecord) RunSummary {
	s := RunSummary{
		ByStatus: make(map[string]int),
	}
	s.TotalRuns = len(runs)
	for _, r := range runs {
		s.ByStatus[r.Status]++
	}
	if s.TotalRuns == 0 {
		return s
	}
	s.SuccessRate = float64(s.ByStatus["COMPLETED"]) / float64(s.TotalRuns)
	s.FailureRate = float64(s.ByStatus["FAILED"]) / float64(s.TotalRuns)
	return s
}

// ComputeDuration calculates duration statistics for COMPLETED runs
// that have a valid, non-empty EndedAt timestamp. Runs with empty or
// unparseable timestamps are skipped. If no valid durations exist, all
// fields are zero.
func ComputeDuration(runs []statedb.RunRecord) DurationStats {
	var durations []float64
	for _, r := range runs {
		if r.Status != "COMPLETED" {
			continue
		}
		if r.EndedAt == "" {
			continue
		}
		start, err := time.Parse(time.RFC3339, r.StartedAt)
		if err != nil {
			continue
		}
		end, err := time.Parse(time.RFC3339, r.EndedAt)
		if err != nil {
			continue
		}
		dur := end.Sub(start).Seconds()
		if dur <= 0 {
			continue
		}
		durations = append(durations, dur)
	}

	if len(durations) == 0 {
		return DurationStats{}
	}

	sort.Float64s(durations)

	var sum float64
	min := durations[0]
	max := durations[0]
	for _, d := range durations {
		sum += d
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}

	return DurationStats{
		Count:   len(durations),
		MinSecs: min,
		MaxSecs: max,
		AvgSecs: sum / float64(len(durations)),
		P50Secs: percentile(durations, 0.5),
		P90Secs: percentile(durations, 0.9),
	}
}

// percentile returns the value at the given percentile p (0.0-1.0)
// from a pre-sorted slice. Returns 0 for an empty slice.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Round(float64(len(sorted)-1) * p))
	return sorted[idx]
}

// DetectFailures groups FAILED runs and computes count and rate
// (count / total) for each failure status.
func DetectFailures(runs []statedb.RunRecord) []FailurePattern {
	total := len(runs)
	if total == 0 {
		return nil
	}

	failCount := 0
	for _, r := range runs {
		if r.Status == "FAILED" {
			failCount++
		}
	}

	if failCount == 0 {
		return nil
	}

	return []FailurePattern{
		{
			Status: "FAILED",
			Count:  failCount,
			Rate:   float64(failCount) / float64(total),
		},
	}
}

// GenerateReport builds a complete analytics report by combining
// summary, duration, and failure data. The Generated field is set to
// the current UTC time in RFC3339 format.
func GenerateReport(runs []statedb.RunRecord) Report {
	return Report{
		Summary:   Summarize(runs),
		Duration:  ComputeDuration(runs),
		Failures:  DetectFailures(runs),
		Generated: time.Now().UTC().Format(time.RFC3339),
	}
}

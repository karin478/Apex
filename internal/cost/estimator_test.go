package cost

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimateRun(t *testing.T) {
	tasks := map[string]string{
		"1": "Write a function that parses JSON input and returns structured data",
		"2": "Add unit tests for the parser",
	}
	est := EstimateRun(tasks, "claude-sonnet-4-20250514")
	assert.Greater(t, est.InputTokens, 0)
	assert.Greater(t, est.OutputTokens, 0)
	assert.Greater(t, est.TotalCost, 0.0)
	assert.Equal(t, "claude-sonnet-4-20250514", est.Model)
	assert.Equal(t, 2, est.NodeCount)
}

func TestEstimateRunOpus(t *testing.T) {
	tasks := map[string]string{
		"1": "some task text here",
	}
	sonnet := EstimateRun(tasks, "claude-sonnet-4-20250514")
	opus := EstimateRun(tasks, "claude-opus-4-20250514")
	// Opus should cost more than sonnet for same input
	assert.Greater(t, opus.TotalCost, sonnet.TotalCost)
}

func TestEstimateRunEmpty(t *testing.T) {
	est := EstimateRun(map[string]string{}, "claude-sonnet-4-20250514")
	assert.Equal(t, 0, est.InputTokens)
	assert.Equal(t, 0.0, est.TotalCost)
	assert.Equal(t, 0, est.NodeCount)
}

func TestFormatCost(t *testing.T) {
	assert.Equal(t, "~$0.01", FormatCost(0.005))
	assert.Equal(t, "~$0.12", FormatCost(0.123))
	assert.Equal(t, "<$0.01", FormatCost(0.001))
}

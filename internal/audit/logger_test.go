package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)
	assert.NotNil(t, logger)
	assert.DirExists(t, dir)
}

func TestLogEntry(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	entry := Entry{
		Task:      "test task",
		RiskLevel: "LOW",
		Outcome:   "success",
		Duration:  100 * time.Millisecond,
		Model:     "claude-opus-4-6",
	}

	err = logger.Log(entry)
	require.NoError(t, err)

	// Verify file was created with today's date
	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(dir, today+".jsonl")
	assert.FileExists(t, logFile)

	// Read and parse the entry
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	var record Record
	require.NoError(t, json.Unmarshal(data, &record))

	assert.Equal(t, "test task", record.Task)
	assert.Equal(t, "LOW", record.RiskLevel)
	assert.Equal(t, "success", record.Outcome)
	assert.NotEmpty(t, record.ActionID)
	assert.NotEmpty(t, record.Timestamp)
	assert.Equal(t, "claude-opus-4-6", record.Model)
	assert.Equal(t, int64(100), record.DurationMs)
}

func TestLogMultipleEntries(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		err = logger.Log(Entry{
			Task:      "task",
			RiskLevel: "LOW",
			Outcome:   "success",
			Duration:  time.Second,
			Model:     "claude-opus-4-6",
		})
		require.NoError(t, err)
	}

	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(dir, today+".jsonl")
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 3)
}

func TestRecentEntries(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		err = logger.Log(Entry{
			Task:      "task",
			RiskLevel: "LOW",
			Outcome:   "success",
			Duration:  time.Second,
			Model:     "claude-opus-4-6",
		})
		require.NoError(t, err)
	}

	entries, err := logger.Recent(3)
	require.NoError(t, err)
	assert.Len(t, entries, 3)
}

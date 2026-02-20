package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lyndonlyu/apex/internal/redact"
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

func TestHashChain(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		logger.Log(Entry{Task: fmt.Sprintf("task-%d", i), RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	}

	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(dir, today+".jsonl")
	data, _ := os.ReadFile(logFile)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	var records []Record
	for _, line := range lines {
		var r Record
		json.Unmarshal([]byte(line), &r)
		records = append(records, r)
	}

	assert.Empty(t, records[0].PrevHash)
	assert.NotEmpty(t, records[0].Hash)
	assert.Equal(t, records[0].Hash, records[1].PrevHash)
	assert.Equal(t, records[1].Hash, records[2].PrevHash)
}

func TestVerifyChainValid(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)
	for i := 0; i < 5; i++ {
		logger.Log(Entry{Task: fmt.Sprintf("task-%d", i), RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	}
	valid, brokenAt, err := logger.Verify()
	require.NoError(t, err)
	assert.True(t, valid)
	assert.Equal(t, -1, brokenAt)
}

func TestVerifyChainBroken(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)
	for i := 0; i < 3; i++ {
		logger.Log(Entry{Task: fmt.Sprintf("task-%d", i), RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	}
	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(dir, today+".jsonl")
	data, _ := os.ReadFile(logFile)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var r Record
	json.Unmarshal([]byte(lines[1]), &r)
	r.Task = "TAMPERED"
	tampered, _ := json.Marshal(r)
	lines[1] = string(tampered)
	os.WriteFile(logFile, []byte(strings.Join(lines, "\n")+"\n"), 0644)
	logger2, _ := NewLogger(dir)
	valid, brokenAt, err := logger2.Verify()
	require.NoError(t, err)
	assert.False(t, valid)
	assert.Equal(t, 1, brokenAt)
}

func TestHashChainPersistence(t *testing.T) {
	dir := t.TempDir()
	logger1, _ := NewLogger(dir)
	logger1.Log(Entry{Task: "task-1", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	logger2, _ := NewLogger(dir)
	logger2.Log(Entry{Task: "task-2", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	valid, _, err := logger2.Verify()
	require.NoError(t, err)
	assert.True(t, valid)
}

func TestLogSandboxLevel(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	entry := Entry{
		Task:         "test task",
		RiskLevel:    "LOW",
		Outcome:      "success",
		Duration:     100 * time.Millisecond,
		Model:        "test",
		SandboxLevel: "ulimit",
	}
	require.NoError(t, logger.Log(entry))

	records, err := logger.Recent(1)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "ulimit", records[0].SandboxLevel)
}

func TestHashChainAcrossDays(t *testing.T) {
	dir := t.TempDir()

	// Simulate a record written on a previous day
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	prevRecord := Record{
		Timestamp:  time.Now().AddDate(0, 0, -1).UTC().Format(time.RFC3339),
		ActionID:   "prev-day-id",
		Task:       "yesterday-task",
		RiskLevel:  "LOW",
		Outcome:    "success",
		DurationMs: 1000,
		Model:      "test",
	}
	prevRecord.Hash = computeHash(prevRecord)
	data, _ := json.Marshal(prevRecord)
	data = append(data, '\n')
	os.WriteFile(filepath.Join(dir, yesterday+".jsonl"), data, 0644)

	// New logger should pick up the last hash from yesterday's file
	logger, err := NewLogger(dir)
	require.NoError(t, err)
	logger.Log(Entry{Task: "today-task", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})

	// Verify chain integrity across day boundary
	valid, brokenAt, err := logger.Verify()
	require.NoError(t, err)
	assert.True(t, valid, "chain should be valid across day boundaries, broken at %d", brokenAt)
}

func TestRecordsForDate(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)

	for i := 0; i < 3; i++ {
		logger.Log(Entry{Task: fmt.Sprintf("task-%d", i), RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	}

	today := time.Now().Format("2006-01-02")
	records, err := logger.RecordsForDate(today)
	require.NoError(t, err)
	assert.Len(t, records, 3)
	assert.Equal(t, "task-0", records[0].Task)
	assert.Equal(t, "task-2", records[2].Task)

	records, err = logger.RecordsForDate("1999-01-01")
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestLastHashForDate(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)

	logger.Log(Entry{Task: "task-0", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})
	logger.Log(Entry{Task: "task-1", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})

	today := time.Now().Format("2006-01-02")
	hash, count, err := logger.LastHashForDate(today)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Equal(t, 2, count)

	records, _ := logger.RecordsForDate(today)
	assert.Equal(t, records[1].Hash, hash)

	hash, count, err = logger.LastHashForDate("1999-01-01")
	require.NoError(t, err)
	assert.Empty(t, hash)
	assert.Equal(t, 0, count)
}

func TestLogRedactsSecretInTask(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	r := redact.New(redact.RedactionConfig{
		Enabled:   true,
		RedactIPs: "none",
	})
	logger.SetRedactor(r)

	err = logger.Log(Entry{
		Task:      "deploy with sk-abcdefghijklmnopqrstuvwxyz",
		RiskLevel: "LOW",
		Outcome:   "success",
		Duration:  time.Second,
		Model:     "test",
	})
	require.NoError(t, err)

	records, err := logger.Recent(1)
	require.NoError(t, err)
	require.Len(t, records, 1)

	assert.NotContains(t, records[0].Task, "sk-abcdefghijklmnopqrstuvwxyz")
	assert.Contains(t, records[0].Task, "[REDACTED]")
}

func TestLogRedactsSecretInError(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	r := redact.New(redact.RedactionConfig{
		Enabled:   true,
		RedactIPs: "none",
	})
	logger.SetRedactor(r)

	err = logger.Log(Entry{
		Task:      "some task",
		RiskLevel: "HIGH",
		Outcome:   "failure",
		Duration:  time.Second,
		Model:     "test",
		Error:     "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.payload.sig",
	})
	require.NoError(t, err)

	records, err := logger.Recent(1)
	require.NoError(t, err)
	require.Len(t, records, 1)

	assert.NotContains(t, records[0].Error, "eyJhbGciOiJIUzI1NiJ9.payload.sig")
	assert.Contains(t, records[0].Error, "[REDACTED]")
}

func TestLogHashCoversRedactedContent(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	r := redact.New(redact.RedactionConfig{
		Enabled:   true,
		RedactIPs: "none",
	})
	logger.SetRedactor(r)

	err = logger.Log(Entry{
		Task:      "deploy with sk-abcdefghijklmnopqrstuvwxyz",
		RiskLevel: "LOW",
		Outcome:   "success",
		Duration:  time.Second,
		Model:     "test",
	})
	require.NoError(t, err)

	records, err := logger.Recent(1)
	require.NoError(t, err)
	require.Len(t, records, 1)

	// Recompute hash from the stored (redacted) record — it should match
	recomputed := computeHash(records[0])
	assert.Equal(t, recomputed, records[0].Hash, "hash must cover redacted content, not original")
}

func TestLogWithTraceFields(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	entry := Entry{
		Task:           "traced task",
		RiskLevel:      "LOW",
		Outcome:        "success",
		Duration:       100 * time.Millisecond,
		Model:          "test",
		TraceID:        "trace-abc",
		ParentActionID: "parent-123",
	}
	require.NoError(t, logger.Log(entry))

	records, err := logger.Recent(1)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "trace-abc", records[0].TraceID)
	assert.Equal(t, "parent-123", records[0].ParentActionID)
}

func TestLogTraceFieldsInHash(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	entry1 := Entry{
		Task:      "same task",
		RiskLevel: "LOW",
		Outcome:   "success",
		Duration:  time.Second,
		Model:     "test",
		TraceID:   "trace-AAA",
	}
	entry2 := Entry{
		Task:      "same task",
		RiskLevel: "LOW",
		Outcome:   "success",
		Duration:  time.Second,
		Model:     "test",
		TraceID:   "trace-BBB",
	}

	// Log to separate loggers so prev_hash doesn't differ
	dir2 := t.TempDir()
	logger2, err := NewLogger(dir2)
	require.NoError(t, err)

	require.NoError(t, logger.Log(entry1))
	require.NoError(t, logger2.Log(entry2))

	recs1, err := logger.Recent(1)
	require.NoError(t, err)
	recs2, err := logger2.Recent(1)
	require.NoError(t, err)

	assert.NotEqual(t, recs1[0].Hash, recs2[0].Hash,
		"different TraceID values must produce different hashes")
}

func TestFindByTraceID(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	// Log 3 entries: 2 with trace-X, 1 with trace-Y
	require.NoError(t, logger.Log(Entry{
		Task: "task-1", RiskLevel: "LOW", Outcome: "success",
		Duration: time.Second, Model: "test", TraceID: "trace-X",
	}))
	require.NoError(t, logger.Log(Entry{
		Task: "task-2", RiskLevel: "LOW", Outcome: "success",
		Duration: time.Second, Model: "test", TraceID: "trace-Y",
	}))
	require.NoError(t, logger.Log(Entry{
		Task: "task-3", RiskLevel: "LOW", Outcome: "success",
		Duration: time.Second, Model: "test", TraceID: "trace-X",
	}))

	results, err := logger.FindByTraceID("trace-X")
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "task-1", results[0].Task)
	assert.Equal(t, "task-3", results[1].Task)
}

func TestFindByTraceIDEmpty(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	require.NoError(t, logger.Log(Entry{
		Task: "task-1", RiskLevel: "LOW", Outcome: "success",
		Duration: time.Second, Model: "test", TraceID: "trace-X",
	}))

	results, err := logger.FindByTraceID("nonexistent-trace")
	require.NoError(t, err)
	assert.NotNil(t, results, "should return empty slice, not nil")
	assert.Len(t, results, 0)
}

func TestFindByTraceIDOrder(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	require.NoError(t, logger.Log(Entry{
		Task: "first", RiskLevel: "LOW", Outcome: "success",
		Duration: time.Second, Model: "test", TraceID: "trace-order",
	}))
	// Small sleep to ensure distinct timestamps
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, logger.Log(Entry{
		Task: "second", RiskLevel: "LOW", Outcome: "success",
		Duration: time.Second, Model: "test", TraceID: "trace-order",
	}))

	results, err := logger.FindByTraceID("trace-order")
	require.NoError(t, err)
	require.Len(t, results, 2)

	// Verify timestamp order (first should be earlier than second)
	assert.Equal(t, "first", results[0].Task)
	assert.Equal(t, "second", results[1].Task)
	assert.True(t, results[0].Timestamp <= results[1].Timestamp,
		"results should be in timestamp order")
}

func TestLogNoRedactorPassthrough(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	// Do NOT call SetRedactor — leave redactor nil

	secret := "deploy with sk-abcdefghijklmnopqrstuvwxyz"
	err = logger.Log(Entry{
		Task:      secret,
		RiskLevel: "LOW",
		Outcome:   "success",
		Duration:  time.Second,
		Model:     "test",
	})
	require.NoError(t, err)

	records, err := logger.Recent(1)
	require.NoError(t, err)
	require.Len(t, records, 1)

	// Secret should appear unchanged — backward compatibility
	assert.Equal(t, secret, records[0].Task)
}

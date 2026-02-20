package event

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// TestFormatQueueStats — verify table + JSON output
// ---------------------------------------------------------------------------

func TestFormatQueueStats(t *testing.T) {
	stats := QueueStats{
		Urgent:      2,
		Normal:      5,
		LongRunning: 1,
		Total:       8,
	}

	// ---- Table output ----
	out := FormatQueueStats(stats)

	// Should contain the header columns.
	assert.Contains(t, out, "PRIORITY")
	assert.Contains(t, out, "COUNT")

	// Should contain data for each priority.
	assert.Contains(t, out, "Urgent")
	assert.Contains(t, out, "Normal")
	assert.Contains(t, out, "LongRunning")
	assert.Contains(t, out, "Total")

	// Should contain the counts.
	assert.Contains(t, out, "2")
	assert.Contains(t, out, "5")
	assert.Contains(t, out, "1")
	assert.Contains(t, out, "8")

	// Should have 5 lines: header + 4 data rows.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	assert.Len(t, lines, 5)

	// Zero stats should also format correctly.
	zeroOut := FormatQueueStats(QueueStats{})
	assert.Contains(t, zeroOut, "PRIORITY")
	assert.Contains(t, zeroOut, "0")

	// ---- JSON output ----
	jsonOut := FormatQueueStatsJSON(stats)

	// Should be valid JSON.
	var parsed QueueStats
	err := json.Unmarshal([]byte(jsonOut), &parsed)
	require.NoError(t, err)

	// Should round-trip correctly.
	assert.Equal(t, 2, parsed.Urgent)
	assert.Equal(t, 5, parsed.Normal)
	assert.Equal(t, 1, parsed.LongRunning)
	assert.Equal(t, 8, parsed.Total)

	// Zero stats JSON should also be valid.
	zeroJSON := FormatQueueStatsJSON(QueueStats{})
	var zeroParsed QueueStats
	err = json.Unmarshal([]byte(zeroJSON), &zeroParsed)
	require.NoError(t, err)
	assert.Equal(t, 0, zeroParsed.Total)
}

// ---------------------------------------------------------------------------
// TestFormatTypes — verify list + empty
// ---------------------------------------------------------------------------

func TestFormatTypes(t *testing.T) {
	types := []string{"file.created", "task.completed", "user.login"}

	out := FormatTypes(types)

	// Should contain each type.
	for _, typ := range types {
		assert.Contains(t, out, typ)
	}

	// Should have 3 lines (one per type).
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	assert.Len(t, lines, 3)

	// Empty input should produce the placeholder message.
	assert.Equal(t, "No event types registered.", FormatTypes(nil))
	assert.Equal(t, "No event types registered.", FormatTypes([]string{}))
}

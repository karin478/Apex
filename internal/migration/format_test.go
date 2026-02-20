package migration

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// TestFormatStatus — verify status output for up-to-date and pending cases
// ---------------------------------------------------------------------------

func TestFormatStatus(t *testing.T) {
	// Case 1: up to date (current == latest).
	out := FormatStatus(3, 3)
	assert.Contains(t, out, "Schema version: 3/3")
	assert.Contains(t, out, "up to date")

	// Should have 2 lines.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	assert.Len(t, lines, 2)

	// Case 2: migrations pending.
	out = FormatStatus(1, 4)
	assert.Contains(t, out, "Schema version: 1/4")
	assert.Contains(t, out, "3 migrations pending")

	// Case 3: single migration pending.
	out = FormatStatus(2, 3)
	assert.Contains(t, out, "Schema version: 2/3")
	assert.Contains(t, out, "1 migrations pending")

	// Case 4: zero versions (empty registry).
	out = FormatStatus(0, 0)
	assert.Contains(t, out, "Schema version: 0/0")
	assert.Contains(t, out, "up to date")

	// Case 5: JSON output.
	jsonOut := FormatStatusJSON(1, 4)

	// Should be valid JSON.
	var parsed struct {
		Current int `json:"current"`
		Latest  int `json:"latest"`
		Pending int `json:"pending"`
	}
	err := json.Unmarshal([]byte(jsonOut), &parsed)
	require.NoError(t, err)

	assert.Equal(t, 1, parsed.Current)
	assert.Equal(t, 4, parsed.Latest)
	assert.Equal(t, 3, parsed.Pending)

	// Zero versions JSON.
	zeroJSON := FormatStatusJSON(0, 0)
	err = json.Unmarshal([]byte(zeroJSON), &parsed)
	require.NoError(t, err)
	assert.Equal(t, 0, parsed.Current)
	assert.Equal(t, 0, parsed.Latest)
	assert.Equal(t, 0, parsed.Pending)
}

// ---------------------------------------------------------------------------
// TestFormatPlan — verify plan table + empty case
// ---------------------------------------------------------------------------

func TestFormatPlan(t *testing.T) {
	migrations := []Migration{
		{Version: 2, Description: "add indexes"},
		{Version: 3, Description: "create events table"},
	}

	out := FormatPlan(migrations)

	// Should contain header columns.
	assert.Contains(t, out, "VERSION")
	assert.Contains(t, out, "DESCRIPTION")

	// Should contain each migration.
	assert.Contains(t, out, "2")
	assert.Contains(t, out, "add indexes")
	assert.Contains(t, out, "3")
	assert.Contains(t, out, "create events table")

	// Should have 3 lines: header + 2 data rows.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	assert.Len(t, lines, 3)

	// Empty input should produce the placeholder message.
	assert.Equal(t, "No pending migrations.", FormatPlan(nil))
	assert.Equal(t, "No pending migrations.", FormatPlan([]Migration{}))
}

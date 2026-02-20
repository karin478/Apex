package ratelimit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatStatus(t *testing.T) {
	statuses := []LimiterStatus{
		{Name: "external_api", Rate: 60, Burst: 60, Available: 58},
		{Name: "k8s_internal", Rate: 30, Burst: 30, Available: 30},
	}

	out := FormatStatus(statuses)

	// Should contain the header columns.
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "RATE")
	assert.Contains(t, out, "BURST")
	assert.Contains(t, out, "AVAILABLE")

	// Should contain data for each entry.
	assert.Contains(t, out, "external_api")
	assert.Contains(t, out, "60.0/s")
	assert.Contains(t, out, "58.0")
	assert.Contains(t, out, "k8s_internal")
	assert.Contains(t, out, "30.0/s")

	// Should have 3 lines: header + 2 data rows.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	assert.Len(t, lines, 3)

	// Empty input should produce the placeholder message.
	assert.Equal(t, "No rate limit groups configured.", FormatStatus(nil))
	assert.Equal(t, "No rate limit groups configured.", FormatStatus([]LimiterStatus{}))
}

func TestFormatStatusJSON(t *testing.T) {
	statuses := []LimiterStatus{
		{Name: "api", Rate: 10, Burst: 5, Available: 3.5},
	}

	out := FormatStatusJSON(statuses)

	// Should be valid JSON.
	var parsed []LimiterStatus
	err := json.Unmarshal([]byte(out), &parsed)
	require.NoError(t, err)

	// Should round-trip correctly.
	require.Len(t, parsed, 1)
	assert.Equal(t, "api", parsed[0].Name)
	assert.Equal(t, float64(10), parsed[0].Rate)
	assert.Equal(t, 5, parsed[0].Burst)
	assert.Equal(t, 3.5, parsed[0].Available)
}

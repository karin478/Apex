package connector

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatConnectorList(t *testing.T) {
	specs := []*ConnectorSpec{
		{
			Name:      "github",
			Type:      "http_api",
			BaseURL:   "https://api.github.com",
			RiskLevel: "medium",
		},
		{
			Name:      "stripe",
			Type:      "http_api",
			BaseURL:   "https://api.stripe.com",
			RiskLevel: "high",
		},
	}

	out := FormatConnectorList(specs)

	// Should contain the header columns.
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "TYPE")
	assert.Contains(t, out, "BASE_URL")
	assert.Contains(t, out, "RISK_LEVEL")

	// Should contain data for each entry.
	assert.Contains(t, out, "github")
	assert.Contains(t, out, "https://api.github.com")
	assert.Contains(t, out, "medium")
	assert.Contains(t, out, "stripe")
	assert.Contains(t, out, "https://api.stripe.com")
	assert.Contains(t, out, "high")

	// Should have 3 lines: header + 2 data rows.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	assert.Len(t, lines, 3)

	// Empty input should produce the placeholder message.
	assert.Equal(t, "No connectors registered.", FormatConnectorList(nil))
	assert.Equal(t, "No connectors registered.", FormatConnectorList([]*ConnectorSpec{}))
}

func TestFormatBreakerStatus(t *testing.T) {
	statuses := map[string]CBStatus{
		"github": {
			State:    CBClosed,
			Failures: 0,
			Cooldown: 30 * time.Second,
		},
		"stripe": {
			State:    CBOpen,
			Failures: 5,
			Cooldown: 60 * time.Second,
		},
	}

	// ---- Table output ----
	out := FormatBreakerStatus(statuses)

	// Should contain the header columns.
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "STATE")
	assert.Contains(t, out, "FAILURES")
	assert.Contains(t, out, "COOLDOWN")

	// Should contain data for each entry.
	assert.Contains(t, out, "github")
	assert.Contains(t, out, "CLOSED")
	assert.Contains(t, out, "stripe")
	assert.Contains(t, out, "OPEN")

	// Should have 3 lines: header + 2 data rows.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	assert.Len(t, lines, 3)

	// Empty input should produce the placeholder message.
	assert.Equal(t, "No connectors registered.", FormatBreakerStatus(nil))
	assert.Equal(t, "No connectors registered.", FormatBreakerStatus(map[string]CBStatus{}))

	// ---- JSON output ----
	jsonOut := FormatBreakerStatusJSON(statuses)

	// Should be valid JSON.
	var parsed map[string]CBStatus
	err := json.Unmarshal([]byte(jsonOut), &parsed)
	require.NoError(t, err)

	// Should round-trip correctly.
	require.Len(t, parsed, 2)
	assert.Equal(t, CBClosed, parsed["github"].State)
	assert.Equal(t, 0, parsed["github"].Failures)
	assert.Equal(t, CBOpen, parsed["stripe"].State)
	assert.Equal(t, 5, parsed["stripe"].Failures)

	// Empty input should produce "{}".
	assert.Equal(t, "{}", FormatBreakerStatusJSON(nil))
	assert.Equal(t, "{}", FormatBreakerStatusJSON(map[string]CBStatus{}))
}

package artifact

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatImpact(t *testing.T) {
	// Case 1: with affected artifacts — uses 16-char hashes to verify truncation.
	result := &ImpactResult{
		RootHash: "abcdef1234567890",
		Affected: []string{"1111111111112222", "3333333333334444"},
		Depth:    2,
	}

	got := FormatImpact(result)

	assert.Contains(t, got, "Impact for: abcdef123456")
	assert.Contains(t, got, "Affected (2):")
	assert.Contains(t, got, "  111111111111")
	assert.Contains(t, got, "  333333333333")
	assert.Contains(t, got, "Max depth: 2")

	// Case 2: no affected — short message.
	empty := &ImpactResult{
		RootHash: "abcdef1234567890",
		Affected: nil,
		Depth:    0,
	}

	got = FormatImpact(empty)
	assert.Equal(t, "No downstream impact for abcdef123456", got)
}

func TestFormatImpactJSON(t *testing.T) {
	result := &ImpactResult{
		RootHash: "abcdef1234567890",
		Affected: []string{"1111111111112222", "3333333333334444"},
		Depth:    2,
	}

	got := FormatImpactJSON(result)

	// Must be valid JSON.
	var parsed ImpactResult
	err := json.Unmarshal([]byte(got), &parsed)
	require.NoError(t, err)

	assert.Equal(t, "abcdef1234567890", parsed.RootHash)
	assert.Equal(t, []string{"1111111111112222", "3333333333334444"}, parsed.Affected)
	assert.Equal(t, 2, parsed.Depth)

	// Verify indented (contains newlines and leading spaces).
	assert.Contains(t, got, "\n")
	assert.Contains(t, got, "  ")
}

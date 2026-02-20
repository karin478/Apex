package kg

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 1. TestFormatEntitiesTable — Table output with type, name, project columns
// ---------------------------------------------------------------------------

func TestFormatEntitiesTable(t *testing.T) {
	t.Run("empty input returns no-entities message", func(t *testing.T) {
		assert.Equal(t, "No entities found.", FormatEntitiesTable(nil))
		assert.Equal(t, "No entities found.", FormatEntitiesTable([]*Entity{}))
	})

	t.Run("table with multiple entities", func(t *testing.T) {
		now := time.Now().UTC()
		entities := []*Entity{
			{
				ID:            "id-1",
				Type:          EntityFunction,
				CanonicalName: "handleRequest",
				Project:       "apex",
				Namespace:     "internal/server",
				CreatedAt:     now,
				UpdatedAt:     now,
			},
			{
				ID:            "id-2",
				Type:          EntityService,
				CanonicalName: "api-gateway",
				Project:       "apex",
				Namespace:     "",
				CreatedAt:     now,
				UpdatedAt:     now,
			},
			{
				ID:            "id-3",
				Type:          EntityFile,
				CanonicalName: "main.go",
				Project:       "my-project",
				Namespace:     "cmd",
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		}

		result := FormatEntitiesTable(entities)
		lines := strings.Split(strings.TrimRight(result, "\n"), "\n")

		// Should have header + separator + 3 data rows = 5 lines.
		require.Len(t, lines, 5, "expected 5 lines: header + separator + 3 rows")

		// Header line should contain all column names.
		assert.Contains(t, lines[0], "TYPE")
		assert.Contains(t, lines[0], "NAME")
		assert.Contains(t, lines[0], "PROJECT")
		assert.Contains(t, lines[0], "NAMESPACE")

		// Separator should be all dashes.
		assert.True(t, strings.Count(lines[1], "-") == len(lines[1]),
			"separator should be all dashes")

		// Data rows should contain entity values.
		assert.Contains(t, lines[2], "function")
		assert.Contains(t, lines[2], "handleRequest")
		assert.Contains(t, lines[2], "apex")
		assert.Contains(t, lines[2], "internal/server")

		assert.Contains(t, lines[3], "service")
		assert.Contains(t, lines[3], "api-gateway")

		assert.Contains(t, lines[4], "file")
		assert.Contains(t, lines[4], "main.go")
		assert.Contains(t, lines[4], "my-project")
		assert.Contains(t, lines[4], "cmd")

		// Columns should be pipe-separated.
		for i, line := range lines {
			if i == 1 { // skip separator
				continue
			}
			assert.Contains(t, line, " | ", "row %d should be pipe-separated", i)
		}
	})
}

// ---------------------------------------------------------------------------
// 2. TestFormatQueryResult — Entity detail + relationship list
// ---------------------------------------------------------------------------

func TestFormatQueryResult(t *testing.T) {
	now := time.Now().UTC()

	center := &Entity{
		ID:            "center-id",
		Type:          EntityService,
		CanonicalName: "api-gateway",
		Project:       "apex",
		Namespace:     "services",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	related1 := &Entity{
		ID:            "rel-1",
		Type:          EntityService,
		CanonicalName: "auth-service",
		Project:       "apex",
		Namespace:     "services",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	related2 := &Entity{
		ID:            "rel-2",
		Type:          EntityFile,
		CanonicalName: "config.yaml",
		Project:       "apex",
		Namespace:     "config",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	rels := []*Relationship{
		{
			ID:        "r-1",
			FromID:    "center-id",
			ToID:      "rel-1",
			RelType:   RelDependsOn,
			Evidence:  "gateway routes to auth",
			CreatedAt: now,
		},
		{
			ID:        "r-2",
			FromID:    "center-id",
			ToID:      "rel-2",
			RelType:   RelConfigures,
			Evidence:  "reads config",
			CreatedAt: now,
		},
	}

	result := FormatQueryResult(center, []*Entity{related1, related2}, rels)

	// Center entity header.
	assert.Contains(t, result, "Entity: api-gateway (service)")
	assert.Contains(t, result, "Project:   apex")
	assert.Contains(t, result, "Namespace: services")
	assert.Contains(t, result, "ID:        center-id")

	// Related count.
	assert.Contains(t, result, "Related (2 entities):")

	// Related entities with relationship types.
	assert.Contains(t, result, "depends_on -> auth-service (service)")
	assert.Contains(t, result, "configures -> config.yaml (file)")
}

// ---------------------------------------------------------------------------
// 3. TestFormatJSON — JSON-indented output
// ---------------------------------------------------------------------------

func TestFormatJSON(t *testing.T) {
	t.Run("entity serialisation", func(t *testing.T) {
		now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
		entity := &Entity{
			ID:            "test-id",
			Type:          EntityFunction,
			CanonicalName: "handleRequest",
			Project:       "apex",
			Namespace:     "internal/server",
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		result := FormatJSON(entity)

		// Should be valid JSON.
		var parsed map[string]interface{}
		err := json.Unmarshal([]byte(result), &parsed)
		require.NoError(t, err, "output should be valid JSON")

		// Verify key fields are present.
		assert.Equal(t, "test-id", parsed["id"])
		assert.Equal(t, "function", parsed["type"])
		assert.Equal(t, "handleRequest", parsed["canonical_name"])
		assert.Equal(t, "apex", parsed["project"])
		assert.Equal(t, "internal/server", parsed["namespace"])

		// Verify 2-space indentation.
		assert.Contains(t, result, "  \"id\":", "should use 2-space indent")
	})

	t.Run("map serialisation", func(t *testing.T) {
		m := map[string]interface{}{
			"key":   "value",
			"count": 42,
		}

		result := FormatJSON(m)

		var parsed map[string]interface{}
		err := json.Unmarshal([]byte(result), &parsed)
		require.NoError(t, err, "output should be valid JSON")
		assert.Equal(t, "value", parsed["key"])
		assert.InDelta(t, 42, parsed["count"], 0.001)
	})

	t.Run("slice serialisation", func(t *testing.T) {
		s := []string{"alpha", "beta", "gamma"}

		result := FormatJSON(s)

		var parsed []string
		err := json.Unmarshal([]byte(result), &parsed)
		require.NoError(t, err, "output should be valid JSON")
		assert.Equal(t, s, parsed)
	})
}

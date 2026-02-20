package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTrace(t *testing.T) {
	tc := NewTrace()

	// TraceID must be non-empty
	assert.NotEmpty(t, tc.TraceID)

	// TraceID must look like a UUID: 36 chars with hyphens
	assert.Len(t, tc.TraceID, 36, "TraceID should be 36 characters (UUID v4 format)")
	assert.Contains(t, tc.TraceID, "-", "TraceID should contain hyphens")

	// ParentActionID must be empty
	assert.Empty(t, tc.ParentActionID)
}

func TestChild(t *testing.T) {
	parent := NewTrace()
	actionID := "action-123"

	child := parent.Child(actionID)

	// Child inherits TraceID from parent
	assert.Equal(t, parent.TraceID, child.TraceID)

	// Child's ParentActionID is set to the given actionID
	assert.Equal(t, actionID, child.ParentActionID)

	// Parent's ParentActionID remains unchanged (empty)
	assert.Empty(t, parent.ParentActionID)
}

func TestChildChain(t *testing.T) {
	root := NewTrace()
	require.NotEmpty(t, root.TraceID)

	childActionID := "child-action"
	child := root.Child(childActionID)

	grandchildActionID := "grandchild-action"
	grandchild := child.Child(grandchildActionID)

	// All share the same TraceID
	assert.Equal(t, root.TraceID, child.TraceID)
	assert.Equal(t, root.TraceID, grandchild.TraceID)

	// Each has the correct ParentActionID
	assert.Empty(t, root.ParentActionID)
	assert.Equal(t, childActionID, child.ParentActionID)
	assert.Equal(t, grandchildActionID, grandchild.ParentActionID)
}

func TestNewTraceUnique(t *testing.T) {
	tc1 := NewTrace()
	tc2 := NewTrace()

	// Two calls must produce different TraceIDs
	assert.NotEqual(t, tc1.TraceID, tc2.TraceID)
}

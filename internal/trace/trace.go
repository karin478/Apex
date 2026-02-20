package trace

import "github.com/google/uuid"

// TraceContext carries a trace identifier and an optional parent action reference
// through a chain of operations.
type TraceContext struct {
	TraceID        string
	ParentActionID string
}

// NewTrace creates a new TraceContext with a freshly generated UUID v4 TraceID
// and an empty ParentActionID.
func NewTrace() TraceContext {
	return TraceContext{
		TraceID: uuid.New().String(),
	}
}

// Child returns a new TraceContext that shares the same TraceID but sets
// ParentActionID to the given actionID.
func (tc TraceContext) Child(actionID string) TraceContext {
	return TraceContext{
		TraceID:        tc.TraceID,
		ParentActionID: actionID,
	}
}

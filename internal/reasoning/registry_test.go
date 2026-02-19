package reasoning

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterAndGetProtocol(t *testing.T) {
	clearRegistry()

	Register(Protocol{
		Name:        "test-protocol",
		Description: "A test protocol",
		Run: func(ctx context.Context, runner Runner, proposal string, progress ProgressFunc) (*ReviewResult, error) {
			return &ReviewResult{Proposal: proposal}, nil
		},
	})

	p, ok := GetProtocol("test-protocol")
	require.True(t, ok)
	assert.Equal(t, "test-protocol", p.Name)
	assert.Equal(t, "A test protocol", p.Description)

	// Run it
	result, err := p.Run(context.Background(), nil, "test", nil)
	require.NoError(t, err)
	assert.Equal(t, "test", result.Proposal)
}

func TestGetUnknownProtocol(t *testing.T) {
	clearRegistry()
	_, ok := GetProtocol("nonexistent")
	assert.False(t, ok)
}

func TestListProtocols(t *testing.T) {
	clearRegistry()
	Register(Protocol{Name: "proto-a", Description: "A"})
	Register(Protocol{Name: "proto-b", Description: "B"})

	list := ListProtocols()
	assert.Len(t, list, 2)
}

func TestBuiltinAdversarialReview(t *testing.T) {
	clearRegistry()
	registerBuiltins()

	p, ok := GetProtocol("adversarial-review")
	require.True(t, ok)
	assert.Equal(t, "adversarial-review", p.Name)
	assert.NotNil(t, p.Run)
}

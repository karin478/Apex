package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- Limiter tests ----------

func TestLimiterAllow(t *testing.T) {
	l := NewLimiter("test", 10, 3) // burst=3

	// First 3 calls should succeed.
	assert.True(t, l.Allow(), "token 1")
	assert.True(t, l.Allow(), "token 2")
	assert.True(t, l.Allow(), "token 3")

	// 4th call should be rejected — bucket is empty.
	assert.False(t, l.Allow(), "token 4 should be rejected")
}

func TestLimiterRefill(t *testing.T) {
	l := NewLimiter("refill", 1000, 3) // 1000 tokens/s, burst=3

	// Drain the bucket.
	for i := 0; i < 3; i++ {
		require.True(t, l.Allow())
	}
	require.False(t, l.Allow(), "bucket should be empty")

	// Sleep enough for at least 1 token to refill (1000/s => 1 token per 1ms).
	time.Sleep(10 * time.Millisecond)

	assert.True(t, l.Allow(), "should succeed after refill")
}

func TestLimiterWait(t *testing.T) {
	l := NewLimiter("wait", 100, 1) // 100/s, burst=1

	// Consume the single token.
	require.True(t, l.Allow())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := l.Wait(ctx)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.True(t, elapsed < 200*time.Millisecond, "should not have timed out; elapsed=%v", elapsed)
}

func TestLimiterWaitCancel(t *testing.T) {
	l := NewLimiter("cancel", 1, 1) // very slow: 1 token/s
	require.True(t, l.Allow())       // drain

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := l.Wait(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

// ---------- Group tests ----------

func TestGroupAdd(t *testing.T) {
	g := NewGroup()
	g.Add("api", 10, 5)

	statuses := g.Status()
	require.Len(t, statuses, 1)
	assert.Equal(t, "api", statuses[0].Name)
	assert.Equal(t, float64(10), statuses[0].Rate)
	assert.Equal(t, 5, statuses[0].Burst)
}

func TestGroupAllow(t *testing.T) {
	g := NewGroup()
	g.Add("a", 10, 2)
	g.Add("b", 10, 1)

	// "a" has burst=2, should allow twice.
	ok, err := g.Allow("a")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = g.Allow("a")
	require.NoError(t, err)
	assert.True(t, ok)

	// "b" has burst=1, should allow once then reject.
	ok, err = g.Allow("b")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = g.Allow("b")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestGroupNotFound(t *testing.T) {
	g := NewGroup()

	_, err := g.Allow("missing")
	assert.EqualError(t, err, "ratelimit: group not found: missing")

	err = g.Wait("missing", context.Background())
	assert.EqualError(t, err, "ratelimit: group not found: missing")
}

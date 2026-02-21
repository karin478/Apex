package failclose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGate(t *testing.T) {
	g := NewGate()
	assert.Empty(t, g.Conditions())
}

func TestGateAddCondition(t *testing.T) {
	g := NewGate()
	g.AddCondition(Condition{Name: "test", Check: func() (bool, string) { return true, "ok" }})
	g.AddCondition(Condition{Name: "test2", Check: func() (bool, string) { return true, "ok" }})
	assert.Len(t, g.Conditions(), 2)
}

func TestGateEvaluateAllPass(t *testing.T) {
	g := NewGate()
	g.AddCondition(Condition{Name: "a", Check: func() (bool, string) { return true, "ok" }})
	g.AddCondition(Condition{Name: "b", Check: func() (bool, string) { return true, "ok" }})

	result := g.Evaluate()
	assert.True(t, result.Allowed)
	assert.Len(t, result.Passed, 2)
	assert.Empty(t, result.Failures)
}

func TestGateEvaluateOneFail(t *testing.T) {
	g := NewGate()
	g.AddCondition(Condition{Name: "good", Check: func() (bool, string) { return true, "ok" }})
	g.AddCondition(Condition{Name: "bad", Check: func() (bool, string) { return false, "broken" }})

	result := g.Evaluate()
	assert.False(t, result.Allowed)
	assert.Len(t, result.Passed, 1)
	assert.Len(t, result.Failures, 1)
	assert.Equal(t, "bad", result.Failures[0].Name)
	assert.Equal(t, "broken", result.Failures[0].Reason)
}

func TestGateEvaluateMultipleFail(t *testing.T) {
	g := NewGate()
	g.AddCondition(Condition{Name: "fail1", Check: func() (bool, string) { return false, "err1" }})
	g.AddCondition(Condition{Name: "fail2", Check: func() (bool, string) { return false, "err2" }})

	result := g.Evaluate()
	assert.False(t, result.Allowed)
	assert.Len(t, result.Failures, 2)
}

func TestGateMustPass(t *testing.T) {
	t.Run("all pass", func(t *testing.T) {
		g := NewGate()
		g.AddCondition(Condition{Name: "ok", Check: func() (bool, string) { return true, "fine" }})
		err := g.MustPass()
		require.NoError(t, err)
	})

	t.Run("one fails", func(t *testing.T) {
		g := NewGate()
		g.AddCondition(Condition{Name: "bad", Check: func() (bool, string) { return false, "nope" }})
		err := g.MustPass()
		assert.ErrorIs(t, err, ErrGateBlocked)
	})
}

func TestHealthCondition(t *testing.T) {
	t.Run("GREEN passes", func(t *testing.T) {
		c := HealthCondition("GREEN")
		pass, _ := c.Check()
		assert.True(t, pass)
	})

	t.Run("YELLOW passes", func(t *testing.T) {
		c := HealthCondition("YELLOW")
		pass, _ := c.Check()
		assert.True(t, pass)
	})

	t.Run("RED fails", func(t *testing.T) {
		c := HealthCondition("RED")
		pass, reason := c.Check()
		assert.False(t, pass)
		assert.Contains(t, reason, "RED")
	})

	t.Run("CRITICAL fails", func(t *testing.T) {
		c := HealthCondition("CRITICAL")
		pass, _ := c.Check()
		assert.False(t, pass)
	})
}

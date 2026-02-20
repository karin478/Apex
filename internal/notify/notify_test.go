package notify

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLevelValue(t *testing.T) {
	assert.Equal(t, 0, LevelValue("INFO"))
	assert.Equal(t, 1, LevelValue("WARN"))
	assert.Equal(t, 2, LevelValue("ERROR"))
	assert.Equal(t, -1, LevelValue("UNKNOWN"))
}

func TestMatchRule(t *testing.T) {
	t.Run("wildcard match", func(t *testing.T) {
		rule := Rule{EventType: "*", MinLevel: "INFO", Channel: "stdout"}
		event := Event{Type: "run.completed", Level: "INFO"}
		assert.True(t, MatchRule(rule, event))
	})

	t.Run("type match", func(t *testing.T) {
		rule := Rule{EventType: "run.completed", MinLevel: "INFO", Channel: "stdout"}
		event := Event{Type: "run.completed", Level: "WARN"}
		assert.True(t, MatchRule(rule, event))
	})

	t.Run("level filter", func(t *testing.T) {
		rule := Rule{EventType: "*", MinLevel: "ERROR", Channel: "stdout"}
		event := Event{Type: "run.completed", Level: "INFO"}
		assert.False(t, MatchRule(rule, event))
	})

	t.Run("type mismatch", func(t *testing.T) {
		rule := Rule{EventType: "health.red", MinLevel: "INFO", Channel: "stdout"}
		event := Event{Type: "run.completed", Level: "ERROR"}
		assert.False(t, MatchRule(rule, event))
	})
}

func TestNewDispatcher(t *testing.T) {
	d := NewDispatcher()
	assert.Empty(t, d.Channels())
	assert.Empty(t, d.Rules())
}

func TestDispatcherRegisterChannel(t *testing.T) {
	d := NewDispatcher()

	t.Run("valid channel", func(t *testing.T) {
		err := d.RegisterChannel(&mockChannel{name: "test"})
		require.NoError(t, err)
		assert.Contains(t, d.Channels(), "test")
	})

	t.Run("empty name", func(t *testing.T) {
		err := d.RegisterChannel(&mockChannel{name: ""})
		assert.Error(t, err)
	})
}

func TestDispatcherAddRule(t *testing.T) {
	d := NewDispatcher()
	d.AddRule(Rule{EventType: "*", MinLevel: "INFO", Channel: "stdout"})
	d.AddRule(Rule{EventType: "run.completed", MinLevel: "WARN", Channel: "file"})
	assert.Len(t, d.Rules(), 2)
}

func TestDispatcherDispatch(t *testing.T) {
	d := NewDispatcher()
	ch1 := &mockChannel{name: "ch1"}
	ch2 := &mockChannel{name: "ch2"}
	err := d.RegisterChannel(ch1)
	require.NoError(t, err)
	err = d.RegisterChannel(ch2)
	require.NoError(t, err)

	d.AddRule(Rule{EventType: "run.completed", MinLevel: "INFO", Channel: "ch1"})
	d.AddRule(Rule{EventType: "*", MinLevel: "ERROR", Channel: "ch2"})

	event := Event{Type: "run.completed", Level: "INFO", TaskID: "t1", Message: "done"}
	errs := d.Dispatch(event)
	assert.Empty(t, errs)

	// ch1 should have received the event (type match).
	assert.Len(t, ch1.received, 1)
	assert.Equal(t, "run.completed", ch1.received[0].Type)

	// ch2 should NOT have received (level too low for ERROR rule).
	assert.Empty(t, ch2.received)
}

func TestDispatcherDispatchPartialFailure(t *testing.T) {
	d := NewDispatcher()
	good := &mockChannel{name: "good"}
	bad := &mockChannel{name: "bad", failOnSend: true}
	err := d.RegisterChannel(good)
	require.NoError(t, err)
	err = d.RegisterChannel(bad)
	require.NoError(t, err)

	d.AddRule(Rule{EventType: "*", MinLevel: "INFO", Channel: "good"})
	d.AddRule(Rule{EventType: "*", MinLevel: "INFO", Channel: "bad"})

	event := Event{Type: "test", Level: "INFO", Message: "hello"}
	errs := d.Dispatch(event)

	// One error from bad channel.
	assert.Len(t, errs, 1)
	// Good channel still received the event.
	assert.Len(t, good.received, 1)
}

// mockChannel is a test double for Channel.
type mockChannel struct {
	name       string
	received   []Event
	failOnSend bool
}

func (m *mockChannel) Name() string { return m.name }

func (m *mockChannel) Send(event Event) error {
	if m.failOnSend {
		return fmt.Errorf("mock: send failed")
	}
	m.received = append(m.received, event)
	return nil
}

package mode

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultModes(t *testing.T) {
	modes := DefaultModes()
	assert.Len(t, modes, 5)
	assert.Contains(t, modes, ModeNormal)
	assert.Contains(t, modes, ModeUrgent)
	assert.Contains(t, modes, ModeExploratory)
	assert.Contains(t, modes, ModeBatch)
	assert.Contains(t, modes, ModeLongRunning)
}

func TestNewSelector(t *testing.T) {
	s := NewSelector(DefaultModes())
	current, _ := s.Current()
	assert.Equal(t, ModeNormal, current)
}

func TestSelectorSelect(t *testing.T) {
	s := NewSelector(DefaultModes())

	t.Run("valid mode", func(t *testing.T) {
		err := s.Select(ModeUrgent)
		require.NoError(t, err)
		current, _ := s.Current()
		assert.Equal(t, ModeUrgent, current)
	})

	t.Run("unknown mode", func(t *testing.T) {
		err := s.Select(Mode("INVALID"))
		assert.Error(t, err)
	})
}

func TestSelectorCurrent(t *testing.T) {
	s := NewSelector(DefaultModes())
	err := s.Select(ModeExploratory)
	require.NoError(t, err)

	current, config := s.Current()
	assert.Equal(t, ModeExploratory, current)
	assert.Equal(t, ModeExploratory, config.Name)
	assert.Equal(t, 8000, config.TokenReserve)
	assert.Equal(t, 1, config.Concurrency)
}

func TestSelectorList(t *testing.T) {
	s := NewSelector(DefaultModes())
	list := s.List()
	assert.Len(t, list, 5)
	// Verify sorted by name.
	for i := 1; i < len(list); i++ {
		assert.True(t, string(list[i-1].Name) <= string(list[i].Name),
			"expected sorted order, got %s before %s", list[i-1].Name, list[i].Name)
	}
}

func TestSelectByComplexity(t *testing.T) {
	s := NewSelector(DefaultModes())

	t.Run("low complexity", func(t *testing.T) {
		mode := s.SelectByComplexity(15)
		assert.Equal(t, ModeNormal, mode)
	})

	t.Run("medium complexity", func(t *testing.T) {
		mode := s.SelectByComplexity(45)
		assert.Equal(t, ModeExploratory, mode)
	})

	t.Run("high complexity", func(t *testing.T) {
		mode := s.SelectByComplexity(80)
		assert.Equal(t, ModeLongRunning, mode)
	})

	t.Run("boundary 30", func(t *testing.T) {
		mode := s.SelectByComplexity(30)
		assert.Equal(t, ModeExploratory, mode)
	})

	t.Run("boundary 60", func(t *testing.T) {
		mode := s.SelectByComplexity(60)
		assert.Equal(t, ModeExploratory, mode)
	})
}

func TestSelectorConfig(t *testing.T) {
	s := NewSelector(DefaultModes())

	t.Run("known mode", func(t *testing.T) {
		config, err := s.Config(ModeBatch)
		require.NoError(t, err)
		assert.Equal(t, ModeBatch, config.Name)
		assert.Equal(t, 8, config.Concurrency)
	})

	t.Run("unknown mode", func(t *testing.T) {
		_, err := s.Config(Mode("NOPE"))
		assert.Error(t, err)
	})
}

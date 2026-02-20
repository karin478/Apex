package profile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	list := r.List()
	assert.Empty(t, list)
}

func TestDefaultProfiles(t *testing.T) {
	profiles := DefaultProfiles()
	assert.Len(t, profiles, 3)

	names := make([]string, len(profiles))
	for i, p := range profiles {
		names[i] = p.Name
	}
	assert.Contains(t, names, "dev")
	assert.Contains(t, names, "staging")
	assert.Contains(t, names, "prod")
}

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()

	t.Run("valid profile", func(t *testing.T) {
		err := r.Register(Profile{Name: "test", Mode: "NORMAL"})
		require.NoError(t, err)

		p, err := r.Get("test")
		require.NoError(t, err)
		assert.Equal(t, "test", p.Name)
	})

	t.Run("empty name", func(t *testing.T) {
		err := r.Register(Profile{Name: ""})
		assert.Error(t, err)
	})
}

func TestRegistryActivate(t *testing.T) {
	r := NewRegistry()
	err := r.Register(Profile{Name: "dev", Mode: "NORMAL"})
	require.NoError(t, err)

	t.Run("valid activate", func(t *testing.T) {
		err := r.Activate("dev")
		require.NoError(t, err)

		active, err := r.Active()
		require.NoError(t, err)
		assert.Equal(t, "dev", active.Name)
	})

	t.Run("unknown profile", func(t *testing.T) {
		err := r.Activate("unknown")
		assert.ErrorIs(t, err, ErrProfileNotFound)
	})
}

func TestRegistryActive(t *testing.T) {
	r := NewRegistry()

	t.Run("no active profile", func(t *testing.T) {
		_, err := r.Active()
		assert.ErrorIs(t, err, ErrNoActiveProfile)
	})

	t.Run("with active profile", func(t *testing.T) {
		err := r.Register(Profile{Name: "staging", Mode: "EXPLORATORY"})
		require.NoError(t, err)
		err = r.Activate("staging")
		require.NoError(t, err)

		active, err := r.Active()
		require.NoError(t, err)
		assert.Equal(t, "staging", active.Name)
		assert.Equal(t, "EXPLORATORY", active.Mode)
	})
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()
	err := r.Register(Profile{Name: "prod", Mode: "BATCH", Concurrency: 8})
	require.NoError(t, err)

	t.Run("known profile", func(t *testing.T) {
		p, err := r.Get("prod")
		require.NoError(t, err)
		assert.Equal(t, "prod", p.Name)
		assert.Equal(t, 8, p.Concurrency)
	})

	t.Run("unknown profile", func(t *testing.T) {
		_, err := r.Get("nope")
		assert.ErrorIs(t, err, ErrProfileNotFound)
	})
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	for _, p := range DefaultProfiles() {
		err := r.Register(p)
		require.NoError(t, err)
	}

	list := r.List()
	assert.Len(t, list, 3)
	// Verify sorted by name.
	for i := 1; i < len(list); i++ {
		assert.True(t, list[i-1].Name <= list[i].Name,
			"expected sorted order, got %s before %s", list[i-1].Name, list[i].Name)
	}
}

package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLevelOrdering(t *testing.T) {
	assert.True(t, None < Ulimit)
	assert.True(t, Ulimit < Docker)
}

func TestLevelString(t *testing.T) {
	assert.Equal(t, "none", None.String())
	assert.Equal(t, "ulimit", Ulimit.String())
	assert.Equal(t, "docker", Docker.String())
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
		err   bool
	}{
		{"none", None, false},
		{"ulimit", Ulimit, false},
		{"docker", Docker, false},
		{"DOCKER", Docker, false},
		{"auto", None, true},
		{"invalid", None, true},
	}
	for _, tt := range tests {
		got, err := ParseLevel(tt.input)
		if tt.err {
			assert.Error(t, err, "input: %s", tt.input)
		} else {
			assert.NoError(t, err, "input: %s", tt.input)
			assert.Equal(t, tt.want, got, "input: %s", tt.input)
		}
	}
}

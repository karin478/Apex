package sandbox

import (
	"context"
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

func TestNoneBackend(t *testing.T) {
	sb := &NoneSandbox{}
	assert.Equal(t, None, sb.Level())

	bin, args, err := sb.Wrap(context.Background(), "claude", []string{"-p", "hello"})
	assert.NoError(t, err)
	assert.Equal(t, "claude", bin)
	assert.Equal(t, []string{"-p", "hello"}, args)
}

func TestUlimitBackend(t *testing.T) {
	sb := &UlimitSandbox{
		MaxMemoryKB:   2097152, // 2GB
		MaxCPUSec:     300,
		MaxFileSizeMB: 100,
	}
	assert.Equal(t, Ulimit, sb.Level())

	bin, args, err := sb.Wrap(context.Background(), "claude", []string{"-p", "hello"})
	assert.NoError(t, err)
	assert.Equal(t, "sh", bin)
	// The args should be: ["-c", "ulimit -v 2097152 -t 300 -f 204800; exec claude -p hello"]
	assert.Len(t, args, 2)
	assert.Equal(t, "-c", args[0])
	assert.Contains(t, args[1], "ulimit")
	assert.Contains(t, args[1], "-v 2097152")
	assert.Contains(t, args[1], "-t 300")
	assert.Contains(t, args[1], "exec claude -p hello")
}

func TestUlimitDefaults(t *testing.T) {
	sb := &UlimitSandbox{} // all zero
	_, args, err := sb.Wrap(context.Background(), "claude", []string{"-p", "hi"})
	assert.NoError(t, err)
	// Should use defaults
	assert.Contains(t, args[1], "ulimit")
	assert.Contains(t, args[1], "exec claude")
}

package sandbox

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Level represents sandbox isolation strength (higher = stronger).
type Level int

const (
	None   Level = iota
	Ulimit
	Docker
)

func (l Level) String() string {
	switch l {
	case None:
		return "none"
	case Ulimit:
		return "ulimit"
	case Docker:
		return "docker"
	default:
		return "unknown"
	}
}

// ParseLevel converts a string to a Level. Does not accept "auto".
func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(s) {
	case "none":
		return None, nil
	case "ulimit":
		return Ulimit, nil
	case "docker":
		return Docker, nil
	default:
		return None, fmt.Errorf("unknown sandbox level: %q", s)
	}
}

// Sandbox wraps a command with isolation.
type Sandbox interface {
	Level() Level
	Wrap(ctx context.Context, binary string, args []string) (string, []string, error)
}

// Detect returns the strongest available sandbox backend.
// Checks Docker first (< 50ms timeout), falls back to Ulimit, then None.
func Detect() Sandbox {
	if dockerAvailable() {
		return &DockerSandbox{}
	}
	return &UlimitSandbox{}
}

// ForLevel returns a sandbox for a specific level.
func ForLevel(level Level) (Sandbox, error) {
	switch level {
	case Docker:
		if !dockerAvailable() {
			return nil, fmt.Errorf("docker is not available")
		}
		return &DockerSandbox{}, nil
	case Ulimit:
		return &UlimitSandbox{}, nil
	case None:
		return &NoneSandbox{}, nil
	default:
		return nil, fmt.Errorf("unknown level: %d", level)
	}
}

func dockerAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	return exec.CommandContext(ctx, "docker", "info").Run() == nil
}

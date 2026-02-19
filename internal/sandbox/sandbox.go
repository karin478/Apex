package sandbox

import (
	"context"
	"fmt"
	"strings"
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

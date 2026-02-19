package sandbox

import (
	"context"
	"fmt"
	"strings"
)

// UlimitSandbox wraps commands with resource limits via ulimit.
type UlimitSandbox struct {
	MaxMemoryKB   int // -v: virtual memory in KB (default: 2097152 = 2GB)
	MaxCPUSec     int // -t: CPU seconds (default: 300)
	MaxFileSizeMB int // -f: file size in MB, converted to 512-byte blocks (default: 100MB)
}

func (u *UlimitSandbox) Level() Level { return Ulimit }

func (u *UlimitSandbox) Wrap(_ context.Context, binary string, args []string) (string, []string, error) {
	mem := u.MaxMemoryKB
	if mem <= 0 {
		mem = 2097152 // 2GB
	}
	cpu := u.MaxCPUSec
	if cpu <= 0 {
		cpu = 300
	}
	fileMB := u.MaxFileSizeMB
	if fileMB <= 0 {
		fileMB = 100
	}
	fileBlocks := fileMB * 2048 // 1MB = 2048 blocks of 512 bytes

	// Build the shell command with ulimit + exec
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shellQuote(a)
	}

	cmd := fmt.Sprintf("ulimit -v %d -t %d -f %d; exec %s %s",
		mem, cpu, fileBlocks, shellQuote(binary), strings.Join(quoted, " "))

	return "sh", []string{"-c", cmd}, nil
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	// If no special chars, return as-is
	safe := true
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '/' || c == ':' || c == '=') {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

package retry

import (
	"context"
	"strings"
)

// ErrorKind classifies an error for retry decisions.
type ErrorKind int

const (
	Retriable    ErrorKind = iota // Transient — worth retrying
	NonRetriable                   // Permanent — fail immediately
	Unknown                        // Unclassified — treat as retriable
)

func (k ErrorKind) String() string {
	switch k {
	case Retriable:
		return "RETRIABLE"
	case NonRetriable:
		return "NON_RETRIABLE"
	default:
		return "UNKNOWN"
	}
}

// nonRetriableKeywords in stderr indicate permanent failures.
var nonRetriableKeywords = []string{
	"permission denied",
	"invalid",
	"not found",
	"unauthorized",
}

// retriableKeywords in stderr indicate transient failures.
var retriableKeywords = []string{
	"timeout",
	"rate limit",
	"connection",
	"temporary",
	"unavailable",
}

// Classify determines if an error is worth retrying based on the error type,
// process exit code, and stderr content.
func Classify(err error, exitCode int, stderr string) ErrorKind {
	// Context errors are always retriable.
	if err == context.DeadlineExceeded || err == context.Canceled {
		return Retriable
	}

	lower := strings.ToLower(stderr)

	// High exit codes (2+) are non-retriable (usage errors, fatal).
	if exitCode >= 2 {
		return NonRetriable
	}

	// Check stderr for non-retriable keywords first (higher priority).
	for _, kw := range nonRetriableKeywords {
		if strings.Contains(lower, kw) {
			return NonRetriable
		}
	}

	// Check stderr for retriable keywords.
	for _, kw := range retriableKeywords {
		if strings.Contains(lower, kw) {
			return Retriable
		}
	}

	return Unknown
}

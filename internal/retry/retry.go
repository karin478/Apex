package retry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
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
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
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

// Policy defines retry behavior with exponential backoff.
type Policy struct {
	MaxAttempts int
	InitDelay   time.Duration
	Multiplier  float64
	MaxDelay    time.Duration
}

// DefaultPolicy returns sensible retry defaults.
func DefaultPolicy() Policy {
	return Policy{
		MaxAttempts: 3,
		InitDelay:   2 * time.Second,
		Multiplier:  2.0,
		MaxDelay:    30 * time.Second,
	}
}

// delay calculates the backoff duration for a given attempt (0-indexed).
func (p Policy) delay(attempt int) time.Duration {
	d := time.Duration(float64(p.InitDelay) * math.Pow(p.Multiplier, float64(attempt)))
	if d > p.MaxDelay {
		d = p.MaxDelay
	}
	return d
}

// Execute runs fn up to MaxAttempts times. If fn returns a nil error, the result
// is returned immediately. If the ErrorKind is NonRetriable, the error is returned
// without further attempts. For Retriable/Unknown, it waits with exponential
// backoff before the next attempt. Respects context cancellation.
func (p Policy) Execute(ctx context.Context, fn func() (string, error, ErrorKind)) (string, error) {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < p.MaxAttempts; attempt++ {
		result, err, kind := fn()
		if err == nil {
			return result, nil
		}
		lastErr = err

		if kind == NonRetriable {
			return "", err
		}

		// Last attempt — don't sleep, just fail.
		if attempt == p.MaxAttempts-1 {
			break
		}

		wait := p.delay(attempt)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "", fmt.Errorf("task failed after %d attempts: %w", p.MaxAttempts, lastErr)
}

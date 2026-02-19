package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClassifyTimeout(t *testing.T) {
	kind := Classify(context.DeadlineExceeded, 0, "")
	assert.Equal(t, Retriable, kind)
}

func TestClassifyContextCanceled(t *testing.T) {
	kind := Classify(context.Canceled, 0, "")
	assert.Equal(t, Retriable, kind)
}

func TestClassifyRateLimit(t *testing.T) {
	kind := Classify(errors.New("fail"), 1, "rate limit exceeded")
	assert.Equal(t, Retriable, kind)
}

func TestClassifyConnectionError(t *testing.T) {
	kind := Classify(errors.New("fail"), 1, "connection refused")
	assert.Equal(t, Retriable, kind)
}

func TestClassifyPermissionDenied(t *testing.T) {
	kind := Classify(errors.New("fail"), 1, "permission denied")
	assert.Equal(t, NonRetriable, kind)
}

func TestClassifyInvalidTask(t *testing.T) {
	kind := Classify(errors.New("fail"), 1, "invalid argument provided")
	assert.Equal(t, NonRetriable, kind)
}

func TestClassifyHighExitCode(t *testing.T) {
	kind := Classify(errors.New("fail"), 2, "something broke")
	assert.Equal(t, NonRetriable, kind)
}

func TestClassifyUnknown(t *testing.T) {
	kind := Classify(errors.New("fail"), 1, "some weird error")
	assert.Equal(t, Unknown, kind)
}

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	assert.Equal(t, 3, p.MaxAttempts)
	assert.Equal(t, 2*time.Second, p.InitDelay)
	assert.Equal(t, 2.0, p.Multiplier)
	assert.Equal(t, 30*time.Second, p.MaxDelay)
}

func TestPolicyExecuteSuccessFirstAttempt(t *testing.T) {
	p := Policy{MaxAttempts: 3, InitDelay: time.Millisecond, Multiplier: 2.0, MaxDelay: time.Second}
	calls := 0
	result, err := p.Execute(context.Background(), func() (string, error, ErrorKind) {
		calls++
		return "ok", nil, Retriable
	})
	assert.NoError(t, err)
	assert.Equal(t, "ok", result)
	assert.Equal(t, 1, calls)
}

func TestPolicyExecuteRetriableSucceedsOnThird(t *testing.T) {
	p := Policy{MaxAttempts: 3, InitDelay: time.Millisecond, Multiplier: 1.0, MaxDelay: time.Second}
	calls := 0
	result, err := p.Execute(context.Background(), func() (string, error, ErrorKind) {
		calls++
		if calls < 3 {
			return "", errors.New("transient"), Retriable
		}
		return "ok", nil, Retriable
	})
	assert.NoError(t, err)
	assert.Equal(t, "ok", result)
	assert.Equal(t, 3, calls)
}

func TestPolicyExecuteNonRetriableStopsImmediately(t *testing.T) {
	p := Policy{MaxAttempts: 3, InitDelay: time.Millisecond, Multiplier: 2.0, MaxDelay: time.Second}
	calls := 0
	_, err := p.Execute(context.Background(), func() (string, error, ErrorKind) {
		calls++
		return "", errors.New("permanent"), NonRetriable
	})
	assert.Error(t, err)
	assert.Equal(t, 1, calls)
	assert.Contains(t, err.Error(), "permanent")
}

func TestPolicyExecuteUnknownRetriesLikeRetriable(t *testing.T) {
	p := Policy{MaxAttempts: 2, InitDelay: time.Millisecond, Multiplier: 1.0, MaxDelay: time.Second}
	calls := 0
	_, err := p.Execute(context.Background(), func() (string, error, ErrorKind) {
		calls++
		return "", errors.New("mystery"), Unknown
	})
	assert.Error(t, err)
	assert.Equal(t, 2, calls)
	assert.Contains(t, err.Error(), "after 2 attempts")
}

func TestPolicyExecuteRespectsContext(t *testing.T) {
	p := Policy{MaxAttempts: 10, InitDelay: time.Second, Multiplier: 2.0, MaxDelay: time.Minute}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := p.Execute(ctx, func() (string, error, ErrorKind) {
		return "", errors.New("fail"), Retriable
	})
	assert.Error(t, err)
}

func TestPolicyDelayCalculation(t *testing.T) {
	p := Policy{MaxAttempts: 5, InitDelay: 100 * time.Millisecond, Multiplier: 2.0, MaxDelay: 500 * time.Millisecond}
	// attempt 0: 100ms, attempt 1: 200ms, attempt 2: 400ms, attempt 3: 500ms (capped)
	assert.Equal(t, 100*time.Millisecond, p.delay(0))
	assert.Equal(t, 200*time.Millisecond, p.delay(1))
	assert.Equal(t, 400*time.Millisecond, p.delay(2))
	assert.Equal(t, 500*time.Millisecond, p.delay(3))
}

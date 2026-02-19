package retry

import (
	"context"
	"errors"
	"testing"

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

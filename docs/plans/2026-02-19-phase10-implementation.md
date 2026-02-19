# Phase 10: Fault Tolerance & Retry Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add retry with exponential backoff to the Pool execution layer so transient task failures are automatically retried before marking a DAG node as failed.

**Architecture:** New `internal/retry` package with error classification (3-level) and a `Policy.Execute()` retry loop. Pool wraps each task in the retry policy. DAG state machine unchanged — nodes stay Running during retries. Config extended with `RetryConfig`.

**Tech Stack:** Go stdlib, Testify

---

### Task 1: Retry Package — Error Classification

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/retry/retry.go`
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/retry/retry_test.go`

**Step 1: Write the failing tests**

Create `retry_test.go`:

```go
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
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/retry/ -v -run TestClassify`
Expected: FAIL — package does not exist

**Step 3: Write minimal implementation**

Create `retry.go`:

```go
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
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/retry/ -v -run TestClassify`
Expected: All 8 tests PASS

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/retry/retry.go internal/retry/retry_test.go
git commit -m "feat(retry): add error classification for fault tolerance"
```

---

### Task 2: Retry Package — Policy & Execute Loop

**Files:**
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/retry/retry.go`
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/retry/retry_test.go`

**Step 1: Write the failing tests**

Append to `retry_test.go`:

```go
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
```

Add `"time"` to the imports.

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/retry/ -v -run "TestDefault|TestPolicy"`
Expected: FAIL — Policy, DefaultPolicy, delay not defined

**Step 3: Write minimal implementation**

Append to `retry.go`:

```go
import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

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
```

Note: merge the two `import` blocks in retry.go into one.

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/retry/ -v`
Expected: All 14 tests PASS

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/retry/retry.go internal/retry/retry_test.go
git commit -m "feat(retry): add Policy with exponential backoff execute loop"
```

---

### Task 3: Config — Add RetryConfig

**Files:**
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/config/config.go`
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/config/config_test.go`

**Step 1: Write the failing tests**

Append to `config_test.go`:

```go
func TestDefaultConfigPhase10(t *testing.T) {
	cfg := Default()
	assert.Equal(t, 3, cfg.Retry.MaxAttempts)
	assert.Equal(t, 2, cfg.Retry.InitDelaySeconds)
	assert.Equal(t, 2.0, cfg.Retry.Multiplier)
	assert.Equal(t, 30, cfg.Retry.MaxDelaySeconds)
}

func TestLoadConfigPhase10Override(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`retry:
  max_attempts: 5
  init_delay_seconds: 1
  multiplier: 3.0
  max_delay_seconds: 60
`)
	require.NoError(t, os.WriteFile(configPath, content, 0644))
	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, 5, cfg.Retry.MaxAttempts)
	assert.Equal(t, 1, cfg.Retry.InitDelaySeconds)
	assert.Equal(t, 3.0, cfg.Retry.Multiplier)
	assert.Equal(t, 60, cfg.Retry.MaxDelaySeconds)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/config/ -v -run Phase10`
Expected: FAIL — cfg.Retry undefined

**Step 3: Write minimal implementation**

In `config.go`, add the struct after `ContextConfig`:

```go
type RetryConfig struct {
	MaxAttempts      int     `yaml:"max_attempts"`
	InitDelaySeconds int     `yaml:"init_delay_seconds"`
	Multiplier       float64 `yaml:"multiplier"`
	MaxDelaySeconds  int     `yaml:"max_delay_seconds"`
}
```

Add to `Config` struct:

```go
Retry      RetryConfig      `yaml:"retry"`
```

In `Default()`, add:

```go
Retry: RetryConfig{
	MaxAttempts:      3,
	InitDelaySeconds: 2,
	Multiplier:       2.0,
	MaxDelaySeconds:  30,
},
```

In `Load()`, add defaults after existing zero-value checks:

```go
if cfg.Retry.MaxAttempts == 0 {
	cfg.Retry.MaxAttempts = 3
}
if cfg.Retry.InitDelaySeconds == 0 {
	cfg.Retry.InitDelaySeconds = 2
}
if cfg.Retry.Multiplier == 0 {
	cfg.Retry.Multiplier = 2.0
}
if cfg.Retry.MaxDelaySeconds == 0 {
	cfg.Retry.MaxDelaySeconds = 30
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/config/ -v`
Expected: All tests PASS

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add RetryConfig for fault tolerance settings"
```

---

### Task 4: Pool Integration — Retry on Task Failure

**Files:**
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/pool/pool.go`
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/pool/pool_test.go`

**Step 1: Write the failing tests**

Add a new `retryRunner` mock and tests to `pool_test.go`:

```go
type retryRunner struct {
	attempts   map[string]*atomic.Int32
	failUntil  int // fail this many times before succeeding
	exitCode   int
	stderr     string
	mu         sync.Mutex
}

func newRetryRunner(failUntil int, exitCode int, stderr string) *retryRunner {
	return &retryRunner{
		attempts:  make(map[string]*atomic.Int32),
		failUntil: failUntil,
		exitCode:  exitCode,
		stderr:    stderr,
	}
}

func (r *retryRunner) RunTask(ctx context.Context, task string) (string, error) {
	r.mu.Lock()
	if _, ok := r.attempts[task]; !ok {
		r.attempts[task] = &atomic.Int32{}
	}
	counter := r.attempts[task]
	r.mu.Unlock()

	n := int(counter.Add(1))
	if n <= r.failUntil {
		return "", &TaskError{ExitCode: r.exitCode, Stderr: r.stderr, Msg: fmt.Sprintf("attempt %d failed", n)}
	}
	return "ok", nil
}

func (r *retryRunner) attemptsFor(task string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.attempts[task]; ok {
		return int(c.Load())
	}
	return 0
}
```

Add `TaskError` type (needed for the runner to carry exit code/stderr):

```go
// TaskError carries structured error info for retry classification.
type TaskError struct {
	ExitCode int
	Stderr   string
	Msg      string
}

func (e *TaskError) Error() string { return e.Msg }
```

Add retry-specific tests:

```go
func TestExecuteWithRetrySuccess(t *testing.T) {
	nodes := []dag.NodeSpec{
		{ID: "a", Task: "flaky", Depends: []string{}},
	}
	d, _ := dag.New(nodes)
	runner := newRetryRunner(2, 1, "connection refused")

	policy := retry.Policy{MaxAttempts: 3, InitDelay: time.Millisecond, Multiplier: 1.0, MaxDelay: time.Second}
	p := New(4, runner)
	p.RetryPolicy = &policy

	err := p.Execute(context.Background(), d)
	require.NoError(t, err)
	assert.Equal(t, dag.Completed, d.Nodes["a"].Status)
	assert.Equal(t, 3, runner.attemptsFor("flaky"))
}

func TestExecuteWithRetryExhausted(t *testing.T) {
	nodes := []dag.NodeSpec{
		{ID: "a", Task: "always-fail", Depends: []string{}},
	}
	d, _ := dag.New(nodes)
	runner := newRetryRunner(999, 1, "connection refused")

	policy := retry.Policy{MaxAttempts: 3, InitDelay: time.Millisecond, Multiplier: 1.0, MaxDelay: time.Second}
	p := New(4, runner)
	p.RetryPolicy = &policy

	err := p.Execute(context.Background(), d)
	assert.NoError(t, err) // pool returns nil; failure recorded in DAG
	assert.Equal(t, dag.Failed, d.Nodes["a"].Status)
	assert.Equal(t, 3, runner.attemptsFor("always-fail"))
}

func TestExecuteWithRetryNonRetriable(t *testing.T) {
	nodes := []dag.NodeSpec{
		{ID: "a", Task: "perm-fail", Depends: []string{}},
	}
	d, _ := dag.New(nodes)
	runner := newRetryRunner(999, 2, "permission denied")

	policy := retry.Policy{MaxAttempts: 3, InitDelay: time.Millisecond, Multiplier: 1.0, MaxDelay: time.Second}
	p := New(4, runner)
	p.RetryPolicy = &policy

	err := p.Execute(context.Background(), d)
	assert.NoError(t, err)
	assert.Equal(t, dag.Failed, d.Nodes["a"].Status)
	assert.Equal(t, 1, runner.attemptsFor("perm-fail")) // stopped after 1
}

func TestExecuteWithoutRetryPolicyFallsBack(t *testing.T) {
	nodes := []dag.NodeSpec{
		{ID: "a", Task: "fail-task", Depends: []string{}},
	}
	d, _ := dag.New(nodes)
	runner := &failRunner{failKeywords: []string{"fail"}}
	p := New(4, runner) // no RetryPolicy set

	err := p.Execute(context.Background(), d)
	assert.NoError(t, err)
	assert.Equal(t, dag.Failed, d.Nodes["a"].Status) // immediate fail, no retry
}
```

Add imports: `"sync"` and `"github.com/lyndonlyu/apex/internal/retry"`.

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/pool/ -v -run "Retry|Fallback"`
Expected: FAIL — RetryPolicy, TaskError not defined

**Step 3: Write minimal implementation**

In `pool.go`, add `RetryPolicy` field and update the import:

```go
import (
	"context"
	"sync"
	"time"

	"github.com/lyndonlyu/apex/internal/dag"
	"github.com/lyndonlyu/apex/internal/retry"
)

type Pool struct {
	maxWorkers  int
	runner      Runner
	RetryPolicy *retry.Policy
}
```

Update the goroutine in `Execute()` to use retry when a policy is set:

```go
go func(n *dag.Node) {
	defer wg.Done()
	defer func() { <-sem }()

	if p.RetryPolicy != nil {
		result, err := p.RetryPolicy.Execute(ctx, func() (string, error, retry.ErrorKind) {
			res, runErr := p.runner.RunTask(ctx, n.Task)
			if runErr != nil {
				exitCode := 0
				stderr := ""
				if te, ok := runErr.(interface{ ExitInfo() (int, string) }); ok {
					exitCode, stderr = te.ExitInfo()
				}
				kind := retry.Classify(runErr, exitCode, stderr)
				return res, runErr, kind
			}
			return res, nil, retry.Retriable
		})
		if err != nil {
			d.MarkFailed(n.ID, err.Error())
			return
		}
		d.MarkCompleted(n.ID, result)
	} else {
		result, err := p.runner.RunTask(ctx, n.Task)
		if err != nil {
			d.MarkFailed(n.ID, err.Error())
			return
		}
		d.MarkCompleted(n.ID, result)
	}
}(node)
```

Update `TaskError` in `pool_test.go` to implement the `ExitInfo()` interface:

```go
func (e *TaskError) ExitInfo() (int, string) { return e.ExitCode, e.Stderr }
```

**Step 4: Run all pool tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/pool/ -v`
Expected: All tests PASS (both old and new)

**Step 5: Run full test suite**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./... 2>&1 | tail -30`
Expected: All packages PASS

**Step 6: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/pool/pool.go internal/pool/pool_test.go
git commit -m "feat: integrate retry policy into pool execution layer"
```

---

### Task 5: Update PROGRESS.md & Final Verification

**Files:**
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/PROGRESS.md`

**Step 1: Run full test suite**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./... -v 2>&1 | tail -50`
Expected: All packages PASS

**Step 2: Update PROGRESS.md**

Move Phase 10 from "Current" to "Completed" table. Set current to "Phase 11 — TBD".

**Step 3: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add PROGRESS.md
git commit -m "docs: mark Phase 10 Fault Tolerance & Retry complete"
```

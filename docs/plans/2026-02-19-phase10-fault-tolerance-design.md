# Phase 10 Fault Tolerance & Retry Design

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Retry layer | Pool only | DAG sees only final success/fail; simplest, minimal changes |
| Error classification | 3-level: RETRIABLE / NON_RETRIABLE / UNKNOWN | Covers all cases without over-engineering |
| Backoff strategy | Exponential with cap | Industry standard, prevents thundering herd |
| UNKNOWN errors | Treat as RETRIABLE | Fail-open for transient issues; bounded by max_attempts |
| DAG state machine | No changes | Node stays Running during retries; transparent |

## Architecture

### 1. Error Classification (`internal/retry/`)

```go
type ErrorKind int

const (
    Retriable    ErrorKind = iota // Timeout, transient CLI errors
    NonRetriable                   // Invalid task, permission denied
    Unknown                        // Unclassified → retry anyway
)

func Classify(err error, exitCode int, stderr string) ErrorKind
```

Classification rules:
- **RETRIABLE**: context.DeadlineExceeded, exit code 1 without "permission denied" / "invalid", stderr contains "timeout" / "rate limit" / "connection"
- **NON_RETRIABLE**: exit code 2+, stderr contains "permission denied" / "invalid" / "not found"
- **UNKNOWN**: everything else

### 2. Retry Policy (`internal/retry/`)

```go
type Policy struct {
    MaxAttempts int           // default 3
    InitDelay   time.Duration // default 2s
    Multiplier  float64       // default 2.0
    MaxDelay    time.Duration // default 30s
}

func (p Policy) Execute(ctx context.Context, fn func() (string, error, ErrorKind)) (string, error)
```

Delay formula: `delay = min(InitDelay * Multiplier^(attempt-1), MaxDelay)`

Sequence for default config: 2s → 4s → give up (3 attempts total).

### 3. Pool Integration

Current flow:
```
pool.Execute() → runner.RunTask() → fail → MarkFailed()
```

New flow:
```
pool.Execute() → retry.Policy.Execute(runner.RunTask) → all retries exhausted → MarkFailed()
                                                       → any attempt succeeds  → MarkCompleted()
```

Changes to `pool.go`:
- Pool gains a `RetryPolicy` field
- The goroutine in Execute wraps `runner.RunTask` in `policy.Execute()`
- Node stays `Running` during retry attempts

### 4. Configuration (`config.go`)

```yaml
retry:
  max_attempts: 3
  init_delay_seconds: 2
  multiplier: 2.0
  max_delay_seconds: 30
```

```go
type RetryConfig struct {
    MaxAttempts     int     `yaml:"max_attempts"`
    InitDelaySeconds int    `yaml:"init_delay_seconds"`
    Multiplier      float64 `yaml:"multiplier"`
    MaxDelaySeconds int     `yaml:"max_delay_seconds"`
}
```

### 5. Observability

- Audit log entries for each retry attempt (via existing audit logger)
- Final failure includes retry count in error message: `"task failed after 3 attempts: <last error>"`

## Scope

**In scope:**
- `internal/retry/retry.go` — ErrorKind, Classify(), Policy, Policy.Execute()
- `internal/retry/retry_test.go` — unit tests
- `internal/pool/pool.go` — integrate retry into execution loop
- `internal/pool/pool_test.go` — update tests
- `internal/config/config.go` — add RetryConfig
- `internal/config/config_test.go` — update tests

**Out of scope (YAGNI):**
- Circuit breaker
- ESCALATED state
- Write-ahead log (WAL)
- DAG state machine changes
- Per-node retry policy overrides

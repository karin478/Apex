# Phase 6 Kill Switch Design

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scope | MVP + Integration: file watcher + SIGTERM + DAG integration + CLI commands | Practical safety without System Health Level dependency |
| Mechanism | goroutine watcher + context cancellation | Zero-invasion via Go's native context propagation |
| Poll interval | 200ms | Fast enough for safety, low CPU overhead |
| Kill Switch file | `~/.claude/KILL_SWITCH` | Matches architecture spec |

## Architecture

### 1. Kill Switch Watcher (`internal/killswitch`)

New package with a single `Watcher` type:

```go
type Watcher struct {
    path     string
    interval time.Duration
}

func New(path string) *Watcher
func (w *Watcher) Watch(ctx context.Context) (context.Context, context.CancelFunc)
func (w *Watcher) IsActive() bool
func (w *Watcher) Clear() error
func (w *Watcher) Activate(reason string) error
```

**`Watch(ctx)`** returns a derived context that gets cancelled when:
- The KILL_SWITCH file appears (watcher detects it)
- The parent context is cancelled (normal completion)

The goroutine polls every 200ms using `os.Stat()`. When the file exists, it calls `cancel()` and exits.

**`Activate(reason)`** creates the KILL_SWITCH file with the reason as content.

**`Clear()`** removes the KILL_SWITCH file.

**`IsActive()`** checks if the file currently exists (stat-only, no read).

### 2. CLI Commands

**`apex kill-switch [reason]`**:
- Creates `~/.claude/KILL_SWITCH` with optional reason text
- Prints confirmation with file path
- If already active, prints current status

**`apex resume`**:
- Deletes `~/.claude/KILL_SWITCH`
- Prints confirmation
- If not active, prints "no kill switch active"

### 3. Integration into DAG Execution

`cmd/apex/run.go` changes:

```
1. Create Watcher with default path
2. Check IsActive() before starting — refuse to run if active
3. Call Watch(ctx) to get killCtx
4. Pass killCtx to Pool.Execute()
5. Pool workers receive cancellation via context
6. After execution, check if context was cancelled by kill switch
7. Audit log records "kill_switch" as outcome for interrupted nodes
8. Manifest records "killed" as outcome
```

### 4. Pool Integration

`internal/pool/pool.go` already accepts a context. The kill switch context propagation requires no changes to the Pool package — context cancellation flows naturally through `ctx.Done()`.

Each worker checks `ctx.Err()` before launching a new task. If cancelled, the node is marked as `dag.Failed` with error "kill switch activated".

### 5. Testing Strategy

- `TestWatcherDetectsFile`: Create file during watch, verify context cancelled
- `TestWatcherParentCancel`: Cancel parent context, verify cleanup
- `TestIsActive`: Check file exists/not exists
- `TestActivateAndClear`: Round-trip create/delete
- `TestRunRefusesWhenActive`: Verify run command rejects when kill switch active

# Phase 7 Snapshot & Rollback Design

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scope | MVP: git stash snapshots + manual restore + list command | Practical rollback without CoW/OverlayFS complexity |
| Mechanism | git stash push/pop | Zero dependency, atomic, works in all git repos |
| Auto-rollback | No — prompt user instead | Preserves user control, allows inspection of failed state |
| Snapshot naming | `apex-{runID}-{timestamp}` | Easy to filter and correlate with manifest |

## Architecture

### 1. Snapshot Manager (`internal/snapshot`)

New package wrapping git stash operations:

```go
type Snapshot struct {
    ID        string    // stash index (e.g., "stash@{0}")
    Message   string    // "apex-{run_id}-{timestamp}"
    RunID     string
    Timestamp time.Time
}

type Manager struct {
    workDir string
}

func New(workDir string) *Manager
func (m *Manager) Create(runID string) (*Snapshot, error)
func (m *Manager) Restore(runID string) error
func (m *Manager) List() ([]Snapshot, error)
func (m *Manager) Drop(runID string) error
```

**Create**: `git stash push --include-untracked -m "apex-{runID}-{timestamp}"`
- Captures all working tree changes including untracked files
- Returns error if not in a git repo or nothing to stash

**Restore**: Finds the matching stash by message prefix `apex-{runID}`, then `git stash pop stash@{N}`
- Restores working tree to pre-execution state
- Removes the stash entry after successful pop

**List**: `git stash list` filtered by `apex-` prefix, parsed into Snapshot structs

**Drop**: Finds matching stash and runs `git stash drop stash@{N}`

### 2. Integration into run.go

```
Before execution:
  1. Detect git repo (git rev-parse --is-inside-work-tree)
  2. If git repo: snapshot.Create(runID)
  3. If not git repo: skip snapshot (warn)

After execution:
  - success → snapshot.Drop(runID) silently
  - failure/killed → print "Snapshot saved. Restore with: apex snapshot restore {runID}"
```

Snapshot is best-effort: failures to create/drop snapshots are warnings, not errors.

### 3. CLI Commands

**`apex snapshot list`**: Shows all apex snapshots in table format (RunID, Timestamp, Message)

**`apex snapshot restore [run-id]`**: Restores the snapshot for the given run ID via git stash pop

### 4. Testing Strategy

Tests use `git init` in temp directories to create real git repos:
- `TestCreateSnapshot`: Init repo, add files, create snapshot, verify stash exists
- `TestRestoreSnapshot`: Create snapshot, modify files, restore, verify original state
- `TestListSnapshots`: Create multiple snapshots, verify list returns correct entries
- `TestDropSnapshot`: Create and drop, verify stash removed
- `TestCreateInNonGitDir`: Verify graceful error
- `TestCreateWithNoChanges`: Verify handling when working tree is clean

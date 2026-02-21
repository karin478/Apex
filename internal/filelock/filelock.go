// Package filelock provides flock-based file locking with ordering enforcement.
//
// It supports a two-level lock hierarchy: a global lock (order 0) and workspace
// locks (order 1). The Manager enforces that at most one workspace lock is held
// at a time, preventing deadlocks from inconsistent lock ordering.
package filelock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// LockVersion is the current version of the lock metadata format.
const LockVersion = 1

// Sentinel errors returned by lock operations.
var (
	// ErrLocked is returned when another process holds the lock.
	ErrLocked = errors.New("lock is held by another process")
	// ErrOrderViolation is returned when lock ordering would be violated.
	ErrOrderViolation = errors.New("lock ordering violation: cannot hold two workspace locks simultaneously")
)

// Lock represents an acquired file lock.
type Lock struct {
	Path  string
	Order int // 0 = global, 1+ = workspace
	file  *os.File
}

// LockInfo is a JSON-serializable snapshot of a held lock.
type LockInfo struct {
	Path       string `json:"path"`
	Order      int    `json:"order"`
	PID        int    `json:"pid"`
	AcquiredAt string `json:"acquired_at"`
}

// Meta is the on-disk metadata written alongside a lock file.
type Meta struct {
	PID       int    `json:"pid"`
	Timestamp string `json:"timestamp"`
	Order     int    `json:"lock_order_position"`
	Version   int    `json:"lock_version"`
}

// Manager tracks held locks and enforces ordering constraints.
type Manager struct {
	held []*Lock
	mu   sync.Mutex
}

// NewManager returns a new lock Manager with no held locks.
func NewManager() *Manager {
	return &Manager{}
}

// AcquireGlobal acquires the global lock at {baseDir}/apex.lock with order 0.
func (m *Manager) AcquireGlobal(baseDir string) (*Lock, error) {
	lockPath := filepath.Join(baseDir, "apex.lock")
	return m.acquire(lockPath, 0)
}

// AcquireWorkspace acquires a workspace lock at {wsDir}/ws.lock with order 1.
// It returns ErrOrderViolation if a workspace lock (order > 0) is already held.
func (m *Manager) AcquireWorkspace(wsDir string) (*Lock, error) {
	m.mu.Lock()
	for _, l := range m.held {
		if l.Order > 0 {
			m.mu.Unlock()
			return nil, ErrOrderViolation
		}
	}
	m.mu.Unlock()

	lockPath := filepath.Join(wsDir, "ws.lock")
	return m.acquire(lockPath, 1)
}

// Release releases the given lock: removes the flock, closes the file,
// deletes the .meta file, and removes the lock from the held list.
func (m *Manager) Release(lock *Lock) error {
	if lock == nil || lock.file == nil {
		return nil
	}

	fd := int(lock.file.Fd())
	if err := syscall.Flock(fd, syscall.LOCK_UN); err != nil {
		return fmt.Errorf("flock LOCK_UN: %w", err)
	}

	if err := lock.file.Close(); err != nil {
		return fmt.Errorf("close lock file: %w", err)
	}

	// Best-effort removal of meta file.
	_ = os.Remove(lock.Path + ".meta")

	m.mu.Lock()
	defer m.mu.Unlock()
	for i, l := range m.held {
		if l == lock {
			m.held = append(m.held[:i], m.held[i+1:]...)
			break
		}
	}

	return nil
}

// HeldLocks returns information about all currently held locks.
func (m *Manager) HeldLocks() []LockInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	infos := make([]LockInfo, 0, len(m.held))
	for _, l := range m.held {
		meta, err := ReadMeta(l.Path)
		acquired := ""
		pid := os.Getpid()
		if err == nil {
			acquired = meta.Timestamp
			pid = meta.PID
		}
		infos = append(infos, LockInfo{
			Path:       l.Path,
			Order:      l.Order,
			PID:        pid,
			AcquiredAt: acquired,
		})
	}
	return infos
}

// IsStale checks whether the lock at lockPath is stale by reading its .meta
// file and testing whether the recorded PID is still alive.
func IsStale(lockPath string) bool {
	meta, err := ReadMeta(lockPath)
	if err != nil {
		// No meta or unreadable meta: treat as stale.
		return true
	}

	proc, err := os.FindProcess(meta.PID)
	if err != nil {
		return true
	}

	// Signal 0 checks process existence without actually sending a signal.
	err = proc.Signal(syscall.Signal(0))
	return err != nil
}

// ReadMeta reads and parses the .meta JSON file associated with lockPath.
func ReadMeta(lockPath string) (Meta, error) {
	metaPath := lockPath + ".meta"
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return Meta{}, fmt.Errorf("read meta: %w", err)
	}

	var meta Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return Meta{}, fmt.Errorf("unmarshal meta: %w", err)
	}
	return meta, nil
}

// acquire is the internal implementation shared by AcquireGlobal and AcquireWorkspace.
func (m *Manager) acquire(lockPath string, order int) (*Lock, error) {
	dir := filepath.Dir(lockPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir for lock: %w", err)
	}

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	fd := int(f.Fd())
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		// Lock is held by another process. Try to read the holder's PID.
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			holderPID := 0
			if meta, metaErr := ReadMeta(lockPath); metaErr == nil {
				holderPID = meta.PID
			}
			return nil, fmt.Errorf("%w (holder PID: %d)", ErrLocked, holderPID)
		}
		return nil, fmt.Errorf("flock: %w", err)
	}

	// Write meta file.
	meta := Meta{
		PID:       os.Getpid(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Order:     order,
		Version:   LockVersion,
	}
	metaData, err := json.Marshal(meta)
	if err != nil {
		syscall.Flock(fd, syscall.LOCK_UN)
		f.Close()
		return nil, fmt.Errorf("marshal meta: %w", err)
	}
	if err := os.WriteFile(lockPath+".meta", metaData, 0644); err != nil {
		syscall.Flock(fd, syscall.LOCK_UN)
		f.Close()
		return nil, fmt.Errorf("write meta: %w", err)
	}

	lock := &Lock{
		Path:  lockPath,
		Order: order,
		file:  f,
	}

	m.mu.Lock()
	m.held = append(m.held, lock)
	m.mu.Unlock()

	return lock, nil
}

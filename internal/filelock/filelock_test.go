package filelock

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcquireAndReleaseGlobal(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager()

	lock, err := mgr.AcquireGlobal(dir)
	require.NoError(t, err)
	assert.Equal(t, 0, lock.Order)

	// Lock file must exist on disk.
	lockPath := filepath.Join(dir, "apex.lock")
	assert.Equal(t, lockPath, lock.Path)
	_, statErr := os.Stat(lockPath)
	assert.NoError(t, statErr, "lock file should exist on disk")

	// HeldLocks should report exactly one lock.
	held := mgr.HeldLocks()
	assert.Len(t, held, 1)

	// Release and verify held list is empty.
	require.NoError(t, mgr.Release(lock))
	assert.Empty(t, mgr.HeldLocks())
}

func TestAcquireAndReleaseWorkspace(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager()

	lock, err := mgr.AcquireWorkspace(dir)
	require.NoError(t, err)
	assert.Equal(t, 1, lock.Order)

	lockPath := filepath.Join(dir, "ws.lock")
	assert.Equal(t, lockPath, lock.Path)
	_, statErr := os.Stat(lockPath)
	assert.NoError(t, statErr, "workspace lock file should exist on disk")

	require.NoError(t, mgr.Release(lock))
	assert.Empty(t, mgr.HeldLocks())
}

func TestLockOrderingGlobalThenWorkspace(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager()

	globalLock, err := mgr.AcquireGlobal(dir)
	require.NoError(t, err)

	wsDir := filepath.Join(dir, "workspace1")
	wsLock, err := mgr.AcquireWorkspace(wsDir)
	require.NoError(t, err, "acquiring workspace after global should succeed")

	held := mgr.HeldLocks()
	assert.Len(t, held, 2)

	require.NoError(t, mgr.Release(wsLock))
	require.NoError(t, mgr.Release(globalLock))
}

func TestLockOrderingWorkspaceWithoutGlobalOK(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager()

	wsLock, err := mgr.AcquireWorkspace(dir)
	require.NoError(t, err, "acquiring workspace without global should succeed")
	assert.Equal(t, 1, wsLock.Order)

	require.NoError(t, mgr.Release(wsLock))
}

func TestLockOrderingTwoWorkspacesFails(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager()

	ws1Dir := filepath.Join(dir, "ws1")
	ws1Lock, err := mgr.AcquireWorkspace(ws1Dir)
	require.NoError(t, err)

	ws2Dir := filepath.Join(dir, "ws2")
	_, err = mgr.AcquireWorkspace(ws2Dir)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrOrderViolation),
		"second workspace lock should return ErrOrderViolation, got: %v", err)

	require.NoError(t, mgr.Release(ws1Lock))
}

func TestNonBlockingReturnsErrLocked(t *testing.T) {
	dir := t.TempDir()

	mgr1 := NewManager()
	lock1, err := mgr1.AcquireGlobal(dir)
	require.NoError(t, err)

	// A second manager in the same process should fail to acquire the same lock.
	mgr2 := NewManager()
	_, err = mgr2.AcquireGlobal(dir)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrLocked),
		"second manager should get ErrLocked, got: %v", err)

	require.NoError(t, mgr1.Release(lock1))
}

func TestStaleLockDetection(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "stale.lock")

	// Write a meta file with a PID that almost certainly does not exist.
	meta := Meta{
		PID:       999999999,
		Timestamp: "2024-01-01T00:00:00Z",
		Order:     0,
		Version:   LockVersion,
	}
	data, err := json.Marshal(meta)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(lockPath+".meta", data, 0644))

	assert.True(t, IsStale(lockPath), "lock with non-existent PID should be stale")
}

func TestReadMeta(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager()

	lock, err := mgr.AcquireGlobal(dir)
	require.NoError(t, err)

	meta, err := ReadMeta(lock.Path)
	require.NoError(t, err)
	assert.Equal(t, os.Getpid(), meta.PID)
	assert.Equal(t, LockVersion, meta.Version)
	assert.Equal(t, 0, meta.Order)
	assert.NotEmpty(t, meta.Timestamp)

	require.NoError(t, mgr.Release(lock))
}

package killswitch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsActive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "KILL_SWITCH")
	w := New(path)

	assert.False(t, w.IsActive())

	os.WriteFile(path, []byte("test"), 0644)
	assert.True(t, w.IsActive())
}

func TestActivateAndClear(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "KILL_SWITCH")
	w := New(path)

	require.NoError(t, w.Activate("emergency"))
	assert.True(t, w.IsActive())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "emergency", string(data))

	require.NoError(t, w.Clear())
	assert.False(t, w.IsActive())
}

func TestClearWhenNotActive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "KILL_SWITCH")
	w := New(path)

	require.NoError(t, w.Clear())
}

func TestWatchDetectsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "KILL_SWITCH")
	w := New(path)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	watchCtx, watchCancel := w.Watch(ctx)
	defer watchCancel()

	go func() {
		time.Sleep(100 * time.Millisecond)
		os.WriteFile(path, []byte("stop"), 0644)
	}()

	<-watchCtx.Done()
	assert.ErrorIs(t, watchCtx.Err(), context.Canceled)
}

func TestWatchParentCancel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "KILL_SWITCH")
	w := New(path)

	ctx, cancel := context.WithCancel(context.Background())
	watchCtx, watchCancel := w.Watch(ctx)
	defer watchCancel()

	cancel()
	<-watchCtx.Done()
	assert.Error(t, watchCtx.Err())
}

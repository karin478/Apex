package killswitch

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

const DefaultInterval = 200 * time.Millisecond

type Watcher struct {
	path      string
	interval  time.Duration
	triggered atomic.Bool
}

func New(path string) *Watcher {
	return &Watcher{
		path:     path,
		interval: DefaultInterval,
	}
}

func (w *Watcher) Path() string {
	return w.path
}

func (w *Watcher) IsActive() bool {
	_, err := os.Stat(w.path)
	return err == nil
}

// WasTriggered returns true if the watcher cancelled the context due to
// detecting the kill switch file. This is reliable even if the file is
// removed before checking.
func (w *Watcher) WasTriggered() bool {
	return w.triggered.Load()
}

func (w *Watcher) Activate(reason string) error {
	if err := os.MkdirAll(filepath.Dir(w.path), 0755); err != nil {
		return err
	}
	return os.WriteFile(w.path, []byte(reason), 0644)
}

func (w *Watcher) Clear() error {
	err := os.Remove(w.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (w *Watcher) Watch(ctx context.Context) (context.Context, context.CancelFunc) {
	watchCtx, cancel := context.WithCancel(ctx)

	// Immediate check before starting ticker
	if w.IsActive() {
		w.triggered.Store(true)
		cancel()
		return watchCtx, cancel
	}

	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-watchCtx.Done():
				return
			case <-ticker.C:
				if w.IsActive() {
					w.triggered.Store(true)
					cancel()
					return
				}
			}
		}
	}()

	return watchCtx, cancel
}

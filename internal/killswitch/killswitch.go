package killswitch

import (
	"context"
	"os"
	"time"
)

const DefaultInterval = 200 * time.Millisecond

type Watcher struct {
	path     string
	interval time.Duration
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

func (w *Watcher) Activate(reason string) error {
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

	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-watchCtx.Done():
				return
			case <-ticker.C:
				if w.IsActive() {
					cancel()
					return
				}
			}
		}
	}()

	return watchCtx, cancel
}

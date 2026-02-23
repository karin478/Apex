// Package writerq provides a single-writer queue that serializes all DB writes
// through one goroutine, batching operations into transactions for SQLite.
package writerq

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Default configuration constants.
const (
	DefaultQueueSize = 1000 // channel capacity, back-pressure limit
	DefaultFlushMs   = 50   // flush interval in milliseconds
	DefaultMaxBatch  = 100  // max ops per transaction
	MaxCrashRestarts = 3    // max writer goroutine restarts after panics
)

// Op represents a single write operation to be executed inside a batch transaction.
type Op struct {
	SQL    string
	Args   []any
	result chan error
}

// Queue serializes database writes through a single goroutine, batching
// operations into transactions for efficiency and SQLite safety.
type Queue struct {
	db       *sql.DB
	ops      chan Op
	stop     chan struct{}
	done     chan struct{}
	mu       sync.Mutex
	closed   bool
	crashes  int
	killPath string
}

// Option configures a Queue.
type Option func(*Queue)

// WithKillSwitchPath sets the file path to write a kill switch marker
// when the writer goroutine has fatally crashed after MaxCrashRestarts.
func WithKillSwitchPath(path string) Option {
	return func(q *Queue) {
		q.killPath = path
	}
}

// New creates a new Queue backed by the given database and starts the
// background writer goroutine. Use Close to shut it down gracefully.
func New(db *sql.DB, opts ...Option) *Queue {
	q := &Queue{
		db:   db,
		ops:  make(chan Op, DefaultQueueSize),
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	for _, opt := range opts {
		opt(q)
	}
	go q.writerLoop()
	return q
}

// Submit sends a write operation to the queue and blocks until it has been
// executed inside a batch transaction. If the context is cancelled before the
// operation can be submitted, ctx.Err() is returned. If the channel is full
// the call blocks (back-pressure).
func (q *Queue) Submit(ctx context.Context, sqlStr string, args ...any) error {
	op := Op{
		SQL:    sqlStr,
		Args:   args,
		result: make(chan error, 1),
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case q.ops <- op:
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-op.result:
		return err
	}
}

// Close signals the writer goroutine to stop, drains remaining operations,
// and waits for the goroutine to finish. It is safe to call multiple times.
func (q *Queue) Close() error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		<-q.done
		return nil
	}
	q.closed = true
	close(q.stop)
	q.mu.Unlock()

	<-q.done
	return nil
}

// writerLoop is the single-writer goroutine. It collects ops from the channel
// and flushes them in batches, either when the batch is full or on a timer tick.
func (q *Queue) writerLoop() {
	defer close(q.done)

	ticker := time.NewTicker(time.Duration(DefaultFlushMs) * time.Millisecond)
	defer ticker.Stop()

	batch := make([]Op, 0, DefaultMaxBatch)

	for {
		select {
		case op := <-q.ops:
			batch = append(batch, op)
			if len(batch) >= DefaultMaxBatch {
				q.safeExecuteBatch(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				q.safeExecuteBatch(batch)
				batch = batch[:0]
			}

		case <-q.stop:
			// Drain remaining ops from the channel.
			for {
				select {
				case op := <-q.ops:
					batch = append(batch, op)
				default:
					goto drained
				}
			}
		drained:
			if len(batch) > 0 {
				q.safeExecuteBatch(batch)
			}
			return
		}
	}
}

// safeExecuteBatch wraps executeBatch with panic recovery. If the writer
// panics more than MaxCrashRestarts times, it writes a kill switch file
// (if configured) and fails all pending ops.
func (q *Queue) safeExecuteBatch(batch []Op) {
	defer func() {
		if r := recover(); r != nil {
			q.mu.Lock()
			q.crashes++
			crashes := q.crashes
			q.mu.Unlock()

			// Fail all ops in this batch.
			for i := range batch {
				select {
				case batch[i].result <- context.Canceled:
				default:
				}
			}

			if crashes >= MaxCrashRestarts {
				if q.killPath != "" {
					writeKillSwitch(q.killPath)
				}
			}
		}
	}()

	q.executeBatch(batch)
}

// executeBatch runs all ops inside a single database transaction.
// Each operation gets its own error: a failed statement only affects itself,
// not other operations in the batch.
func (q *Queue) executeBatch(batch []Op) {
	tx, err := q.db.Begin()
	if err != nil {
		for i := range batch {
			batch[i].result <- err
		}
		return
	}

	results := make([]error, len(batch))
	anyFailed := false
	for i := range batch {
		if _, execErr := tx.Exec(batch[i].SQL, batch[i].Args...); execErr != nil {
			results[i] = execErr
			anyFailed = true
		}
	}

	if anyFailed {
		_ = tx.Rollback()
		// Retry successful ops in a new transaction, report failed ones
		retryTx, retryErr := q.db.Begin()
		if retryErr != nil {
			for i := range batch {
				if results[i] == nil {
					results[i] = retryErr
				}
				batch[i].result <- results[i]
			}
			return
		}
		for i := range batch {
			if results[i] == nil {
				if _, execErr := retryTx.Exec(batch[i].SQL, batch[i].Args...); execErr != nil {
					results[i] = execErr
				}
			}
		}
		commitErr := retryTx.Commit()
		for i := range batch {
			if results[i] == nil {
				batch[i].result <- commitErr
			} else {
				batch[i].result <- results[i]
			}
		}
		return
	}

	commitErr := tx.Commit()
	for i := range batch {
		batch[i].result <- commitErr
	}
}

// writeKillSwitch creates a marker file indicating the writer queue has
// fatally crashed after the maximum number of restarts.
func writeKillSwitch(path string) {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	_ = os.WriteFile(path, []byte("writerq: fatal crash after max restarts"), 0644)
}

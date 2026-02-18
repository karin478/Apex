package pool

import (
	"context"
	"sync"
	"time"

	"github.com/lyndonlyu/apex/internal/dag"
)

// Runner executes a single task and returns the result.
type Runner interface {
	RunTask(ctx context.Context, task string) (string, error)
}

// Pool manages concurrent execution of DAG nodes using a bounded worker pool.
type Pool struct {
	maxWorkers int
	runner     Runner
}

// New creates a new Pool with the given concurrency limit and task runner.
// If maxWorkers is <= 0, it defaults to 4.
func New(maxWorkers int, runner Runner) *Pool {
	if maxWorkers <= 0 {
		maxWorkers = 4
	}
	return &Pool{
		maxWorkers: maxWorkers,
		runner:     runner,
	}
}

// Execute runs all nodes in the DAG concurrently, respecting dependency order
// and the pool's concurrency limit. It returns an error only if the context
// is cancelled; individual task failures are recorded in the DAG via MarkFailed.
func (p *Pool) Execute(ctx context.Context, d *dag.DAG) error {
	for !d.IsComplete() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		ready := d.ReadyNodes()
		if len(ready) == 0 {
			if d.IsComplete() {
				break
			}
			// Safety yield to prevent tight loop in unexpected states.
			time.Sleep(5 * time.Millisecond)
			continue
		}

		sem := make(chan struct{}, p.maxWorkers)
		var wg sync.WaitGroup

		for _, node := range ready {
			d.MarkRunning(node.ID)
			sem <- struct{}{}
			wg.Add(1)

			go func(n *dag.Node) {
				defer wg.Done()
				defer func() { <-sem }()

				result, err := p.runner.RunTask(ctx, n.Task)
				if err != nil {
					d.MarkFailed(n.ID, err.Error())
					return
				}
				d.MarkCompleted(n.ID, result)
			}(node)
		}

		wg.Wait()

		// After workers finish, check if context was cancelled during execution.
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	return nil
}

package pool

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lyndonlyu/apex/internal/dag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRunner struct {
	callCount atomic.Int32
	delay     time.Duration
}

func (m *mockRunner) RunTask(ctx context.Context, task string) (string, error) {
	m.callCount.Add(1)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "result for: " + task, nil
}

type failRunner struct {
	failKeywords []string
}

func (f *failRunner) RunTask(ctx context.Context, task string) (string, error) {
	for _, kw := range f.failKeywords {
		if strings.Contains(task, kw) {
			return "", fmt.Errorf("task failed: %s", task)
		}
	}
	return "ok", nil
}

func TestNewPool(t *testing.T) {
	runner := &mockRunner{}
	p := New(4, runner)
	assert.NotNil(t, p)
}

func TestExecuteLinearDAG(t *testing.T) {
	nodes := []dag.NodeSpec{
		{ID: "a", Task: "first", Depends: []string{}},
		{ID: "b", Task: "second", Depends: []string{"a"}},
		{ID: "c", Task: "third", Depends: []string{"b"}},
	}
	d, _ := dag.New(nodes)
	runner := &mockRunner{}
	p := New(4, runner)

	err := p.Execute(context.Background(), d)
	require.NoError(t, err)

	assert.Equal(t, dag.Completed, d.Nodes["a"].Status)
	assert.Equal(t, dag.Completed, d.Nodes["b"].Status)
	assert.Equal(t, dag.Completed, d.Nodes["c"].Status)
	assert.Equal(t, int32(3), runner.callCount.Load())
}

func TestExecuteParallelDAG(t *testing.T) {
	nodes := []dag.NodeSpec{
		{ID: "a", Task: "parallel 1", Depends: []string{}},
		{ID: "b", Task: "parallel 2", Depends: []string{}},
		{ID: "c", Task: "parallel 3", Depends: []string{}},
		{ID: "d", Task: "final", Depends: []string{"a", "b", "c"}},
	}
	d, _ := dag.New(nodes)

	runner := &mockRunner{delay: 50 * time.Millisecond}
	p := New(4, runner)

	start := time.Now()
	err := p.Execute(context.Background(), d)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.True(t, d.IsComplete())
	assert.False(t, d.HasFailure())
	assert.True(t, duration < 300*time.Millisecond, "parallel execution too slow: %v", duration)
}

func TestExecuteWithFailure(t *testing.T) {
	nodes := []dag.NodeSpec{
		{ID: "a", Task: "will fail", Depends: []string{}},
		{ID: "b", Task: "depends on a", Depends: []string{"a"}},
		{ID: "c", Task: "independent", Depends: []string{}},
	}
	d, _ := dag.New(nodes)

	runner := &failRunner{failKeywords: []string{"will fail"}}
	p := New(4, runner)

	err := p.Execute(context.Background(), d)
	assert.NoError(t, err)
	assert.True(t, d.HasFailure())
	assert.Equal(t, dag.Failed, d.Nodes["a"].Status)
	assert.Equal(t, dag.Failed, d.Nodes["b"].Status) // cascade
	assert.Equal(t, dag.Completed, d.Nodes["c"].Status) // independent
}

func TestExecuteSingleNode(t *testing.T) {
	nodes := []dag.NodeSpec{
		{ID: "only", Task: "single task", Depends: []string{}},
	}
	d, _ := dag.New(nodes)
	runner := &mockRunner{}
	p := New(1, runner)

	err := p.Execute(context.Background(), d)
	require.NoError(t, err)
	assert.Equal(t, dag.Completed, d.Nodes["only"].Status)
}

func TestExecuteContextCancel(t *testing.T) {
	nodes := []dag.NodeSpec{
		{ID: "slow", Task: "slow task", Depends: []string{}},
	}
	d, _ := dag.New(nodes)
	runner := &mockRunner{delay: 5 * time.Second}
	p := New(1, runner)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := p.Execute(ctx, d)
	assert.Error(t, err)
}

package aggregator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Summarize
// ---------------------------------------------------------------------------

func TestSummarize(t *testing.T) {
	p := NewPipeline(StrategySummarize)
	p.Add(Input{NodeID: "node-1", Content: "Hello from node 1"})
	p.Add(Input{NodeID: "node-2", Content: "Hello from node 2"})

	res, err := p.Execute()
	require.NoError(t, err)

	assert.Equal(t, StrategySummarize, res.Strategy)
	assert.Equal(t, 2, res.InputCount)
	assert.Nil(t, res.Data)
	assert.Contains(t, res.Output, "--- [node-1] ---")
	assert.Contains(t, res.Output, "Hello from node 1")
	assert.Contains(t, res.Output, "--- [node-2] ---")
	assert.Contains(t, res.Output, "Hello from node 2")
	assert.False(t, res.CreatedAt.IsZero())
}

func TestSummarizeEmpty(t *testing.T) {
	p := NewPipeline(StrategySummarize)

	res, err := p.Execute()
	assert.Nil(t, res)
	assert.EqualError(t, err, "aggregator: no inputs")
}

// ---------------------------------------------------------------------------
// Merge
// ---------------------------------------------------------------------------

func TestMerge(t *testing.T) {
	p := NewPipeline(StrategyMerge)
	p.Add(Input{
		NodeID: "n1",
		Data: []interface{}{
			map[string]interface{}{"id": "a", "name": "alpha"},
			map[string]interface{}{"id": "b", "name": "bravo"},
		},
	})
	p.Add(Input{
		NodeID: "n2",
		Data: []interface{}{
			map[string]interface{}{"id": "c", "name": "charlie"},
		},
	})

	res, err := p.Execute()
	require.NoError(t, err)

	assert.Equal(t, StrategyMerge, res.Strategy)
	assert.Equal(t, 2, res.InputCount)

	merged, ok := res.Data.([]interface{})
	require.True(t, ok)
	assert.Len(t, merged, 3)
	assert.Equal(t, "Merged 3 items from 2 inputs", res.Output)
	assert.False(t, res.CreatedAt.IsZero())
}

func TestMergeDedup(t *testing.T) {
	p := NewPipeline(StrategyMerge)
	p.SetMergeOptions(MergeOptions{KeyField: "id"})

	p.Add(Input{
		NodeID: "n1",
		Data: []interface{}{
			map[string]interface{}{"id": "a", "name": "alpha-old"},
			map[string]interface{}{"id": "b", "name": "bravo"},
		},
	})
	p.Add(Input{
		NodeID: "n2",
		Data: []interface{}{
			map[string]interface{}{"id": "a", "name": "alpha-new"},
			map[string]interface{}{"id": "c", "name": "charlie"},
		},
	})

	res, err := p.Execute()
	require.NoError(t, err)

	merged, ok := res.Data.([]interface{})
	require.True(t, ok)
	assert.Len(t, merged, 3, "dedup should collapse duplicates (last wins)")

	// Verify "a" kept the newer version.
	found := false
	for _, item := range merged {
		m := item.(map[string]interface{})
		if m["id"] == "a" {
			assert.Equal(t, "alpha-new", m["name"])
			found = true
		}
	}
	assert.True(t, found, "item with id=a should exist")
}

func TestMergeSort(t *testing.T) {
	p := NewPipeline(StrategyMerge)
	p.SetMergeOptions(MergeOptions{SortField: "name"})

	p.Add(Input{
		NodeID: "n1",
		Data: []interface{}{
			map[string]interface{}{"name": "charlie"},
			map[string]interface{}{"name": "alpha"},
		},
	})
	p.Add(Input{
		NodeID: "n2",
		Data: []interface{}{
			map[string]interface{}{"name": "bravo"},
		},
	})

	res, err := p.Execute()
	require.NoError(t, err)

	merged, ok := res.Data.([]interface{})
	require.True(t, ok)
	require.Len(t, merged, 3)

	// Verify ascending order by name.
	names := make([]string, len(merged))
	for i, item := range merged {
		m := item.(map[string]interface{})
		names[i] = m["name"].(string)
	}
	assert.Equal(t, []string{"alpha", "bravo", "charlie"}, names)
}

// ---------------------------------------------------------------------------
// Reduce
// ---------------------------------------------------------------------------

func TestReduce(t *testing.T) {
	p := NewPipeline(StrategyReduce)
	p.Add(Input{NodeID: "n1", Data: float64(10)})
	p.Add(Input{NodeID: "n2", Data: float64(20)})
	p.Add(Input{NodeID: "n3", Data: float64(30)})
	p.Add(Input{NodeID: "n4", Data: float64(40)})

	res, err := p.Execute()
	require.NoError(t, err)

	assert.Equal(t, StrategyReduce, res.Strategy)
	assert.Equal(t, 4, res.InputCount)
	assert.False(t, res.CreatedAt.IsZero())

	stats, ok := res.Data.(ReduceStats)
	require.True(t, ok)

	assert.Equal(t, 4, stats.Count)
	assert.Equal(t, 100.0, stats.Sum)
	assert.Equal(t, 10.0, stats.Min)
	assert.Equal(t, 40.0, stats.Max)
	assert.Equal(t, 25.0, stats.Avg)

	assert.Equal(t, "Count: 4, Sum: 100.00, Min: 10.00, Max: 40.00, Avg: 25.00", res.Output)
}

func TestReduceEmpty(t *testing.T) {
	p := NewPipeline(StrategyReduce)

	res, err := p.Execute()
	assert.Nil(t, res)
	assert.EqualError(t, err, "aggregator: no inputs")
}

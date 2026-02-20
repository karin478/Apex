package aggregator

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Strategy constants
// ---------------------------------------------------------------------------

// Strategy defines the aggregation method used by a Pipeline.
type Strategy string

const (
	StrategySummarize Strategy = "summarize"
	StrategyMerge     Strategy = "merge"
	StrategyReduce    Strategy = "reduce"
)

// ---------------------------------------------------------------------------
// Domain structs
// ---------------------------------------------------------------------------

// Input represents a single data contribution from a node.
type Input struct {
	NodeID  string      `json:"node_id"`
	Content string      `json:"content"`
	Data    interface{} `json:"data"`
}

// MergeOptions configures deduplication and sorting for the merge strategy.
type MergeOptions struct {
	KeyField  string `json:"key_field"`
	SortField string `json:"sort_field"`
}

// ReduceStats holds the computed statistics from a reduce aggregation.
type ReduceStats struct {
	Count int     `json:"count"`
	Sum   float64 `json:"sum"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Avg   float64 `json:"avg"`
}

// Result is the output of a pipeline execution.
type Result struct {
	Strategy   Strategy    `json:"strategy"`
	Output     string      `json:"output"`
	Data       interface{} `json:"data,omitempty"`
	InputCount int         `json:"input_count"`
	CreatedAt  time.Time   `json:"created_at"`
}

// ---------------------------------------------------------------------------
// Pipeline
// ---------------------------------------------------------------------------

// Pipeline aggregates multiple inputs using a chosen strategy.
// Pipeline is not safe for concurrent use; all Add calls must complete before Execute is called.
type Pipeline struct {
	strategy Strategy
	inputs   []Input
	mergeOpt *MergeOptions
}

// NewPipeline creates a pipeline with the given strategy.
func NewPipeline(strategy Strategy) *Pipeline {
	return &Pipeline{
		strategy: strategy,
	}
}

// SetMergeOptions sets merge options for the merge strategy.
func (p *Pipeline) SetMergeOptions(opts MergeOptions) {
	p.mergeOpt = &opts
}

// Add appends an input to the pipeline.
func (p *Pipeline) Add(input Input) {
	p.inputs = append(p.inputs, input)
}

// Execute dispatches to the strategy-specific internal function and returns
// the aggregated result.
func (p *Pipeline) Execute() (*Result, error) {
	if len(p.inputs) == 0 {
		return nil, errors.New("aggregator: no inputs")
	}

	switch p.strategy {
	case StrategySummarize:
		return p.executeSummarize()
	case StrategyMerge:
		return p.executeMerge()
	case StrategyReduce:
		return p.executeReduce()
	default:
		return nil, fmt.Errorf("aggregator: unknown strategy: %s", p.strategy)
	}
}

// ---------------------------------------------------------------------------
// Summarize
// ---------------------------------------------------------------------------

func (p *Pipeline) executeSummarize() (*Result, error) {
	var parts []string
	for _, in := range p.inputs {
		if in.Content == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("--- [%s] ---\n%s", in.NodeID, in.Content))
	}

	return &Result{
		Strategy:   StrategySummarize,
		Output:     strings.Join(parts, "\n"),
		Data:       nil,
		InputCount: len(p.inputs),
		CreatedAt:  time.Now().UTC(),
	}, nil
}

// ---------------------------------------------------------------------------
// Structured Merge
// ---------------------------------------------------------------------------

func (p *Pipeline) executeMerge() (*Result, error) {
	var merged []interface{}

	for i, in := range p.inputs {
		arr, ok := in.Data.([]interface{})
		if !ok {
			return nil, fmt.Errorf("aggregator: input %d data is not an array", i)
		}
		merged = append(merged, arr...)
	}

	// Dedup by KeyField (last wins).
	if p.mergeOpt != nil && p.mergeOpt.KeyField != "" {
		seen := make(map[interface{}]int) // key value -> index in deduped
		var deduped []interface{}
		for _, item := range merged {
			m, ok := item.(map[string]interface{})
			if !ok {
				deduped = append(deduped, item)
				continue
			}
			keyVal, exists := m[p.mergeOpt.KeyField]
			if !exists {
				deduped = append(deduped, item)
				continue
			}
			if idx, found := seen[keyVal]; found {
				deduped[idx] = item // last wins
			} else {
				seen[keyVal] = len(deduped)
				deduped = append(deduped, item)
			}
		}
		merged = deduped
	}

	// Sort by SortField ascending. Note: values are compared as strings via
	// fmt.Sprintf("%v", ...), so numeric fields will sort lexicographically
	// (e.g. "9" > "10") rather than by numeric value.
	if p.mergeOpt != nil && p.mergeOpt.SortField != "" {
		sortField := p.mergeOpt.SortField
		sort.SliceStable(merged, func(i, j int) bool {
			mi, oki := merged[i].(map[string]interface{})
			mj, okj := merged[j].(map[string]interface{})
			if !oki || !okj {
				return false
			}
			vi := fmt.Sprintf("%v", mi[sortField])
			vj := fmt.Sprintf("%v", mj[sortField])
			return vi < vj
		})
	}

	return &Result{
		Strategy:   StrategyMerge,
		Output:     fmt.Sprintf("Merged %d items from %d inputs", len(merged), len(p.inputs)),
		Data:       merged,
		InputCount: len(p.inputs),
		CreatedAt:  time.Now().UTC(),
	}, nil
}

// ---------------------------------------------------------------------------
// Statistical Reduce
// ---------------------------------------------------------------------------

func (p *Pipeline) executeReduce() (*Result, error) {
	values := make([]float64, 0, len(p.inputs))

	for i, in := range p.inputs {
		v, ok := toFloat64(in.Data)
		if !ok {
			return nil, fmt.Errorf("aggregator: input %d data is not numeric", i)
		}
		values = append(values, v)
	}

	stats := ReduceStats{
		Count: len(values),
		Min:   math.Inf(1),
		Max:   math.Inf(-1),
	}
	for _, v := range values {
		stats.Sum += v
		if v < stats.Min {
			stats.Min = v
		}
		if v > stats.Max {
			stats.Max = v
		}
	}
	stats.Avg = stats.Sum / float64(stats.Count)

	return &Result{
		Strategy:   StrategyReduce,
		Output:     fmt.Sprintf("Count: %d, Sum: %.2f, Min: %.2f, Max: %.2f, Avg: %.2f", stats.Count, stats.Sum, stats.Min, stats.Max, stats.Avg),
		Data:       stats,
		InputCount: len(p.inputs),
		CreatedAt:  time.Now().UTC(),
	}, nil
}

// toFloat64 attempts to convert an interface{} value to float64.
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	default:
		return 0, false
	}
}

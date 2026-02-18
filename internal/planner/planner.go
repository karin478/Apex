package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/lyndonlyu/apex/internal/dag"
	"github.com/lyndonlyu/apex/internal/executor"
)

// complexPattern matches multi-step conjunctions that indicate a task
// should be decomposed rather than handled as a single unit.
var complexPattern = regexp.MustCompile(`(?i)\b(and then|then|after that|first .+ then|followed by|next|finally|step \d|phase \d)\b`)

// IsSimpleTask returns true if the task is short (under 50 words) and does
// not contain multi-step language patterns. Simple tasks are executed as a
// single node without LLM decomposition.
func IsSimpleTask(task string) bool {
	words := strings.Fields(task)
	if len(words) > 50 {
		return false
	}
	return !complexPattern.MatchString(task)
}

// BuildPlannerPrompt constructs the system prompt sent to the LLM for
// decomposing a complex task into a DAG of subtasks.
func BuildPlannerPrompt(task string) string {
	return fmt.Sprintf(`You are a task planner. Decompose the following task into subtasks.

Return ONLY a JSON array. Each element has:
- "id": short unique identifier (e.g. "step1", "analyze", "test")
- "task": clear description of what to do
- "depends": array of IDs this step depends on (empty if none)

Rules:
- Maximize parallelism: independent steps should not depend on each other
- Keep steps focused: each step should be one clear action
- Minimum steps needed, don't over-decompose
- Return valid JSON only, no markdown, no explanation

Task: %s`, task)
}

// ParseNodes parses raw LLM output into a slice of dag.NodeSpec.
// It handles both bare JSON arrays and JSON wrapped in markdown code fences.
// Returns an error if the JSON is invalid or the resulting array is empty.
func ParseNodes(raw string) ([]dag.NodeSpec, error) {
	cleaned := raw

	// Strip markdown code fences if present.
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(.*?)\\n?```")
	if matches := re.FindStringSubmatch(raw); len(matches) > 1 {
		cleaned = matches[1]
	}
	cleaned = strings.TrimSpace(cleaned)

	var nodes []dag.NodeSpec
	if err := json.Unmarshal([]byte(cleaned), &nodes); err != nil {
		return nil, fmt.Errorf("failed to parse planner output: %w", err)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("planner returned empty node list")
	}

	return nodes, nil
}

// SingleNodeFallback creates a one-element NodeSpec slice that wraps the
// entire task as a single node. Used when a task is simple or when LLM
// decomposition fails.
func SingleNodeFallback(task string) []dag.NodeSpec {
	return []dag.NodeSpec{
		{ID: "task", Task: task, Depends: []string{}},
	}
}

// Plan decomposes a task into DAG node specifications. For simple tasks it
// returns a single-node fallback immediately. For complex tasks it calls
// the LLM via the executor to produce a multi-step plan. If the LLM call
// or JSON parsing fails, it gracefully falls back to a single node.
func Plan(ctx context.Context, exec *executor.Executor, task string, model string, timeout int) ([]dag.NodeSpec, error) {
	if IsSimpleTask(task) {
		return SingleNodeFallback(task), nil
	}

	planExec := executor.New(executor.Options{
		Model:   model,
		Effort:  "high",
		Timeout: time.Duration(timeout) * time.Second,
	})

	prompt := BuildPlannerPrompt(task)
	result, err := planExec.Run(ctx, prompt)
	if err != nil {
		return SingleNodeFallback(task), nil
	}

	nodes, err := ParseNodes(result.Output)
	if err != nil {
		return SingleNodeFallback(task), nil
	}

	return nodes, nil
}

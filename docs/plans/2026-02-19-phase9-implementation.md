# Phase 9: Dry-Run Mode Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `--dry-run` flag to `apex run` that shows risk, DAG plan, context token usage, and cost estimate without executing.

**Architecture:** New `internal/cost` package for token-to-cost estimation. `--dry-run` flag on `runCmd` short-circuits after context enrichment to print a report. No snapshot/execution/audit/manifest.

**Tech Stack:** Go stdlib, Cobra flags, Testify

---

### Task 1: Cost Estimator Package

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/cost/estimator.go`
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/cost/estimator_test.go`

**Step 1: Write the failing tests**

Create `estimator_test.go`:

```go
package cost

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimateRun(t *testing.T) {
	tasks := map[string]string{
		"1": "Write a function that parses JSON input and returns structured data",
		"2": "Add unit tests for the parser",
	}
	est := EstimateRun(tasks, "claude-sonnet-4-20250514")
	assert.Greater(t, est.InputTokens, 0)
	assert.Greater(t, est.OutputTokens, 0)
	assert.Greater(t, est.TotalCost, 0.0)
	assert.Equal(t, "claude-sonnet-4-20250514", est.Model)
	assert.Equal(t, 2, est.NodeCount)
}

func TestEstimateRunOpus(t *testing.T) {
	tasks := map[string]string{
		"1": "some task text here",
	}
	sonnet := EstimateRun(tasks, "claude-sonnet-4-20250514")
	opus := EstimateRun(tasks, "claude-opus-4-20250514")
	// Opus should cost more than sonnet for same input
	assert.Greater(t, opus.TotalCost, sonnet.TotalCost)
}

func TestEstimateRunEmpty(t *testing.T) {
	est := EstimateRun(map[string]string{}, "claude-sonnet-4-20250514")
	assert.Equal(t, 0, est.InputTokens)
	assert.Equal(t, 0.0, est.TotalCost)
	assert.Equal(t, 0, est.NodeCount)
}

func TestFormatCost(t *testing.T) {
	assert.Equal(t, "~$0.01", FormatCost(0.005))
	assert.Equal(t, "~$0.12", FormatCost(0.123))
	assert.Equal(t, "<$0.01", FormatCost(0.001))
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/cost/ -v`

Expected: FAIL (package doesn't exist)

**Step 3: Implement estimator.go**

Create `estimator.go`:

```go
package cost

import (
	"fmt"
	"strings"
)

type Estimate struct {
	InputTokens  int
	OutputTokens int
	TotalCost    float64
	Model        string
	NodeCount    int
}

// modelPricing stores per-1M-token prices: [input, output]
var modelPricing = map[string][2]float64{
	"sonnet": {3.0, 15.0},
	"opus":   {15.0, 75.0},
	"haiku":  {0.25, 1.25},
}

func EstimateRun(enrichedTasks map[string]string, model string) *Estimate {
	if len(enrichedTasks) == 0 {
		return &Estimate{Model: model}
	}

	totalInput := 0
	for _, text := range enrichedTasks {
		totalInput += estimateTokens(text)
	}

	// Estimate output as 2x input (typical for code generation)
	totalOutput := totalInput * 2

	inputPrice, outputPrice := lookupPricing(model)
	totalCost := float64(totalInput)/1_000_000*inputPrice + float64(totalOutput)/1_000_000*outputPrice

	return &Estimate{
		InputTokens:  totalInput,
		OutputTokens: totalOutput,
		TotalCost:    totalCost,
		Model:        model,
		NodeCount:    len(enrichedTasks),
	}
}

func FormatCost(cost float64) string {
	if cost < 0.005 {
		return "<$0.01"
	}
	return fmt.Sprintf("~$%.2f", cost)
}

func lookupPricing(model string) (float64, float64) {
	lower := strings.ToLower(model)
	for key, prices := range modelPricing {
		if strings.Contains(lower, key) {
			return prices[0], prices[1]
		}
	}
	// Default to sonnet pricing
	return 3.0, 15.0
}

func estimateTokens(text string) int {
	runes := []rune(text)
	if len(runes) == 0 {
		return 0
	}
	est := len(runes) / 3
	if est == 0 {
		est = 1
	}
	return est
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./internal/cost/ -v`

Expected: ALL PASS (4 tests)

**Step 5: Commit**

```bash
git add internal/cost/estimator.go internal/cost/estimator_test.go
git commit -m "feat(cost): add token-to-cost estimator for dry-run mode"
```

---

### Task 2: Add --dry-run Flag and Report

**Files:**
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/run.go`

**Step 1: Add flag and dry-run report logic**

Add a package-level var and register the flag:

```go
var dryRun bool

func init() {
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show execution plan and cost estimate without running")
}
```

Add import for `"github.com/lyndonlyu/apex/internal/cost"`.

After the context enrichment loop (after line 132 in current run.go), add the dry-run exit point:

```go
	// Dry-run: print report and exit
	if dryRun {
		fmt.Println("\n[DRY RUN]")
		fmt.Printf("Risk: %s", risk)
		if risk.ShouldRequireApproval() {
			fmt.Print(" (approval required for execution)")
		}
		fmt.Println()

		fmt.Printf("\nPlan: %d steps\n", len(d.Nodes))
		for i, n := range d.NodeSlice() {
			nodeRisk := governance.Classify(n.Task)
			fmt.Fprintf(os.Stdout, "  [%d] %-40s %s\n", i+1, n.Task, nodeRisk)
		}

		fmt.Println("\nContext:")
		totalTokens := 0
		for i, n := range d.NodeSlice() {
			if enriched, ok := enrichedTasks[n.ID]; ok {
				tokens := len([]rune(enriched)) / 3
				if tokens == 0 {
					tokens = 1
				}
				totalTokens += tokens
				fmt.Fprintf(os.Stdout, "  [%d] %d tokens\n", i+1, tokens)
			}
		}
		fmt.Fprintf(os.Stdout, "  Budget: %d/%d (%d%%)\n", totalTokens, cfg.Context.TokenBudget,
			totalTokens*100/max(cfg.Context.TokenBudget, 1))

		est := cost.EstimateRun(enrichedTasks, cfg.Claude.Model)
		fmt.Printf("\nCost estimate: %s (%d calls, %s)\n", cost.FormatCost(est.TotalCost), est.NodeCount, est.Model)

		fmt.Println("\nNo changes made. Run without --dry-run to execute.")
		return nil
	}
```

Note: Restore original task names before dry-run report for display. Move the `origTasks` restore block before the dry-run check, or use origTasks for display in the report.

Actually, simpler approach: do dry-run report BEFORE the task swap (move it before line 134). The `enrichedTasks` map has the enriched text for token counting, and `d.Nodes` still have original task names for display.

**Step 2: Verify build**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build -o bin/apex ./cmd/apex/`

Expected: Build succeeds

**Step 3: Verify flag**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && ./bin/apex run --help`

Expected: Shows `--dry-run` flag in output

**Step 4: Commit**

```bash
git add cmd/apex/run.go
git commit -m "feat: add --dry-run flag to apex run for execution preview"
```

---

### Task 3: E2E Verification

**Step 1: Run all tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./... -v 2>&1 | tail -30`

Expected: ALL packages PASS (18 packages: 17 existing + 1 new cost)

**Step 2: Build**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build -o bin/apex ./cmd/apex/`

Expected: Build succeeds

**Step 3: Verify CLI**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
./bin/apex run --help
./bin/apex --help
```

Expected: `--dry-run` flag visible in run help, all commands listed

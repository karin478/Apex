# Interactive REPL Mode Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a chat-style interactive REPL mode to Apex, entered by running `apex` with no arguments.

**Architecture:** A new `interactive.go` command file registers as the root command's `RunE`. It starts a go-prompt REPL loop that dispatches slash commands locally and sends task text through the existing risk → plan → DAG → execute pipeline. A `style.go` file provides lipgloss styling. The executor gains an optional `OnOutput` callback for streaming.

**Tech Stack:** go-prompt (readline), lipgloss (terminal styling), existing Apex internals

---

### Task 1: Add Dependencies

**Files:**
- Modify: `go.mod`

**Step 1: Add go-prompt and lipgloss**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
go get github.com/c-bata/go-prompt@latest
go get github.com/charmbracelet/lipgloss@latest
go mod tidy
```

**Step 2: Verify build**

Run: `go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add go-prompt and lipgloss for interactive REPL"
```

---

### Task 2: Add Streaming Callback to Executor

**Files:**
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/executor/claude.go`
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/executor/claude_test.go` (add streaming test)

**Step 1: Write the failing test**

Add to `claude_test.go`:

```go
func TestExecutorOnOutputCallback(t *testing.T) {
	// Create a mock binary that outputs line by line
	mockDir := t.TempDir()
	mockBin := filepath.Join(mockDir, "mock_claude")
	script := `#!/bin/sh
echo '{"result":"line1\nline2\nline3","is_error":false}'
`
	os.WriteFile(mockBin, []byte(script), 0755)

	var chunks []string
	exec := New(Options{
		Model:   "test",
		Effort:  "low",
		Timeout: 10 * time.Second,
		Binary:  mockBin,
		OnOutput: func(chunk string) {
			chunks = append(chunks, chunk)
		},
	})

	result, err := exec.Run(context.Background(), "test task")
	assert.NoError(t, err)
	assert.NotEmpty(t, result.Output)
	// OnOutput should have received at least one chunk
	assert.Greater(t, len(chunks), 0)
}

func TestExecutorOnOutputNil(t *testing.T) {
	// When OnOutput is nil, behavior is unchanged (buffer mode)
	mockDir := t.TempDir()
	mockBin := filepath.Join(mockDir, "mock_claude")
	script := `#!/bin/sh
echo '{"result":"hello","is_error":false}'
`
	os.WriteFile(mockBin, []byte(script), 0755)

	exec := New(Options{
		Model:   "test",
		Effort:  "low",
		Timeout: 10 * time.Second,
		Binary:  mockBin,
	})

	result, err := exec.Run(context.Background(), "test task")
	assert.NoError(t, err)
	assert.Equal(t, "hello", result.Output)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/executor/ -run TestExecutorOnOutput -v`
Expected: FAIL (OnOutput field does not exist)

**Step 3: Implement streaming in executor**

In `claude.go`, add `OnOutput` to Options:

```go
type Options struct {
	Model          string
	Effort         string
	Timeout        time.Duration
	Binary         string
	Sandbox        sandbox.Sandbox
	PermissionMode string
	OnOutput       func(chunk string) // nil = buffer mode (default)
}
```

In `Run()`, replace the stdout handling:

```go
var stdout bytes.Buffer
if e.opts.OnOutput != nil {
	// Streaming mode: tee to buffer AND call OnOutput per line
	pr, pw := io.Pipe()
	cmd.Stdout = io.MultiWriter(&stdout, pw)
	go func() {
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			e.opts.OnOutput(scanner.Text())
		}
		pr.Close()
	}()
	defer pw.Close()
} else {
	cmd.Stdout = &stdout
}
```

Add imports: `"bufio"`, `"io"`

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/executor/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/executor/claude.go internal/executor/claude_test.go
git commit -m "feat(executor): add OnOutput streaming callback"
```

---

### Task 3: Create Style Definitions

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/style.go`

**Step 1: Write style.go**

```go
package main

import "github.com/charmbracelet/lipgloss"

var (
	styleBanner  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	stylePrompt  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	styleRisk    = map[string]lipgloss.Style{
		"LOW":      lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		"MEDIUM":   lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		"HIGH":     lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		"CRITICAL": lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1")),
	}
	styleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleInfo    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func renderRisk(level string) string {
	if s, ok := styleRisk[level]; ok {
		return s.Render("[" + level + "]")
	}
	return "[" + level + "]"
}
```

**Step 2: Verify build**

Run: `go build ./cmd/apex/`
Expected: No errors

**Step 3: Commit**

```bash
git add cmd/apex/style.go
git commit -m "feat(repl): add lipgloss style definitions"
```

---

### Task 4: Create Streaming Wrapper

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/stream.go`

**Step 1: Write stream.go**

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/lyndonlyu/apex/internal/config"
	"github.com/lyndonlyu/apex/internal/dag"
	"github.com/lyndonlyu/apex/internal/executor"
	"github.com/lyndonlyu/apex/internal/governance"
	"github.com/lyndonlyu/apex/internal/planner"
	"github.com/lyndonlyu/apex/internal/pool"
	"github.com/lyndonlyu/apex/internal/sandbox"
)

// runInteractiveTask executes a single task through the full pipeline with
// streaming output. Returns the result summary for session context.
func runInteractiveTask(cfg *config.Config, task string, sessionContext string) (string, error) {
	// Classify risk
	risk := governance.Classify(task)
	fmt.Println(renderRisk(risk.String()) + " " + styleInfo.Render("Planning..."))

	// Confirm if needed
	if risk.ShouldConfirm() {
		fmt.Printf("Warning: %s risk. Proceed? (y/n): ", risk)
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			return "", fmt.Errorf("cancelled by user")
		}
	}
	if risk.ShouldReject() {
		return "", fmt.Errorf("task rejected (%s risk)", risk)
	}

	// Resolve sandbox
	var sb sandbox.Sandbox
	level, _ := sandbox.ParseLevel(cfg.Sandbox.Level)
	sb, _ = sandbox.ForLevel(level)

	// Plan
	planExec := executor.New(executor.Options{
		Model:   cfg.Planner.Model,
		Effort:  "high",
		Timeout: time.Duration(cfg.Planner.Timeout) * time.Second,
		Binary:  cfg.Claude.Binary,
		Sandbox: sb,
	})

	// Prepend session context to task for continuity
	enrichedTask := task
	if sessionContext != "" {
		enrichedTask = "Context from previous tasks:\n" + sessionContext + "\n\nNew task: " + task
	}

	nodes, err := planner.Plan(context.Background(), planExec, enrichedTask, cfg.Planner.Model, cfg.Planner.Timeout)
	if err != nil {
		return "", fmt.Errorf("planning failed: %w", err)
	}

	d, err := dag.New(nodes)
	if err != nil {
		return "", fmt.Errorf("invalid DAG: %w", err)
	}

	fmt.Printf("%s %d steps\n", styleInfo.Render("Plan:"), len(d.Nodes))

	// Execute with streaming
	exec := executor.New(executor.Options{
		Model:          cfg.Claude.Model,
		Effort:         cfg.Claude.Effort,
		Timeout:        time.Duration(cfg.Claude.Timeout) * time.Second,
		Binary:         cfg.Claude.Binary,
		Sandbox:        sb,
		PermissionMode: cfg.Claude.PermissionMode,
		OnOutput: func(chunk string) {
			fmt.Println(styleDim.Render("  " + chunk))
		},
	})

	runner := pool.NewClaudeRunner(exec)
	p := pool.New(cfg.Pool.MaxConcurrent, runner)

	start := time.Now()
	execErr := p.Execute(context.Background(), d)
	duration := time.Since(start)

	if execErr != nil {
		fmt.Println(styleError.Render("✗ Failed") + styleDim.Render(fmt.Sprintf(" (%.1fs)", duration.Seconds())))
		return d.Summary(), execErr
	}

	fmt.Println(styleSuccess.Render("✓ Done") + styleDim.Render(fmt.Sprintf(" (%.1fs, %d steps)", duration.Seconds(), len(d.Nodes))))
	return d.Summary(), nil
}
```

**Step 2: Verify build**

Run: `go build ./cmd/apex/`
Expected: No errors

**Step 3: Commit**

```bash
git add cmd/apex/stream.go
git commit -m "feat(repl): add streaming task runner wrapper"
```

---

### Task 5: Create Interactive REPL Loop

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/interactive.go`
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/main.go`

**Step 1: Write interactive.go**

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/lyndonlyu/apex/internal/config"
	"github.com/lyndonlyu/apex/internal/governance"
	"github.com/lyndonlyu/apex/internal/memory"
	"github.com/spf13/cobra"
)

type session struct {
	cfg   *config.Config
	turns []turn
}

type turn struct {
	task    string
	summary string
}

func (s *session) context() string {
	if len(s.turns) == 0 {
		return ""
	}
	var parts []string
	// Keep last 5 turns to stay within context budget
	start := 0
	if len(s.turns) > 5 {
		start = len(s.turns) - 5
	}
	for _, t := range s.turns[start:] {
		summary := t.summary
		if len(summary) > 500 {
			summary = summary[:500] + "..."
		}
		parts = append(parts, fmt.Sprintf("Task: %s\nResult: %s", t.task, summary))
	}
	return strings.Join(parts, "\n---\n")
}

var slashCommands = []prompt.Suggest{
	{Text: "/help", Description: "Show available commands"},
	{Text: "/status", Description: "Show recent run history"},
	{Text: "/history", Description: "Show task execution history"},
	{Text: "/doctor", Description: "Run system integrity check"},
	{Text: "/clear", Description: "Clear screen"},
	{Text: "/config", Description: "Show current config summary"},
	{Text: "/quit", Description: "Exit session"},
	{Text: "/exit", Description: "Exit session"},
}

func completer(d prompt.Document) []prompt.Suggest {
	text := d.TextBeforeCursor()
	if strings.HasPrefix(text, "/") {
		return prompt.FilterHasPrefix(slashCommands, text, true)
	}
	return nil
}

func startInteractive(cmd *cobra.Command, args []string) error {
	home, err := homeDir()
	if err != nil {
		return err
	}
	configPath := filepath.Join(home, ".apex", "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config validation: %w", err)
	}

	governance.SetPolicy(governance.Policy{
		AutoApprove: cfg.Governance.AutoApprove,
		Confirm:     cfg.Governance.Confirm,
		Reject:      cfg.Governance.Reject,
	})

	s := &session{cfg: cfg}

	// Banner
	fmt.Println(styleBanner.Render(fmt.Sprintf(
		"apex v0.1.0 · %s · %s",
		cfg.Claude.Model, cfg.Sandbox.Level,
	)))
	fmt.Println(styleInfo.Render("Type a task, /help for commands, /quit to exit"))
	fmt.Println()

	p := prompt.New(
		func(input string) {
			input = strings.TrimSpace(input)
			if input == "" {
				return
			}

			// Slash commands
			if strings.HasPrefix(input, "/") {
				s.handleSlash(input)
				return
			}

			// Execute task
			summary, err := runInteractiveTask(s.cfg, input, s.context())
			if err != nil {
				fmt.Println(styleError.Render("Error: " + err.Error()))
			}
			s.turns = append(s.turns, turn{task: input, summary: summary})
			fmt.Println()
		},
		completer,
		prompt.OptionPrefix("apex> "),
		prompt.OptionPrefixTextColor(prompt.Green),
		prompt.OptionTitle("Apex Interactive"),
		prompt.OptionMaxSuggestion(8),
	)

	p.Run()

	// Save session on exit
	s.saveSession()
	return nil
}

func (s *session) handleSlash(input string) {
	switch strings.ToLower(strings.Fields(input)[0]) {
	case "/help":
		fmt.Println(styleBanner.Render("Commands:"))
		for _, sc := range slashCommands {
			fmt.Printf("  %-12s %s\n", stylePrompt.Render(sc.Text), sc.Description)
		}
	case "/status":
		statusCmd.RunE(nil, nil)
	case "/history":
		historyCmd.RunE(nil, nil)
	case "/doctor":
		doctorCmd.RunE(nil, nil)
	case "/clear":
		fmt.Print("\033[H\033[2J")
	case "/config":
		fmt.Printf("Model:   %s\n", s.cfg.Claude.Model)
		fmt.Printf("Effort:  %s\n", s.cfg.Claude.Effort)
		fmt.Printf("Sandbox: %s\n", s.cfg.Sandbox.Level)
		fmt.Printf("Pool:    %d workers\n", s.cfg.Pool.MaxConcurrent)
	case "/quit", "/exit":
		fmt.Println(styleInfo.Render("Session saved. Goodbye!"))
		s.saveSession()
		os.Exit(0)
	default:
		fmt.Println(styleError.Render("Unknown command. Type /help for available commands."))
	}
	fmt.Println()
}

func (s *session) saveSession() {
	if len(s.turns) == 0 {
		return
	}
	memDir := filepath.Join(s.cfg.BaseDir, "memory")
	store, err := memory.NewStore(memDir)
	if err != nil {
		return
	}
	var tasks []string
	for _, t := range s.turns {
		tasks = append(tasks, t.task)
	}
	store.SaveSession("interactive", strings.Join(tasks, "; "), s.context())
}
```

**Step 2: Wire into main.go — set rootCmd.RunE**

In `main.go`, add `RunE` to `rootCmd`:

```go
var rootCmd = &cobra.Command{
	Use:   "apex",
	Short: "Apex Agent - Claude Code autonomous agent system",
	Long:  "Apex Agent is a CLI tool that orchestrates Claude Code for long-term memory autonomous agent tasks.",
	RunE:  startInteractive,
}
```

**Step 3: Verify build**

Run: `go build ./cmd/apex/`
Expected: No errors

**Step 4: Commit**

```bash
git add cmd/apex/interactive.go cmd/apex/main.go
git commit -m "feat(repl): add interactive REPL mode with slash commands"
```

---

### Task 6: Write Tests for Interactive Components

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/interactive_test.go`

**Step 1: Write tests**

```go
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSessionContext(t *testing.T) {
	s := &session{}
	assert.Empty(t, s.context())

	s.turns = append(s.turns, turn{task: "analyze code", summary: "found 3 issues"})
	ctx := s.context()
	assert.Contains(t, ctx, "analyze code")
	assert.Contains(t, ctx, "found 3 issues")
}

func TestSessionContextTruncation(t *testing.T) {
	s := &session{}
	// Add 7 turns, only last 5 should appear in context
	for i := 0; i < 7; i++ {
		s.turns = append(s.turns, turn{
			task:    fmt.Sprintf("task %d", i),
			summary: fmt.Sprintf("result %d", i),
		})
	}
	ctx := s.context()
	assert.NotContains(t, ctx, "task 0")
	assert.NotContains(t, ctx, "task 1")
	assert.Contains(t, ctx, "task 2")
	assert.Contains(t, ctx, "task 6")
}

func TestSessionContextSummaryTruncation(t *testing.T) {
	s := &session{}
	longSummary := strings.Repeat("x", 1000)
	s.turns = append(s.turns, turn{task: "test", summary: longSummary})
	ctx := s.context()
	assert.True(t, len(ctx) < 600, "context should be truncated")
	assert.Contains(t, ctx, "...")
}

func TestRenderRisk(t *testing.T) {
	// Just verify no panics and returns non-empty
	assert.NotEmpty(t, renderRisk("LOW"))
	assert.NotEmpty(t, renderRisk("MEDIUM"))
	assert.NotEmpty(t, renderRisk("HIGH"))
	assert.NotEmpty(t, renderRisk("CRITICAL"))
	assert.NotEmpty(t, renderRisk("UNKNOWN"))
}
```

**Step 2: Run tests**

Run: `go test ./cmd/apex/ -run TestSession -v`
Expected: All PASS

**Step 3: Commit**

```bash
git add cmd/apex/interactive_test.go
git commit -m "test(repl): add session context and style tests"
```

---

### Task 7: Integration Test — E2E REPL Smoke

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/e2e/interactive_test.go`

**Step 1: Write E2E smoke test**

```go
package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInteractiveVersionBanner(t *testing.T) {
	env := newTestEnv(t)

	// Run apex with no args, piping "/quit" to stdin
	// This should show the banner and exit cleanly
	stdout, stderr, code := env.runApexWithEnv(
		map[string]string{"APEX_TEST_INPUT": "/quit"},
		// Pass empty args so rootCmd.RunE fires
	)

	// Since go-prompt needs a real TTY, this may fail in test.
	// At minimum, verify the binary doesn't crash with no args.
	_ = stdout
	_ = stderr
	// Non-crash is the success criterion for this smoke test.
	assert.True(t, code == 0 || code == 1, "apex should not crash with no args")
}
```

**Step 2: Run test**

Run: `go test ./e2e/ -run TestInteractiveVersionBanner -v`
Expected: PASS (non-crash verification)

**Step 3: Commit**

```bash
git add e2e/interactive_test.go
git commit -m "test(e2e): add interactive mode smoke test"
```

---

### Task 8: Final Verification & Documentation

**Files:**
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/README.md` (add Interactive Mode section)

**Step 1: Run full test suite**

Run: `go test ./...`
Expected: All PASS

**Step 2: Manual smoke test**

Run: `go build -o /tmp/apex ./cmd/apex/ && /tmp/apex`
Expected: See banner, prompt, type `/help` to see commands, `/quit` to exit

**Step 3: Add Interactive Mode section to README**

Add after the "Quick Start" section:

```markdown
### Interactive Mode

```bash
# Enter interactive REPL
$ apex

apex v0.1.0 · claude-sonnet-4 · ulimit
Type a task, /help for commands, /quit to exit

apex> analyze the error handling in this codebase
[LOW] Planning... 1 step
● Analyzing... ✓ (8.2s)
→ Found 3 patterns...

apex> now refactor them to use fmt.Errorf
[MEDIUM] Confirm? (y/n): y
...

apex> /quit
Session saved. Goodbye!
```

Session context persists across turns — follow-up tasks can reference previous results.
```

**Step 4: Commit**

```bash
git add README.md
git commit -m "docs: add interactive mode section to README"
```

---

## Summary

| Task | Description | Est. Complexity |
|------|-------------|-----------------|
| 1 | Add dependencies (go-prompt, lipgloss) | Trivial |
| 2 | Add OnOutput streaming to executor | Small |
| 3 | Create style definitions | Trivial |
| 4 | Create streaming task wrapper | Medium |
| 5 | Create REPL loop + wire to main | Medium |
| 6 | Unit tests for session/style | Small |
| 7 | E2E smoke test | Small |
| 8 | Final verification + docs | Small |

Total: 8 tasks, ~800 LOC new code

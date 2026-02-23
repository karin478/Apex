# Slash Commands Expansion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Expand Apex REPL from 7 to 30 slash commands using a registry-based architecture, bridging existing cobra handlers and adding new session-level features.

**Architecture:** Replace monolithic switch in `handleSlash` with a command registry (`[]slashCmd`). Each command is a struct with name, aliases, description, and handler. `/help` and autocomplete are auto-generated from the registry. New handlers live in `cmd/apex/slash.go`; REPL loop stays in `interactive.go`.

**Tech Stack:** Go, chzyer/readline (autocomplete), lipgloss (styling), os/exec (shell escape)

---

### Task 1: Command Registry Infrastructure

**Files:**
- Create: `cmd/apex/slash.go`
- Modify: `cmd/apex/interactive.go`

**Step 1: Create `slash.go` with the registry type and all command registrations**

```go
// cmd/apex/slash.go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/chzyer/readline"
)

// slashCmd defines a single slash command in the REPL.
type slashCmd struct {
	name    string
	aliases []string
	desc    string
	// handler returns true if the session should exit.
	handler func(s *session, args string, rl *readline.Instance) bool
}

// slashCommands is the global command registry. Order determines /help display order.
var slashCommands = []slashCmd{
	// --- Session Control ---
	{name: "help", desc: "Show available commands", handler: cmdHelp},
	{name: "new", desc: "Start fresh session without exiting", handler: cmdNew},
	{name: "compact", desc: "Compress context to free token budget", handler: cmdCompact},
	{name: "copy", desc: "Copy last response to clipboard", handler: cmdCopy},
	{name: "export", desc: "Export session transcript to file", handler: cmdExport},
	{name: "clear", desc: "Clear screen", handler: cmdClear},

	// --- Runtime Config ---
	{name: "model", desc: "View or switch model (e.g. /model claude-sonnet-4-6)", handler: cmdModel},
	{name: "permissions", desc: "View or switch permission mode", handler: cmdPermissions},
	{name: "mode", desc: "View or switch execution mode", handler: cmdMode},
	{name: "config", desc: "Show current config summary", handler: cmdConfig},

	// --- Context & Files ---
	{name: "mention", desc: "Attach file to next task context", handler: cmdMention},
	{name: "context", desc: "Show context size and turn count", handler: cmdContext},
	{name: "diff", desc: "Show git diff of working directory", handler: cmdDiff},

	// --- Memory & Knowledge ---
	{name: "memory", desc: "Search memory (/memory <query>) or clear (/memory clear)", handler: cmdMemory},
	{name: "kg", desc: "Query knowledge graph (/kg <query>)", handler: cmdKG},

	// --- Execution Tools ---
	{name: "plan", desc: "Plan task without executing (/plan <task>)", handler: cmdPlan},
	{name: "review", desc: "Review current git changes", handler: cmdReview},
	{name: "trace", desc: "Show last execution trace", handler: cmdTrace},
	{name: "metrics", desc: "Show execution metrics", handler: cmdMetrics},

	// --- System ---
	{name: "status", desc: "Show recent run history", handler: cmdStatus},
	{name: "history", desc: "Show task execution history", handler: cmdHistory},
	{name: "doctor", desc: "Run system integrity check", handler: cmdDoctor},
	{name: "snapshot", desc: "List execution snapshots", handler: cmdSnapshot},
	{name: "plugin", desc: "List loaded plugins", handler: cmdPlugin},
	{name: "gc", desc: "Clean up old runs/audit/snapshots", handler: cmdGC},

	// --- Utility ---
	{name: "version", desc: "Show version info", handler: cmdVersion},
	{name: "quit", aliases: []string{"exit"}, desc: "Exit session", handler: cmdQuit},
}

// findCommand looks up a slash command by name or alias.
func findCommand(name string) *slashCmd {
	name = strings.ToLower(name)
	for i := range slashCommands {
		if slashCommands[i].name == name {
			return &slashCommands[i]
		}
		for _, a := range slashCommands[i].aliases {
			if a == name {
				return &slashCommands[i]
			}
		}
	}
	return nil
}

// buildCompleter generates a readline PrefixCompleter from the registry.
func buildCompleter() *readline.PrefixCompleter {
	items := make([]readline.PrefixCompleterInterface, 0, len(slashCommands))
	for _, cmd := range slashCommands {
		items = append(items, readline.PcItem("/"+cmd.name))
		for _, a := range cmd.aliases {
			items = append(items, readline.PcItem("/"+a))
		}
	}
	return readline.NewPrefixCompleter(items...)
}
```

**Step 2: Add command handler stubs (all 28 handlers)**

Append to `slash.go`:

```go
// ── Session Control ──

func cmdHelp(s *session, args string, rl *readline.Instance) bool {
	fmt.Println()
	for _, cmd := range slashCommands {
		fmt.Printf("  %-16s %s\n", stylePrompt.Render("/"+cmd.name), cmd.desc)
	}
	fmt.Println()
	fmt.Println(styleDim.Render("  Tip: !<cmd> executes a shell command (e.g. !ls)"))
	fmt.Println()
	return false
}

func cmdNew(s *session, args string, rl *readline.Instance) bool {
	s.turns = nil
	s.lastOutput = ""
	s.attachments = nil
	fmt.Print("\033[H\033[2J")
	printBanner(s.cfg)
	fmt.Println(styleSuccess.Render("  New session started."))
	fmt.Println()
	return false
}

func cmdCompact(s *session, args string, rl *readline.Instance) bool {
	if len(s.turns) <= 2 {
		fmt.Println(styleInfo.Render("  Context already minimal."))
		fmt.Println()
		return false
	}
	// Keep last 2 turns full, compress older ones to task-only
	for i := 0; i < len(s.turns)-2; i++ {
		s.turns[i].summary = truncate(s.turns[i].summary, 80)
	}
	fmt.Println(styleSuccess.Render(fmt.Sprintf("  Compacted %d turns. Last 2 kept in full.", len(s.turns)-2)))
	fmt.Println()
	return false
}

func cmdCopy(s *session, args string, rl *readline.Instance) bool {
	if s.lastOutput == "" {
		fmt.Println(styleError.Render("  No response to copy."))
		fmt.Println()
		return false
	}
	var clipCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		clipCmd = exec.Command("pbcopy")
	case "linux":
		clipCmd = exec.Command("xclip", "-selection", "clipboard")
	default:
		fmt.Println(styleError.Render("  Clipboard not supported on " + runtime.GOOS))
		fmt.Println()
		return false
	}
	clipCmd.Stdin = strings.NewReader(s.lastOutput)
	if err := clipCmd.Run(); err != nil {
		fmt.Println(styleError.Render("  Copy failed: " + err.Error()))
		fmt.Println()
		return false
	}
	fmt.Println(styleSuccess.Render("  Copied to clipboard."))
	fmt.Println()
	return false
}

func cmdExport(s *session, args string, rl *readline.Instance) bool {
	if len(s.turns) == 0 {
		fmt.Println(styleInfo.Render("  Nothing to export."))
		fmt.Println()
		return false
	}
	dir := filepath.Join(s.home, ".apex", "exports")
	os.MkdirAll(dir, 0o755)
	name := fmt.Sprintf("session-%s.md", time.Now().Format("20060102-150405"))
	path := filepath.Join(dir, name)
	var sb strings.Builder
	sb.WriteString("# Apex Session Export\n\n")
	for i, t := range s.turns {
		sb.WriteString(fmt.Sprintf("## Turn %d\n\n**Task:** %s\n\n**Response:**\n\n%s\n\n---\n\n", i+1, t.task, t.summary))
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		fmt.Println(styleError.Render("  Export failed: " + err.Error()))
		fmt.Println()
		return false
	}
	fmt.Println(styleSuccess.Render("  Exported to " + path))
	fmt.Println()
	return false
}

func cmdClear(s *session, args string, rl *readline.Instance) bool {
	fmt.Print("\033[H\033[2J")
	printBanner(s.cfg)
	return false
}

// ── Runtime Config ──

func cmdModel(s *session, args string, rl *readline.Instance) bool {
	if args == "" {
		fmt.Printf("  Model:  %s\n", s.cfg.Claude.Model)
		fmt.Printf("  Effort: %s\n", s.cfg.Claude.Effort)
		fmt.Println()
		return false
	}
	parts := strings.Fields(args)
	s.cfg.Claude.Model = parts[0]
	if len(parts) > 1 {
		s.cfg.Claude.Effort = parts[1]
	}
	fmt.Println(styleSuccess.Render(fmt.Sprintf("  Switched to %s (effort: %s)", s.cfg.Claude.Model, s.cfg.Claude.Effort)))
	fmt.Println()
	return false
}

func cmdPermissions(s *session, args string, rl *readline.Instance) bool {
	if args == "" {
		fmt.Printf("  Permission mode: %s\n", s.cfg.Claude.PermissionMode)
		fmt.Println()
		return false
	}
	s.cfg.Claude.PermissionMode = strings.TrimSpace(args)
	fmt.Println(styleSuccess.Render("  Permission mode set to " + s.cfg.Claude.PermissionMode))
	fmt.Println()
	return false
}

func cmdMode(s *session, args string, rl *readline.Instance) bool {
	if args == "" {
		return bridgeCobra(runModeList, nil)
	}
	return bridgeCobra(runModeSelect, []string{strings.TrimSpace(args)})
}

func cmdConfig(s *session, args string, rl *readline.Instance) bool {
	fmt.Printf("  Model:       %s\n", s.cfg.Claude.Model)
	fmt.Printf("  Effort:      %s\n", s.cfg.Claude.Effort)
	fmt.Printf("  Sandbox:     %s\n", s.cfg.Sandbox.Level)
	fmt.Printf("  Pool:        %d workers\n", s.cfg.Pool.MaxConcurrent)
	fmt.Printf("  Permissions: %s\n", s.cfg.Claude.PermissionMode)
	fmt.Println()
	return false
}

// ── Context & Files ──

func cmdMention(s *session, args string, rl *readline.Instance) bool {
	path := strings.TrimSpace(args)
	if path == "" {
		if len(s.attachments) == 0 {
			fmt.Println(styleInfo.Render("  No files attached. Usage: /mention <path>"))
		} else {
			fmt.Println(styleInfo.Render(fmt.Sprintf("  %d file(s) attached:", len(s.attachments))))
			for _, a := range s.attachments {
				fmt.Println("    " + a)
			}
		}
		fmt.Println()
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println(styleError.Render("  Cannot read file: " + err.Error()))
		fmt.Println()
		return false
	}
	s.attachments = append(s.attachments, path)
	fmt.Println(styleSuccess.Render(fmt.Sprintf("  Attached %s (%d bytes). Will be included in next task.", path, len(data))))
	fmt.Println()
	return false
}

func cmdContext(s *session, args string, rl *readline.Instance) bool {
	ctx := s.context()
	fmt.Printf("  Turns:       %d\n", len(s.turns))
	fmt.Printf("  Context:     %d chars\n", len(ctx))
	fmt.Printf("  Attachments: %d\n", len(s.attachments))
	fmt.Println()
	return false
}

func cmdDiff(s *session, args string, rl *readline.Instance) bool {
	cmd := exec.Command("git", "diff", "--stat")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	fmt.Println()
	return false
}

// ── Memory & Knowledge ──

func cmdMemory(s *session, args string, rl *readline.Instance) bool {
	args = strings.TrimSpace(args)
	if args == "clear" {
		s.turns = nil
		s.lastOutput = ""
		fmt.Println(styleSuccess.Render("  Session memory cleared."))
		fmt.Println()
		return false
	}
	if args == "" {
		return bridgeCobra(searchMemory, []string{"*"})
	}
	return bridgeCobra(searchMemory, []string{args})
}

func cmdKG(s *session, args string, rl *readline.Instance) bool {
	args = strings.TrimSpace(args)
	if args == "" {
		return bridgeCobra(runKGList, nil)
	}
	return bridgeCobra(runKGQuery, []string{args})
}

// ── Execution Tools ──

func cmdPlan(s *session, args string, rl *readline.Instance) bool {
	task := strings.TrimSpace(args)
	if task == "" {
		fmt.Println(styleError.Render("  Usage: /plan <task description>"))
		fmt.Println()
		return false
	}
	return bridgeCobra(planTask, []string{task})
}

func cmdReview(s *session, args string, rl *readline.Instance) bool {
	arg := strings.TrimSpace(args)
	if arg == "" {
		arg = "Review the current git working tree changes"
	}
	return bridgeCobra(runReview, []string{arg})
}

func cmdTrace(s *session, args string, rl *readline.Instance) bool {
	if args := strings.TrimSpace(args); args != "" {
		return bridgeCobra(showTrace, []string{args})
	}
	return bridgeCobra(showTrace, nil)
}

func cmdMetrics(s *session, args string, rl *readline.Instance) bool {
	return bridgeCobra(showMetrics, nil)
}

// ── System ──

func cmdStatus(s *session, args string, rl *readline.Instance) bool {
	return bridgeCobra(showStatus, nil)
}

func cmdHistory(s *session, args string, rl *readline.Instance) bool {
	return bridgeCobra(showHistory, nil)
}

func cmdDoctor(s *session, args string, rl *readline.Instance) bool {
	return bridgeCobra(runDoctor, nil)
}

func cmdSnapshot(s *session, args string, rl *readline.Instance) bool {
	return bridgeCobra(listSnapshots, nil)
}

func cmdPlugin(s *session, args string, rl *readline.Instance) bool {
	return bridgeCobra(pluginList, nil)
}

func cmdGC(s *session, args string, rl *readline.Instance) bool {
	return bridgeCobra(runGC, nil)
}

// ── Utility ──

func cmdVersion(s *session, args string, rl *readline.Instance) bool {
	fmt.Println("  apex v0.1.0")
	fmt.Println()
	return false
}

func cmdQuit(s *session, args string, rl *readline.Instance) bool {
	s.saveSession()
	return true
}

// ── Helpers ──

// bridgeCobra calls an existing cobra RunE handler with synthetic args.
// Returns false (never exits session).
func bridgeCobra(fn func(*cobra.Command, []string) error, args []string) bool {
	if args == nil {
		args = []string{}
	}
	if err := fn(nil, args); err != nil {
		fmt.Println(styleError.Render("  Error: " + err.Error()))
	}
	fmt.Println()
	return false
}

// runShellCommand handles the !<cmd> escape syntax.
func runShellCommand(cmdStr string) {
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Println(styleError.Render("  " + err.Error()))
	}
	fmt.Println()
}

// truncate shortens a string to max n characters.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
```

Note: `bridgeCobra` passes `nil` for `cmd` — all the existing handlers that we checked accept nil cmd safely (they only use `args`). For handlers that reference `cmd.Flags()`, we would need a synthetic command, but none of our bridged handlers do.

**Step 3: Rewrite `interactive.go` to use registry**

Replace the `handleSlash` method and REPL loop to dispatch via the registry instead of a switch:

```go
// In interactive.go, the session struct becomes:
type session struct {
	cfg         *config.Config
	turns       []turn
	lastOutput  string
	attachments []string
	home        string
}

// The REPL loop changes from:
//   s.handleSlash(input, rl)
// To:
//   dispatchSlash(s, input, rl)

// The entire handleSlash method is deleted.
```

The REPL loop in `startInteractive` changes to:
- Build completer from `buildCompleter()` instead of manual `PcItem` list
- Store `home` in session struct
- On `!` prefix, call `runShellCommand(input[1:])`
- On `/` prefix, parse command name + args, call `findCommand(name).handler(...)`
- After task execution, store result in `s.lastOutput`
- Before task execution, prepend `s.attachments` content to task, then clear attachments

**Step 4: Build and verify compilation**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go build ./cmd/apex/`
Expected: Build succeeds with no errors (only sqlite cgo warnings)

**Step 5: Run existing tests**

Run: `go test ./cmd/apex/ -v -run "TestSession|TestRender"`
Expected: All 4 existing tests pass

**Step 6: Commit**

```bash
git add cmd/apex/slash.go cmd/apex/interactive.go
git commit -m "feat(repl): add command registry with 30 slash commands

Replace monolithic handleSlash switch with registry-based dispatch.
Add 22 new commands: /new, /compact, /copy, /export, /model,
/permissions, /mode, /mention, /context, /diff, /memory, /kg,
/plan, /review, /trace, /metrics, /snapshot, /plugin, /gc,
/version, and ! shell escape.

Auto-generate /help and autocomplete from registry."
```

---

### Task 2: Unit Tests for New Commands

**Files:**
- Modify: `cmd/apex/interactive_test.go`

**Step 1: Write tests for the pure (non-cobra-bridge) commands**

```go
func TestCmdCompact(t *testing.T) {
	s := &session{
		cfg:   &config.Config{},
		turns: make([]turn, 5),
	}
	for i := range s.turns {
		s.turns[i] = turn{task: fmt.Sprintf("task %d", i), summary: strings.Repeat("x", 200)}
	}
	cmdCompact(s, "", nil)
	// First 3 turns should be truncated to 80+3 chars
	for i := 0; i < 3; i++ {
		if len(s.turns[i].summary) > 84 {
			t.Errorf("turn %d not compacted: len=%d", i, len(s.turns[i].summary))
		}
	}
	// Last 2 should be untouched
	if len(s.turns[3].summary) != 200 {
		t.Error("turn 3 should be full")
	}
}

func TestCmdNew(t *testing.T) {
	cfg := &config.Config{}
	cfg.Claude.Model = "test"
	cfg.Sandbox.Level = "none"
	s := &session{
		cfg:         cfg,
		turns:       []turn{{task: "x", summary: "y"}},
		lastOutput:  "hello",
		attachments: []string{"file.txt"},
	}
	cmdNew(s, "", nil)
	if len(s.turns) != 0 || s.lastOutput != "" || len(s.attachments) != 0 {
		t.Error("session not reset")
	}
}

func TestCmdContext(t *testing.T) {
	s := &session{
		cfg:         &config.Config{},
		turns:       []turn{{task: "a", summary: "b"}, {task: "c", summary: "d"}},
		attachments: []string{"f1.go", "f2.go"},
	}
	// Just ensure it doesn't panic
	cmdContext(s, "", nil)
}

func TestCmdMention(t *testing.T) {
	s := &session{cfg: &config.Config{}}
	// Non-existent file
	cmdMention(s, "/tmp/nonexistent_file_12345.txt", nil)
	if len(s.attachments) != 0 {
		t.Error("should not attach nonexistent file")
	}
}

func TestFindCommand(t *testing.T) {
	cmd := findCommand("help")
	if cmd == nil || cmd.name != "help" {
		t.Error("should find /help")
	}
	cmd = findCommand("exit")
	if cmd == nil || cmd.name != "quit" {
		t.Error("should find /exit as alias of /quit")
	}
	cmd = findCommand("nonexistent")
	if cmd != nil {
		t.Error("should return nil for unknown command")
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello", 3) != "hel..." {
		t.Error("should truncate")
	}
	if truncate("hi", 10) != "hi" {
		t.Error("should not truncate short strings")
	}
}

func TestBuildCompleter(t *testing.T) {
	c := buildCompleter()
	if c == nil {
		t.Error("should return non-nil completer")
	}
}
```

**Step 2: Run tests**

Run: `go test ./cmd/apex/ -v -run "TestCmd|TestFind|TestTruncate|TestBuild"`
Expected: All new tests PASS

**Step 3: Commit**

```bash
git add cmd/apex/interactive_test.go
git commit -m "test(repl): add unit tests for slash command registry"
```

---

### Task 3: Integration Test for Shell Escape

**Files:**
- Modify: `cmd/apex/interactive_test.go`

**Step 1: Write test for runShellCommand**

```go
func TestRunShellCommand(t *testing.T) {
	// Capture that it doesn't panic on a simple command
	runShellCommand("echo hello")
}
```

**Step 2: Run test**

Run: `go test ./cmd/apex/ -v -run "TestRunShell"`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/apex/interactive_test.go
git commit -m "test(repl): add shell escape integration test"
```

---

### Task 4: Full Test Suite Verification

**Step 1: Run all tests**

Run: `cd /Users/lyndonlyu/Downloads/ai_agent_cli_project && go test ./... 2>&1 | tail -50`
Expected: All packages PASS

**Step 2: Build and manual smoke test**

Run: `go build -o /tmp/apex_dev ./cmd/apex/ && echo "Build OK"`
Expected: Build OK

**Step 3: Final commit if any fixups needed**

Only if tests revealed issues that need fixing.

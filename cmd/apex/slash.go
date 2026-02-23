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
	"github.com/spf13/cobra"
)

// slashCmd represents a single slash command in the interactive REPL.
type slashCmd struct {
	name    string
	aliases []string
	desc    string
	handler func(s *session, args string, rl *readline.Instance) bool
}

// slashCommands is the global registry of all interactive slash commands.
// Populated in init() to avoid initialization cycles (cmdHelp references slashCommands).
var slashCommands []slashCmd

func init() {
	slashCommands = []slashCmd{
		// Session
		{name: "help", desc: "Show available commands", handler: cmdHelp},
		{name: "new", desc: "Start a new session (clear context)", handler: cmdNew},
		{name: "compact", desc: "Compact session context", handler: cmdCompact},
		{name: "copy", desc: "Copy last output to clipboard", handler: cmdCopy},
		{name: "export", desc: "Export session to markdown", handler: cmdExport},
		{name: "clear", desc: "Clear screen", handler: cmdClear},

		// Config
		{name: "model", desc: "Show or set model [model] [effort]", handler: cmdModel},
		{name: "permissions", desc: "Show or set permission mode", handler: cmdPermissions},
		{name: "mode", desc: "List or select execution mode", handler: cmdMode},
		{name: "config", desc: "Show current configuration", handler: cmdConfig},

		// Context
		{name: "mention", desc: "Attach a file to next task", handler: cmdMention},
		{name: "context", desc: "Show session context stats", handler: cmdContext},
		{name: "diff", desc: "Show git diff summary", handler: cmdDiff},

		// Memory
		{name: "memory", desc: "Search or clear session memory", handler: cmdMemory},
		{name: "kg", desc: "Knowledge graph operations", handler: cmdKG},

		// Execution
		{name: "plan", desc: "Plan a task without executing", handler: cmdPlan},
		{name: "review", desc: "Review current changes", handler: cmdReview},
		{name: "trace", desc: "Show execution trace", handler: cmdTrace},
		{name: "metrics", desc: "Show execution metrics", handler: cmdMetrics},

		// System
		{name: "status", desc: "Show system status", handler: cmdStatus},
		{name: "history", desc: "Show task execution history", handler: cmdHistory},
		{name: "doctor", desc: "Run system integrity check", handler: cmdDoctor},
		{name: "snapshot", desc: "List snapshots", handler: cmdSnapshot},
		{name: "plugin", desc: "List plugins", handler: cmdPlugin},
		{name: "gc", desc: "Run garbage collection", handler: cmdGC},

		// Utility
		{name: "version", desc: "Show version", handler: cmdVersion},
		{name: "quit", aliases: []string{"exit"}, desc: "Exit session", handler: cmdQuit},
	}
}

// findCommand looks up a slash command by name or alias (case-insensitive).
func findCommand(name string) *slashCmd {
	lower := strings.ToLower(name)
	for i := range slashCommands {
		if slashCommands[i].name == lower {
			return &slashCommands[i]
		}
		for _, alias := range slashCommands[i].aliases {
			if alias == lower {
				return &slashCommands[i]
			}
		}
	}
	return nil
}

// buildCompleter auto-generates a readline completer from the command registry.
func buildCompleter() *readline.PrefixCompleter {
	var items []readline.PrefixCompleterInterface
	for _, c := range slashCommands {
		items = append(items, readline.PcItem("/"+c.name))
		for _, alias := range c.aliases {
			items = append(items, readline.PcItem("/"+alias))
		}
	}
	return readline.NewPrefixCompleter(items...)
}

// bridgeCobra calls an existing cobra RunE handler with synthetic args.
// It prints any error and always returns false (never exits session).
func bridgeCobra(fn func(*cobra.Command, []string) error, args []string) bool {
	if err := fn(nil, args); err != nil {
		fmt.Println(styleError.Render("Error: " + err.Error()))
	}
	return false
}

// runShellCommand runs an arbitrary shell command with stdio connected.
func runShellCommand(cmdStr string) {
	c := exec.Command("sh", "-c", cmdStr)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	if err := c.Run(); err != nil {
		fmt.Println(styleError.Render("Shell error: " + err.Error()))
	}
}

// truncate truncates a string to n runes and appends "..." if needed.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

// --- Handler implementations ---

func cmdHelp(s *session, args string, rl *readline.Instance) bool {
	fmt.Println()
	for _, c := range slashCommands {
		fmt.Printf("  %-16s %s\n", stylePrompt.Render("/"+c.name), c.desc)
	}
	fmt.Println()
	fmt.Println(styleDim.Render("  Tip: Use !<cmd> to run a shell command"))
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
	// Truncate all turns except the last 2
	for i := 0; i < len(s.turns)-2; i++ {
		s.turns[i].summary = truncate(s.turns[i].summary, 80)
		s.turns[i].task = truncate(s.turns[i].task, 80)
	}
	fmt.Println(styleSuccess.Render(fmt.Sprintf("  Compacted %d turns (kept last 2 full).", len(s.turns)-2)))
	fmt.Println()
	return false
}

func cmdCopy(s *session, args string, rl *readline.Instance) bool {
	if s.lastOutput == "" {
		fmt.Println(styleError.Render("  No output to copy."))
		fmt.Println()
		return false
	}
	var clipCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		clipCmd = exec.Command("pbcopy")
	default:
		clipCmd = exec.Command("xclip", "-selection", "clipboard")
	}
	clipCmd.Stdin = strings.NewReader(s.lastOutput)
	if err := clipCmd.Run(); err != nil {
		fmt.Println(styleError.Render("  Copy failed: " + err.Error()))
	} else {
		fmt.Println(styleSuccess.Render("  Copied to clipboard."))
	}
	fmt.Println()
	return false
}

func cmdExport(s *session, args string, rl *readline.Instance) bool {
	if len(s.turns) == 0 {
		fmt.Println(styleInfo.Render("  Nothing to export."))
		fmt.Println()
		return false
	}
	exportDir := filepath.Join(s.home, ".apex", "exports")
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		fmt.Println(styleError.Render("  Export dir error: " + err.Error()))
		fmt.Println()
		return false
	}
	filename := fmt.Sprintf("session-%s.md", time.Now().Format("20060102-150405"))
	path := filepath.Join(exportDir, filename)

	var sb strings.Builder
	sb.WriteString("# Apex Session Export\n\n")
	for i, t := range s.turns {
		sb.WriteString(fmt.Sprintf("## Turn %d\n\n", i+1))
		sb.WriteString(fmt.Sprintf("**Task:** %s\n\n", t.task))
		sb.WriteString(fmt.Sprintf("**Response:**\n\n%s\n\n", t.summary))
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		fmt.Println(styleError.Render("  Export write error: " + err.Error()))
	} else {
		fmt.Println(styleSuccess.Render("  Exported to " + path))
	}
	fmt.Println()
	return false
}

func cmdClear(s *session, args string, rl *readline.Instance) bool {
	fmt.Print("\033[H\033[2J")
	printBanner(s.cfg)
	return false
}

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
	fmt.Println(styleSuccess.Render(fmt.Sprintf("  Model set to %s (effort: %s)", s.cfg.Claude.Model, s.cfg.Claude.Effort)))
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
		bridgeCobra(runModeList, nil)
	} else {
		bridgeCobra(runModeSelect, []string{args})
	}
	fmt.Println()
	return false
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

func cmdMention(s *session, args string, rl *readline.Instance) bool {
	if args == "" {
		if len(s.attachments) == 0 {
			fmt.Println(styleInfo.Render("  No files attached."))
		} else {
			fmt.Printf("  %d file(s) attached:\n", len(s.attachments))
			for _, p := range s.attachments {
				fmt.Printf("    %s\n", p)
			}
		}
		fmt.Println()
		return false
	}
	path := strings.TrimSpace(args)
	info, err := os.Stat(path)
	if err != nil {
		fmt.Println(styleError.Render("  " + err.Error()))
		fmt.Println()
		return false
	}
	s.attachments = append(s.attachments, path)
	fmt.Println(styleSuccess.Render(fmt.Sprintf("  Attached %s (%d bytes)", path, info.Size())))
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
	c := exec.Command("git", "diff", "--stat")
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		fmt.Println(styleError.Render("  git diff error: " + err.Error()))
	}
	fmt.Println()
	return false
}

func cmdMemory(s *session, args string, rl *readline.Instance) bool {
	if args == "clear" {
		s.turns = nil
		s.lastOutput = ""
		fmt.Println(styleSuccess.Render("  Session memory cleared."))
		fmt.Println()
		return false
	}
	if args == "" {
		bridgeCobra(searchMemory, []string{"*"})
	} else {
		bridgeCobra(searchMemory, []string{args})
	}
	fmt.Println()
	return false
}

func cmdKG(s *session, args string, rl *readline.Instance) bool {
	if args == "" {
		bridgeCobra(runKGList, nil)
	} else {
		bridgeCobra(runKGQuery, []string{args})
	}
	fmt.Println()
	return false
}

func cmdPlan(s *session, args string, rl *readline.Instance) bool {
	if args == "" {
		fmt.Println(styleInfo.Render("  Usage: /plan <task description>"))
		fmt.Println()
		return false
	}
	bridgeCobra(planTask, []string{args})
	fmt.Println()
	return false
}

func cmdReview(s *session, args string, rl *readline.Instance) bool {
	arg := args
	if arg == "" {
		arg = "Review the current git working tree changes"
	}
	bridgeCobra(runReview, []string{arg})
	fmt.Println()
	return false
}

func cmdTrace(s *session, args string, rl *readline.Instance) bool {
	if args != "" {
		bridgeCobra(showTrace, []string{args})
	} else {
		bridgeCobra(showTrace, nil)
	}
	fmt.Println()
	return false
}

func cmdMetrics(s *session, args string, rl *readline.Instance) bool {
	bridgeCobra(showMetrics, nil)
	fmt.Println()
	return false
}

func cmdStatus(s *session, args string, rl *readline.Instance) bool {
	bridgeCobra(showStatus, nil)
	fmt.Println()
	return false
}

func cmdHistory(s *session, args string, rl *readline.Instance) bool {
	bridgeCobra(showHistory, nil)
	fmt.Println()
	return false
}

func cmdDoctor(s *session, args string, rl *readline.Instance) bool {
	bridgeCobra(runDoctor, nil)
	fmt.Println()
	return false
}

func cmdSnapshot(s *session, args string, rl *readline.Instance) bool {
	bridgeCobra(listSnapshots, nil)
	fmt.Println()
	return false
}

func cmdPlugin(s *session, args string, rl *readline.Instance) bool {
	bridgeCobra(pluginList, nil)
	fmt.Println()
	return false
}

func cmdGC(s *session, args string, rl *readline.Instance) bool {
	bridgeCobra(runGC, nil)
	fmt.Println()
	return false
}

func cmdVersion(s *session, args string, rl *readline.Instance) bool {
	fmt.Println("  apex v0.1.0")
	fmt.Println()
	return false
}

func cmdQuit(s *session, args string, rl *readline.Instance) bool {
	return true
}

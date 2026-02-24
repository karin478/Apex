package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/chzyer/readline"
	"github.com/lyndonlyu/apex/internal/health"
	"github.com/spf13/cobra"
)

// slashCmd represents a single slash command in the interactive REPL.
type slashCmd struct {
	name    string
	aliases []string
	group   string
	desc    string
	handler func(s *session, args string, rl *readline.Instance) bool
}

// slashCommands is the global registry of all interactive slash commands.
// Populated in init() to avoid initialization cycles (cmdHelp references slashCommands).
var slashCommands []slashCmd

func init() {
	slashCommands = []slashCmd{
		// Session
		{name: "help", group: "Session", desc: "Show available commands", handler: cmdHelp},
		{name: "new", group: "Session", desc: "Start a new session (clear context)", handler: cmdNew},
		{name: "compact", group: "Session", desc: "Compact session context", handler: cmdCompact},
		{name: "copy", group: "Session", desc: "Copy last output to clipboard", handler: cmdCopy},
		{name: "export", group: "Session", desc: "Export session to markdown", handler: cmdExport},
		{name: "clear", group: "Session", desc: "Clear screen", handler: cmdClear},

		// Config
		{name: "model", group: "Config", desc: "Show or set model [model] [effort]", handler: cmdModel},
		{name: "permissions", group: "Config", desc: "Show or set permission mode", handler: cmdPermissions},
		{name: "mode", group: "Config", desc: "List or select execution mode", handler: cmdMode},
		{name: "config", group: "Config", desc: "Show current configuration", handler: cmdConfig},
		{name: "theme", group: "Config", desc: "Switch theme (dark/light)", handler: cmdTheme},

		// Context
		{name: "mention", group: "Context", desc: "Attach a file to next task", handler: cmdMention},
		{name: "context", group: "Context", desc: "Show session context stats", handler: cmdContext},
		{name: "diff", group: "Context", desc: "Show git diff summary", handler: cmdDiff},

		// Memory
		{name: "memory", group: "Memory", desc: "Search or clear session memory", handler: cmdMemory},
		{name: "kg", group: "Memory", desc: "Knowledge graph operations", handler: cmdKG},

		// Execution
		{name: "plan", group: "Execution", desc: "Plan a task without executing", handler: cmdPlan},
		{name: "review", group: "Execution", desc: "Review current changes", handler: cmdReview},
		{name: "trace", group: "Execution", desc: "Show execution trace", handler: cmdTrace},
		{name: "metrics", group: "Execution", desc: "Show execution metrics", handler: cmdMetrics},

		// System
		{name: "status", group: "System", desc: "Show system status", handler: cmdStatus},
		{name: "history", group: "System", desc: "Show task execution history", handler: cmdHistory},
		{name: "doctor", group: "System", desc: "Run system integrity check", handler: cmdDoctor},
		{name: "snapshot", group: "System", desc: "List snapshots", handler: cmdSnapshot},
		{name: "plugin", group: "System", desc: "List plugins", handler: cmdPlugin},
		{name: "gc", group: "System", desc: "Run garbage collection", handler: cmdGC},

		// Utility
		{name: "version", group: "Utility", desc: "Show version", handler: cmdVersion},
		{name: "quit", aliases: []string{"exit"}, group: "Utility", desc: "Exit session", handler: cmdQuit},
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
	lastGroup := ""
	for _, c := range slashCommands {
		if c.group != lastGroup {
			if lastGroup != "" {
				fmt.Println()
			}
			fmt.Println(styleDim.Render("  " + c.group))
			lastGroup = c.group
		}
		fmt.Printf("    %-16s %s\n", stylePrompt.Render("/"+c.name), c.desc)
	}
	fmt.Println()
	fmt.Println(styleDim.Render("  Tip: !<cmd> runs a shell command · Tab for autocomplete"))
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
		fmt.Fprintf(&sb, "## Turn %d\n\n", i+1)
		fmt.Fprintf(&sb, "**Task:** %s\n\n", t.task)
		fmt.Fprintf(&sb, "**Response:**\n\n%s\n\n", t.summary)
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

// claudeModel defines an available Claude model with metadata.
type claudeModel struct {
	id    string // full model ID, e.g. "claude-opus-4-6"
	alias string // short alias, e.g. "opus"
	desc  string // one-line description
}

// availableModels is the built-in list of Claude models supported by the CLI.
var availableModels = []claudeModel{
	{id: "claude-opus-4-6", alias: "opus", desc: "Most capable, best for complex tasks"},
	{id: "claude-sonnet-4-5", alias: "sonnet", desc: "Balanced speed and capability"},
	{id: "claude-haiku-4-5", alias: "haiku", desc: "Fastest, best for simple tasks"},
}

// validEfforts lists the valid effort levels.
var validEfforts = []string{"low", "medium", "high"}

// findModelByQuery matches a model by alias, id, or number (1-based).
func findModelByQuery(query string) *claudeModel {
	q := strings.ToLower(strings.TrimSpace(query))
	// Try number (1-based)
	if n, err := fmt.Sscanf(q, "%d", new(int)); n == 1 && err == nil {
		var idx int
		fmt.Sscanf(q, "%d", &idx)
		if idx >= 1 && idx <= len(availableModels) {
			return &availableModels[idx-1]
		}
	}
	// Try alias or id match
	for i := range availableModels {
		if q == availableModels[i].alias || q == availableModels[i].id {
			return &availableModels[i]
		}
	}
	// Try substring match
	for i := range availableModels {
		if strings.Contains(availableModels[i].id, q) || strings.Contains(availableModels[i].alias, q) {
			return &availableModels[i]
		}
	}
	return nil
}

func cmdModel(s *session, args string, rl *readline.Instance) bool {
	if args == "" {
		// Show current model + available models
		fmt.Println()
		fmt.Printf("  %s %s %s\n",
			styleBannerTitle.Render("Model:"),
			s.cfg.Claude.Model,
			styleDim.Render(fmt.Sprintf("(effort: %s)", s.cfg.Claude.Effort)))
		fmt.Println()

		for i, m := range availableModels {
			num := fmt.Sprintf("%d", i+1)
			if m.id == s.cfg.Claude.Model || m.alias == s.cfg.Claude.Model {
				fmt.Printf("  %s %s  %-8s  %s\n",
					styleSuccess.Render("●"),
					stylePrompt.Render(num+"."),
					m.alias,
					styleDim.Render(m.desc))
			} else {
				fmt.Printf("    %s  %-8s  %s\n",
					styleDim.Render(num+"."),
					m.alias,
					styleDim.Render(m.desc))
			}
		}

		fmt.Println()
		fmt.Println(styleDim.Render("  /model <name|number> [effort]"))
		fmt.Println(styleDim.Render("  /model effort <low|medium|high>"))
		fmt.Println()
		return false
	}

	parts := strings.Fields(args)

	// Handle "/model effort <level>"
	if parts[0] == "effort" {
		if len(parts) < 2 {
			fmt.Println(styleInfo.Render(fmt.Sprintf("  Current effort: %s", s.cfg.Claude.Effort)))
			fmt.Println(styleDim.Render(fmt.Sprintf("  Available: %s", strings.Join(validEfforts, ", "))))
			fmt.Println()
			return false
		}
		level := strings.ToLower(parts[1])
		valid := false
		for _, e := range validEfforts {
			if e == level {
				valid = true
				break
			}
		}
		if !valid {
			fmt.Println(styleError.Render(fmt.Sprintf("  Unknown effort level: %s", parts[1])))
			fmt.Println(styleDim.Render(fmt.Sprintf("  Available: %s", strings.Join(validEfforts, ", "))))
			fmt.Println()
			return false
		}
		s.cfg.Claude.Effort = level
		fmt.Println(styleSuccess.Render(fmt.Sprintf("  Effort set to %s", level)))
		fmt.Println()
		return false
	}

	// Find model by query
	m := findModelByQuery(parts[0])
	if m == nil {
		fmt.Println(styleError.Render(fmt.Sprintf("  Unknown model: %s", parts[0])))
		fmt.Println(styleDim.Render("  Available: opus, sonnet, haiku"))
		fmt.Println()
		return false
	}

	s.cfg.Claude.Model = m.id
	if len(parts) > 1 {
		level := strings.ToLower(parts[1])
		valid := false
		for _, e := range validEfforts {
			if e == level {
				valid = true
				break
			}
		}
		if valid {
			s.cfg.Claude.Effort = level
		}
	}

	fmt.Println(styleSuccess.Render(fmt.Sprintf("  Model: %s (%s, effort: %s)", m.id, m.alias, s.cfg.Claude.Effort)))
	fmt.Println()
	return false
}

// validPermissionModes lists the permission modes supported by Claude CLI.
var validPermissionModes = []string{"default", "acceptEdits", "bypassPermissions", "plan"}

func cmdPermissions(s *session, args string, rl *readline.Instance) bool {
	if args == "" {
		perm := s.cfg.Claude.PermissionMode
		if perm == "" {
			perm = "default"
		}
		fmt.Println()
		fmt.Printf("  %s %s\n", styleBannerTitle.Render("Permission mode:"), perm)
		fmt.Println()
		for _, m := range validPermissionModes {
			if m == perm {
				fmt.Printf("  %s %s\n", styleSuccess.Render("●"), m)
			} else {
				fmt.Printf("    %s\n", styleDim.Render(m))
			}
		}
		fmt.Println()
		fmt.Println(styleDim.Render("  /permissions <mode>"))
		fmt.Println()
		return false
	}
	mode := strings.TrimSpace(args)
	valid := false
	for _, m := range validPermissionModes {
		if m == mode {
			valid = true
			break
		}
	}
	if !valid {
		fmt.Println(styleError.Render(fmt.Sprintf("  Unknown permission mode: %s", mode)))
		fmt.Println(styleDim.Render(fmt.Sprintf("  Available: %s", strings.Join(validPermissionModes, ", "))))
		fmt.Println()
		return false
	}
	s.cfg.Claude.PermissionMode = mode
	fmt.Println(styleSuccess.Render("  Permission mode set to " + mode))
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
	fmt.Println()
	fmt.Println("  " + styleRespTitle.Render("◆ Configuration") + styleRespRule.Render(" ───────────────────────────"))
	fmt.Println()
	label := func(name string) string { return styleDim.Render(fmt.Sprintf("  %-14s", name)) }

	fmt.Printf("%s %s %s\n", label("Model"), s.cfg.Claude.Model, styleDim.Render("(effort: "+s.cfg.Claude.Effort+")"))
	perm := s.cfg.Claude.PermissionMode
	if perm == "" {
		perm = "default"
	}
	fmt.Printf("%s %s\n", label("Permissions"), perm)
	fmt.Printf("%s %s\n", label("Sandbox"), s.cfg.Sandbox.Level)
	fmt.Printf("%s %d workers\n", label("Pool"), s.cfg.Pool.MaxConcurrent)
	fmt.Printf("%s %d tokens\n", label("Token budget"), s.cfg.Context.TokenBudget)
	fmt.Printf("%s %ds (long: %ds)\n", label("Timeout"), s.cfg.Claude.Timeout, s.cfg.Claude.LongTaskTimeout)
	fmt.Printf("%s %d attempts, %.1fx backoff\n", label("Retry"), s.cfg.Retry.MaxAttempts, s.cfg.Retry.Multiplier)
	fmt.Printf("%s auto:%s · confirm:%s · reject:%s\n", label("Governance"),
		strings.Join(s.cfg.Governance.AutoApprove, ","),
		strings.Join(s.cfg.Governance.Confirm, ","),
		strings.Join(s.cfg.Governance.Reject, ","))
	fmt.Println()
	return false
}

func cmdTheme(s *session, args string, rl *readline.Instance) bool {
	name := strings.TrimSpace(args)
	if name == "" {
		fmt.Printf("  Current theme: %s\n", activeTheme.Name)
		fmt.Println(styleDim.Render("  Available: dark, light"))
		fmt.Println()
		return false
	}
	if !SetTheme(name) {
		fmt.Println(styleError.Render("  Unknown theme: " + name))
		fmt.Println(styleDim.Render("  Available: dark, light"))
		fmt.Println()
		return false
	}
	fmt.Println(styleSuccess.Render("  Theme set to " + name))
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
	fmt.Println()
	label := func(name string) string { return styleDim.Render(fmt.Sprintf("  %-14s", name)) }
	fmt.Printf("%s %d\n", label("Turns"), len(s.turns))
	var ctxStr string
	if len(ctx) > 1000 {
		ctxStr = fmt.Sprintf("%.1fk chars", float64(len(ctx))/1000)
	} else {
		ctxStr = fmt.Sprintf("%d chars", len(ctx))
	}
	fmt.Printf("%s %s\n", label("Context"), ctxStr)
	fmt.Printf("%s %d\n", label("Attachments"), len(s.attachments))
	if len(s.attachments) > 0 {
		for _, p := range s.attachments {
			fmt.Printf("                %s\n", styleDim.Render(p))
		}
	}
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
	fmt.Println()
	fmt.Println("  " + styleRespTitle.Render("◆ Status") + styleRespRule.Render(" ──────────────────────────────"))
	fmt.Println()

	// Claude CLI version
	cliVersion := styleError.Render("not found")
	if out, err := exec.Command("claude", "--version").Output(); err == nil {
		cliVersion = strings.TrimSpace(string(out))
	}
	label := func(s string) string { return styleDim.Render(fmt.Sprintf("  %-14s", s)) }

	fmt.Printf("%s %s\n", label("Claude CLI"), cliVersion)
	fmt.Printf("%s %s %s\n", label("Model"), s.cfg.Claude.Model, styleDim.Render("(effort: "+s.cfg.Claude.Effort+")"))

	perm := s.cfg.Claude.PermissionMode
	if perm == "" {
		perm = "default"
	}
	fmt.Printf("%s %s\n", label("Permissions"), perm)
	fmt.Printf("%s %s\n", label("Sandbox"), s.cfg.Sandbox.Level)
	fmt.Printf("%s %d workers\n", label("Pool"), s.cfg.Pool.MaxConcurrent)
	fmt.Printf("%s auto:%s · confirm:%s · reject:%s\n", label("Risk gate"),
		strings.Join(s.cfg.Governance.AutoApprove, ","),
		strings.Join(s.cfg.Governance.Confirm, ","),
		strings.Join(s.cfg.Governance.Reject, ","))

	ctx := s.context()
	var ctxStr string
	if len(ctx) > 1000 {
		ctxStr = fmt.Sprintf("%.1fk chars", float64(len(ctx))/1000)
	} else {
		ctxStr = fmt.Sprintf("%d chars", len(ctx))
	}
	fmt.Printf("%s %d turns · %s context\n", label("Session"), len(s.turns), ctxStr)

	report := health.Evaluate(s.cfg.BaseDir)
	healthStyle := styleSuccess
	switch report.Level {
	case health.YELLOW:
		healthStyle = lipgloss.NewStyle().Foreground(activeTheme.Warning)
	case health.RED, health.CRITICAL:
		healthStyle = styleError
	}
	fmt.Printf("%s %s\n", label("Health"), healthStyle.Render(report.Level.String()))

	fmt.Println()
	return false
}

func cmdHistory(s *session, args string, rl *readline.Instance) bool {
	bridgeCobra(showHistory, nil)
	fmt.Println()
	return false
}

func cmdDoctor(s *session, args string, rl *readline.Instance) bool {
	fmt.Println()
	fmt.Println("  " + styleRespTitle.Render("◆ Health Check") + styleRespRule.Render(" ────────────────────────────"))
	fmt.Println()

	check := func(name string, ok bool, detail string) {
		if ok {
			fmt.Printf("  %s %-20s %s\n", styleSuccess.Render("[✓]"), name, styleDim.Render(detail))
		} else {
			fmt.Printf("  %s %-20s %s\n", styleError.Render("[✗]"), name, detail)
		}
	}

	// Claude CLI
	cliOK := false
	cliDetail := "not found in PATH"
	if out, err := exec.Command("claude", "--version").Output(); err == nil {
		cliOK = true
		cliDetail = strings.TrimSpace(string(out))
	}
	check("Claude CLI", cliOK, cliDetail)

	// Config file
	configPath := filepath.Join(s.cfg.BaseDir, "config.yaml")
	configOK := false
	configDetail := configPath + " not found"
	if _, err := os.Stat(configPath); err == nil {
		configOK = true
		configDetail = "valid"
	}
	check("Config", configOK, configDetail)

	// Run full health evaluation
	report := health.Evaluate(s.cfg.BaseDir)
	for _, c := range report.Components {
		check(c.Name, c.Healthy, c.Detail)
	}

	// Summary
	fmt.Println()
	healthStyle := styleSuccess
	switch report.Level {
	case health.YELLOW:
		healthStyle = lipgloss.NewStyle().Foreground(activeTheme.Warning)
	case health.RED, health.CRITICAL:
		healthStyle = styleError
	}
	fmt.Printf("  System health: %s\n", healthStyle.Render(report.Level.String()))
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
	fmt.Println()
	fmt.Printf("  %s %s\n", styleBannerTitle.Render("Apex"), "v0.1.0")
	if out, err := exec.Command("claude", "--version").Output(); err == nil {
		fmt.Printf("  %s %s\n", styleDim.Render("Claude CLI"), strings.TrimSpace(string(out)))
	}
	fmt.Printf("  %s %s/%s\n", styleDim.Render("Platform"), runtime.GOOS, runtime.GOARCH)
	fmt.Println()
	return false
}

func cmdQuit(s *session, args string, rl *readline.Instance) bool {
	return true
}

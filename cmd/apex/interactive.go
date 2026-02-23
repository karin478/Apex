package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

func startInteractive(cmd *cobra.Command, args []string) error {
	// Suppress cobra usage output for interactive mode errors
	cmd.SilenceUsage = true

	if !isTerminal() {
		cmd.SilenceUsage = false // Show usage when no TTY so user sees available commands
		return fmt.Errorf("interactive mode requires a TTY; use 'apex run <task>' for non-interactive execution")
	}

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

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(stylePrompt.Render("apex> "))
		if !scanner.Scan() {
			break // EOF (Ctrl+D)
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Slash commands
		if strings.HasPrefix(input, "/") {
			if s.handleSlash(input) {
				break // /quit or /exit
			}
			continue
		}

		// Execute task
		summary, err := runInteractiveTask(s.cfg, input, s.context())
		if err != nil {
			fmt.Println(styleError.Render("Error: " + err.Error()))
		}
		s.turns = append(s.turns, turn{task: input, summary: summary})
		fmt.Println()
	}

	// Save session on exit
	s.saveSession()
	fmt.Println(styleInfo.Render("Session saved. Goodbye!"))
	return nil
}

// handleSlash processes slash commands. Returns true if the session should exit.
func (s *session) handleSlash(input string) bool {
	switch strings.ToLower(strings.Fields(input)[0]) {
	case "/help":
		fmt.Println(styleBanner.Render("Commands:"))
		cmds := [][2]string{
			{"/help", "Show available commands"},
			{"/status", "Show recent run history"},
			{"/history", "Show task execution history"},
			{"/doctor", "Run system integrity check"},
			{"/clear", "Clear screen"},
			{"/config", "Show current config summary"},
			{"/quit", "Exit session"},
		}
		for _, c := range cmds {
			fmt.Printf("  %-12s %s\n", stylePrompt.Render(c[0]), c[1])
		}
	case "/status":
		if err := showStatus(nil, nil); err != nil {
			fmt.Println(styleError.Render("Error: " + err.Error()))
		}
	case "/history":
		if err := showHistory(nil, nil); err != nil {
			fmt.Println(styleError.Render("Error: " + err.Error()))
		}
	case "/doctor":
		if err := runDoctor(nil, nil); err != nil {
			fmt.Println(styleError.Render("Error: " + err.Error()))
		}
	case "/clear":
		fmt.Print("\033[H\033[2J")
	case "/config":
		fmt.Printf("Model:   %s\n", s.cfg.Claude.Model)
		fmt.Printf("Effort:  %s\n", s.cfg.Claude.Effort)
		fmt.Printf("Sandbox: %s\n", s.cfg.Sandbox.Level)
		fmt.Printf("Pool:    %d workers\n", s.cfg.Pool.MaxConcurrent)
	case "/quit", "/exit":
		s.saveSession()
		return true
	default:
		fmt.Println(styleError.Render("Unknown command. Type /help for available commands."))
	}
	fmt.Println()
	return false
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

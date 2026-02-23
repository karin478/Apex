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

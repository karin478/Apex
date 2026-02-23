package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
	"github.com/lyndonlyu/apex/internal/config"
	"github.com/lyndonlyu/apex/internal/governance"
	"github.com/lyndonlyu/apex/internal/memory"
	"github.com/spf13/cobra"
)

type session struct {
	cfg         *config.Config
	turns       []turn
	lastOutput  string
	attachments []string
	home        string
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

func (s *session) printStatusLine() {
	ctx := s.context()
	ctxLen := len(ctx)
	var ctxStr string
	if ctxLen > 1000 {
		ctxStr = fmt.Sprintf("%.1fk chars", float64(ctxLen)/1000)
	} else {
		ctxStr = fmt.Sprintf("%d chars", ctxLen)
	}
	line := fmt.Sprintf("  %s · context: %s · %d turns · %s",
		s.cfg.Claude.Model, ctxStr, len(s.turns), s.cfg.Sandbox.Level)
	fmt.Println(styleDim.Render(line))
}

func printBanner(cfg *config.Config) {
	cwd, _ := os.Getwd()
	fmt.Println()
	fmt.Println(styleBannerTitle.Render("  ◆ Apex v0.1.0"))
	fmt.Println(styleBannerInfo.Render(fmt.Sprintf("  %s · %s · %s", cfg.Claude.Model, cfg.Sandbox.Level, cwd)))
	fmt.Println()
	fmt.Println(styleDim.Render("  /help for commands · /quit to exit · Tab for autocomplete"))
	fmt.Println()
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

	s := &session{cfg: cfg, home: home}

	printBanner(cfg)

	completer := buildCompleter()

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          stylePrompt.Render("❯ "),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		HistoryFile:     filepath.Join(home, ".apex", "history"),
		AutoComplete:    completer,
	})
	if err != nil {
		return fmt.Errorf("readline init: %w", err)
	}
	defer rl.Close()

	for {
		if len(s.turns) > 0 {
			s.printStatusLine()
		}
		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			continue // Ctrl+C: ignore, show new prompt
		}
		if err == io.EOF {
			break // Ctrl+D: exit
		}
		if err != nil {
			break
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		// Shell escape
		if strings.HasPrefix(input, "!") {
			shellCmd := strings.TrimSpace(input[1:])
			if shellCmd == "" {
				fmt.Println(styleInfo.Render("  Usage: !<command>"))
				fmt.Println()
			} else {
				runShellCommand(shellCmd)
			}
			continue
		}

		// Slash commands via registry
		if strings.HasPrefix(input, "/") {
			fields := strings.Fields(input[1:])
			if len(fields) == 0 {
				continue
			}
			cmdName := fields[0]
			cmdArgs := strings.TrimSpace(strings.TrimPrefix(input[1:], cmdName))
			sc := findCommand(cmdName)
			if sc == nil {
				fmt.Println(styleError.Render("Unknown command. Type /help for available commands."))
				fmt.Println()
			} else if sc.handler(s, cmdArgs, rl) {
				break
			}
			continue
		}

		// Prepend attached file contents to task
		taskInput := input
		if len(s.attachments) > 0 {
			var fileParts []string
			for _, path := range s.attachments {
				data, readErr := os.ReadFile(path)
				if readErr == nil {
					fileParts = append(fileParts, fmt.Sprintf("--- File: %s ---\n%s", path, string(data)))
				}
			}
			if len(fileParts) > 0 {
				taskInput = strings.Join(fileParts, "\n") + "\n\n" + input
			}
			s.attachments = nil
		}

		// Execute task
		fmt.Println() // blank line after input
		summary, err := runInteractiveTask(s.cfg, taskInput, s.context())
		if err != nil {
			fmt.Println(styleError.Render("  Error: " + err.Error()))
		}
		s.lastOutput = summary
		s.turns = append(s.turns, turn{task: input, summary: summary})
		fmt.Println()
	}

	// Save session on exit
	s.saveSession()
	fmt.Println(styleInfo.Render("Session saved. Goodbye!"))
	return nil
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

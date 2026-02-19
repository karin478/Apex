package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/config"
	"github.com/lyndonlyu/apex/internal/redact"
	"github.com/spf13/cobra"
)

var redactCmd = &cobra.Command{
	Use:   "redact",
	Short: "Redaction tools for scrubbing sensitive data",
}

var redactTestCmd = &cobra.Command{
	Use:   "test [input]",
	Short: "Test redaction rules against input text",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		home, _ := os.UserHomeDir()
		cfgPath := filepath.Join(home, ".apex", "config.yaml")
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		redactor := redact.New(cfg.Redaction)
		result := redactor.Redact(args[0])
		fmt.Println(result)
		return nil
	},
}

func init() {
	redactCmd.AddCommand(redactTestCmd)
}

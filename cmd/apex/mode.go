package main

import (
	"fmt"
	"strings"

	"github.com/lyndonlyu/apex/internal/mode"
	"github.com/spf13/cobra"
)

var modeFormat string

var modeCmd = &cobra.Command{
	Use:   "mode",
	Short: "Execution mode management",
}

var modeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available execution modes",
	RunE:  runModeList,
}

var modeSelectCmd = &cobra.Command{
	Use:   "select <mode>",
	Short: "Select an execution mode",
	Args:  cobra.ExactArgs(1),
	RunE:  runModeSelect,
}

var modeConfigCmd = &cobra.Command{
	Use:   "config <mode>",
	Short: "Show configuration for a specific mode",
	Args:  cobra.ExactArgs(1),
	RunE:  runModeConfig,
}

func init() {
	modeListCmd.Flags().StringVar(&modeFormat, "format", "", "Output format (json)")
	modeCmd.AddCommand(modeListCmd, modeSelectCmd, modeConfigCmd)
}

func runModeList(cmd *cobra.Command, args []string) error {
	s := mode.NewSelector(mode.DefaultModes())
	list := s.List()

	if modeFormat == "json" {
		out, err := mode.FormatModeListJSON(list)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(mode.FormatModeList(list))
	}
	return nil
}

func runModeSelect(cmd *cobra.Command, args []string) error {
	s := mode.NewSelector(mode.DefaultModes())
	m := mode.Mode(strings.ToUpper(args[0]))

	if err := s.Select(m); err != nil {
		return err
	}
	current, _ := s.Current()
	fmt.Printf("Mode selected: %s\n", current)
	return nil
}

func runModeConfig(cmd *cobra.Command, args []string) error {
	s := mode.NewSelector(mode.DefaultModes())
	m := mode.Mode(strings.ToUpper(args[0]))

	config, err := s.Config(m)
	if err != nil {
		return err
	}
	fmt.Print(mode.FormatModeConfig(config))
	return nil
}

package main

import (
	"fmt"
	"os"

	"github.com/lyndonlyu/apex/internal/precheck"
	"github.com/spf13/cobra"
)

var precheckFormat string

var precheckCmd = &cobra.Command{
	Use:   "precheck",
	Short: "Run environment precheck",
	RunE:  runPrecheck,
}

func init() {
	precheckCmd.Flags().StringVar(&precheckFormat, "format", "", "Output format (json)")
}

func runPrecheck(cmd *cobra.Command, args []string) error {
	home := os.Getenv("HOME")
	runner := precheck.DefaultRunner(home)
	result := runner.Run()

	if precheckFormat == "json" {
		out, err := precheck.FormatRunResultJSON(result)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(precheck.FormatRunResult(result))
	}
	return nil
}

package main

import (
	"fmt"

	"github.com/lyndonlyu/apex/internal/failclose"
	"github.com/spf13/cobra"
)

var failcloseFormat string

var failcloseCmd = &cobra.Command{
	Use:   "gate",
	Short: "Fail-closed safety gate",
}

var gateCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Run fail-closed gate check",
	RunE:  runGateCheck,
}

var gateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered gate conditions",
	RunE:  runGateList,
}

func init() {
	gateCheckCmd.Flags().StringVar(&failcloseFormat, "format", "", "Output format (json)")
	failcloseCmd.AddCommand(gateCheckCmd, gateListCmd)
}

func runGateCheck(cmd *cobra.Command, args []string) error {
	g := failclose.DefaultGate()
	result := g.Evaluate()

	if failcloseFormat == "json" {
		out, err := failclose.FormatGateResultJSON(result)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(failclose.FormatGateResult(result))
	}
	return nil
}

func runGateList(cmd *cobra.Command, args []string) error {
	g := failclose.DefaultGate()
	fmt.Print(failclose.FormatConditionList(g.Conditions()))
	return nil
}

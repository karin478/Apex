package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "apex",
	Short: "Apex Agent - Claude Code autonomous agent system",
	Long:  "Apex Agent is a CLI tool that orchestrates Claude Code for long-term memory autonomous agent tasks.",
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("apex v0.1.0")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(memoryCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(killSwitchCmd)
	rootCmd.AddCommand(resumeCmd)
	rootCmd.AddCommand(snapshotCmd)
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(pluginCmd)
	rootCmd.AddCommand(redactCmd)
	rootCmd.AddCommand(traceCmd)
	rootCmd.AddCommand(metricsCmd)
	rootCmd.AddCommand(gcCmd)
	rootCmd.AddCommand(hypothesisCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(auditPolicyCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

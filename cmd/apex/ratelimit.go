package main

import (
	"fmt"

	"github.com/lyndonlyu/apex/internal/ratelimit"
	"github.com/spf13/cobra"
)

var ratelimitFormat string

var ratelimitCmd = &cobra.Command{
	Use:   "ratelimit",
	Short: "Manage rate limit groups",
}

var ratelimitStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show rate limit group status",
	RunE:  ratelimitStatus,
}

func init() {
	ratelimitStatusCmd.Flags().StringVar(&ratelimitFormat, "format", "", "Output format (json)")
	ratelimitCmd.AddCommand(ratelimitStatusCmd)
}

func ratelimitStatus(cmd *cobra.Command, args []string) error {
	// No groups are registered at runtime yet; future phases will register
	// groups via configuration. For now, return an empty status.
	var statuses []ratelimit.LimiterStatus

	if ratelimitFormat == "json" {
		fmt.Println(ratelimit.FormatStatusJSON(statuses))
	} else {
		fmt.Print(ratelimit.FormatStatus(statuses))
	}
	return nil
}

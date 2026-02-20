package main

import (
	"fmt"

	"github.com/lyndonlyu/apex/internal/event"
	"github.com/spf13/cobra"
)

var eventFormat string

var eventCmd = &cobra.Command{
	Use:   "event",
	Short: "Manage async event runtime",
}

var eventQueueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Display event queue statistics",
	RunE:  runEventQueue,
}

var eventTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "List registered event types",
	RunE:  runEventTypes,
}

func init() {
	eventQueueCmd.Flags().StringVar(&eventFormat, "format", "", "Output format (json)")
	eventCmd.AddCommand(eventQueueCmd, eventTypesCmd)
}

func runEventQueue(cmd *cobra.Command, args []string) error {
	q := event.NewQueue()
	stats := q.Stats()

	if eventFormat == "json" {
		fmt.Println(event.FormatQueueStatsJSON(stats))
	} else {
		fmt.Print(event.FormatQueueStats(stats))
	}
	return nil
}

func runEventTypes(cmd *cobra.Command, args []string) error {
	r := event.NewRouter()
	types := r.Types()

	fmt.Print(event.FormatTypes(types))
	return nil
}

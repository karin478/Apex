package main

import (
	"fmt"

	"github.com/lyndonlyu/apex/internal/notify"
	"github.com/spf13/cobra"
)

var notifyLevel string

// notifyDispatcher is a package-level dispatcher with default channels and rules.
var notifyDispatcher = func() *notify.Dispatcher {
	d := notify.NewDispatcher()
	_ = d.RegisterChannel(notify.NewStdoutChannel())
	d.AddRule(notify.Rule{EventType: "*", MinLevel: "INFO", Channel: "stdout"})
	return d
}()

var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Notification management",
}

var notifyChannelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "List registered notification channels",
	RunE:  runNotifyChannels,
}

var notifyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List notification rules",
	RunE:  runNotifyList,
}

var notifySendCmd = &cobra.Command{
	Use:   "send <type> <message>",
	Short: "Send a test notification",
	Args:  cobra.ExactArgs(2),
	RunE:  runNotifySend,
}

func init() {
	notifySendCmd.Flags().StringVar(&notifyLevel, "level", "INFO", "Event level (INFO/WARN/ERROR)")
	notifyCmd.AddCommand(notifyChannelsCmd, notifyListCmd, notifySendCmd)
}

func runNotifyChannels(cmd *cobra.Command, args []string) error {
	fmt.Print(notify.FormatChannelList(notifyDispatcher.Channels()))
	return nil
}

func runNotifyList(cmd *cobra.Command, args []string) error {
	fmt.Print(notify.FormatRuleList(notifyDispatcher.Rules()))
	return nil
}

func runNotifySend(cmd *cobra.Command, args []string) error {
	event := notify.Event{
		Type:    args[0],
		Message: args[1],
		Level:   notifyLevel,
	}
	errs := notifyDispatcher.Dispatch(event)
	for _, err := range errs {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %v\n", err)
	}
	return nil
}

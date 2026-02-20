package main

import (
	"fmt"

	"github.com/lyndonlyu/apex/internal/profile"
	"github.com/spf13/cobra"
)

var profileFormat string

// profileRegistry is a package-level registry pre-loaded with defaults.
var profileRegistry = func() *profile.Registry {
	r := profile.NewRegistry()
	for _, p := range profile.DefaultProfiles() {
		_ = r.Register(p)
	}
	return r
}()

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Configuration profile management",
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered profiles",
	RunE:  runProfileList,
}

var profileShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show configuration for a specific profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileShow,
}

var profileActivateCmd = &cobra.Command{
	Use:   "activate <name>",
	Short: "Activate a named profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileActivate,
}

func init() {
	profileListCmd.Flags().StringVar(&profileFormat, "format", "", "Output format (json)")
	profileCmd.AddCommand(profileListCmd, profileShowCmd, profileActivateCmd)
}

func runProfileList(cmd *cobra.Command, args []string) error {
	list := profileRegistry.List()

	if profileFormat == "json" {
		out, err := profile.FormatProfileListJSON(list)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(profile.FormatProfileList(list))
	}
	return nil
}

func runProfileShow(cmd *cobra.Command, args []string) error {
	p, err := profileRegistry.Get(args[0])
	if err != nil {
		return err
	}
	fmt.Print(profile.FormatProfile(p))
	return nil
}

func runProfileActivate(cmd *cobra.Command, args []string) error {
	if err := profileRegistry.Activate(args[0]); err != nil {
		return err
	}
	active, err := profileRegistry.Active()
	if err != nil {
		return err
	}
	fmt.Printf("Profile activated: %s\n", active.Name)
	return nil
}

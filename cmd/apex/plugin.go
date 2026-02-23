package main

import (
	"fmt"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/plugin"
	"github.com/spf13/cobra"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage plugins",
}

var pluginScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan for plugins in ~/.apex/plugins/",
	RunE:  pluginScan,
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all plugins",
	RunE:  pluginList,
}

var pluginEnableCmd = &cobra.Command{
	Use:   "enable [name]",
	Short: "Enable a plugin",
	Args:  cobra.ExactArgs(1),
	RunE:  pluginEnable,
}

var pluginDisableCmd = &cobra.Command{
	Use:   "disable [name]",
	Short: "Disable a plugin",
	Args:  cobra.ExactArgs(1),
	RunE:  pluginDisable,
}

var pluginVerifyCmd = &cobra.Command{
	Use:   "verify [name]",
	Short: "Verify plugin checksum",
	Args:  cobra.ExactArgs(1),
	RunE:  pluginVerify,
}

func init() {
	pluginCmd.AddCommand(pluginScanCmd)
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginEnableCmd)
	pluginCmd.AddCommand(pluginDisableCmd)
	pluginCmd.AddCommand(pluginVerifyCmd)
}

func pluginsDir() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".apex", "plugins"), nil
}

func pluginScan(cmd *cobra.Command, args []string) error {
	pDir, err := pluginsDir()
	if err != nil {
		return err
	}
	reg := plugin.NewRegistry(pDir)
	plugins, err := reg.Scan()
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}
	fmt.Printf("Found %d plugin(s)\n", len(plugins))
	for _, p := range plugins {
		fmt.Printf("  %s v%s [%s]\n", p.Name, p.Version, p.Type)
	}
	return nil
}

func pluginList(cmd *cobra.Command, args []string) error {
	pDir, err := pluginsDir()
	if err != nil {
		return err
	}
	reg := plugin.NewRegistry(pDir)
	if _, err := reg.Scan(); err != nil {
		return err
	}
	plugins := reg.List()
	if len(plugins) == 0 {
		fmt.Println("No plugins found.")
		fmt.Printf("Place plugins in %s/\n", pDir)
		return nil
	}

	fmt.Printf("Plugins (%d found)\n", len(plugins))
	fmt.Println("=================")
	fmt.Println()
	for _, p := range plugins {
		status := "enabled"
		if !p.Enabled {
			status = "disabled"
		}
		fmt.Printf("  %-24s v%-8s [%s]  %s\n", p.Name, p.Version, status, p.Description)
	}
	return nil
}

func pluginEnable(cmd *cobra.Command, args []string) error {
	pDir, err := pluginsDir()
	if err != nil {
		return err
	}
	reg := plugin.NewRegistry(pDir)
	if _, err := reg.Scan(); err != nil {
		return err
	}
	if err := reg.Enable(args[0]); err != nil {
		return err
	}
	fmt.Printf("Plugin %q enabled\n", args[0])
	return nil
}

func pluginDisable(cmd *cobra.Command, args []string) error {
	pDir, err := pluginsDir()
	if err != nil {
		return err
	}
	reg := plugin.NewRegistry(pDir)
	if _, err := reg.Scan(); err != nil {
		return err
	}
	if err := reg.Disable(args[0]); err != nil {
		return err
	}
	fmt.Printf("Plugin %q disabled\n", args[0])
	return nil
}

func pluginVerify(cmd *cobra.Command, args []string) error {
	pDir, err := pluginsDir()
	if err != nil {
		return err
	}
	reg := plugin.NewRegistry(pDir)
	if _, err := reg.Scan(); err != nil {
		return err
	}
	p, ok := reg.Get(args[0])
	if !ok {
		return fmt.Errorf("plugin %q not found", args[0])
	}

	valid, err := reg.Verify(args[0])
	if err != nil {
		return err
	}
	if valid {
		fmt.Printf("Checksum: OK (%s)\n", p.Checksum)
	} else if p.Checksum == "" {
		fmt.Println("Checksum: SKIP (no checksum defined)")
	} else {
		fmt.Printf("Checksum: MISMATCH (expected %s)\n", p.Checksum)
	}
	return nil
}

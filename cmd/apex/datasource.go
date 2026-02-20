package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/datapuller"
	"github.com/spf13/cobra"
)

var datasourceFormat string

var datasourceCmd = &cobra.Command{
	Use:   "datasource",
	Short: "Manage external data sources",
}

var datasourceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured data sources",
	RunE:  runDatasourceList,
}

var datasourcePullCmd = &cobra.Command{
	Use:   "pull [name]",
	Short: "Pull data from a named source",
	Args:  cobra.ExactArgs(1),
	RunE:  runDatasourcePull,
}

var datasourceValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate data source YAML specs",
	RunE:  runDatasourceValidate,
}

func init() {
	datasourceListCmd.Flags().StringVar(&datasourceFormat, "format", "", "Output format (json)")
	datasourceCmd.AddCommand(datasourceListCmd, datasourcePullCmd, datasourceValidateCmd)
}

func datasourceDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("datapuller: home dir: %w", err)
	}
	return filepath.Join(home, ".claude", "data_sources"), nil
}

func loadAllSources() ([]*datapuller.SourceSpec, error) {
	dir, err := datasourceDir()
	if err != nil {
		return nil, err
	}
	return datapuller.LoadDir(dir)
}

func runDatasourceList(cmd *cobra.Command, args []string) error {
	specs, err := loadAllSources()
	if err != nil {
		return err
	}

	if datasourceFormat == "json" {
		out, err := datapuller.FormatSourceListJSON(specs)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(datapuller.FormatSourceList(specs))
	}
	return nil
}

func runDatasourcePull(cmd *cobra.Command, args []string) error {
	specs, err := loadAllSources()
	if err != nil {
		return err
	}

	name := args[0]
	var target *datapuller.SourceSpec
	for _, s := range specs {
		if s.Name == name {
			target = s
			break
		}
	}
	if target == nil {
		return fmt.Errorf("data source %q not found", name)
	}

	result := datapuller.Pull(*target, http.DefaultClient)
	fmt.Print(datapuller.FormatPullResult(result))
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func runDatasourceValidate(cmd *cobra.Command, args []string) error {
	dir, err := datasourceDir()
	if err != nil {
		return err
	}

	var paths []string
	for _, ext := range []string{"*.yaml", "*.yml"} {
		matches, _ := filepath.Glob(filepath.Join(dir, ext))
		paths = append(paths, matches...)
	}

	if len(paths) == 0 {
		fmt.Println("No data source specs found.")
		return nil
	}

	hasErrors := false
	for _, p := range paths {
		spec, err := datapuller.LoadSpec(p)
		if err != nil {
			fmt.Printf("FAIL  %s: %v\n", filepath.Base(p), err)
			hasErrors = true
			continue
		}
		fmt.Printf("OK    %s (%s)\n", filepath.Base(p), spec.Name)
	}
	if hasErrors {
		return fmt.Errorf("one or more specs failed validation")
	}
	return nil
}

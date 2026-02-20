package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/connector"
	"github.com/spf13/cobra"
)

var connectorFormat string

var connectorCmd = &cobra.Command{
	Use:   "connector",
	Short: "Manage external connectors",
}

var connectorListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered connectors",
	RunE:  runConnectorList,
}

var connectorStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show circuit breaker status for all connectors",
	RunE:  runConnectorStatus,
}

func init() {
	connectorStatusCmd.Flags().StringVar(&connectorFormat, "format", "", "Output format (json)")
	connectorCmd.AddCommand(connectorListCmd, connectorStatusCmd)
}

// connectorSpecDir returns the path to the connector spec directory.
func connectorSpecDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("connector: home dir: %w", err)
	}
	return filepath.Join(home, ".claude", "connectors"), nil
}

// loadAllSpecs scans the connector spec directory for YAML files and loads each one.
func loadAllSpecs() ([]*connector.ConnectorSpec, error) {
	dir, err := connectorSpecDir()
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, ext := range []string{"*.yaml", "*.yml"} {
		matches, err := filepath.Glob(filepath.Join(dir, ext))
		if err != nil {
			return nil, fmt.Errorf("connector: glob %s: %w", ext, err)
		}
		paths = append(paths, matches...)
	}

	var specs []*connector.ConnectorSpec
	for _, path := range paths {
		spec, err := connector.LoadSpec(path)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func runConnectorList(cmd *cobra.Command, args []string) error {
	specs, err := loadAllSpecs()
	if err != nil {
		return err
	}

	fmt.Print(connector.FormatConnectorList(specs))
	return nil
}

func runConnectorStatus(cmd *cobra.Command, args []string) error {
	specs, err := loadAllSpecs()
	if err != nil {
		return err
	}

	// Create a registry and register all specs to get breaker statuses.
	reg := connector.NewRegistry()
	for _, spec := range specs {
		if err := reg.Register(spec); err != nil {
			return err
		}
	}

	// Collect breaker statuses.
	statuses := make(map[string]connector.CBStatus)
	for _, spec := range specs {
		st, ok := reg.BreakerStatus(spec.Name)
		if ok {
			statuses[spec.Name] = *st
		}
	}

	if connectorFormat == "json" {
		fmt.Println(connector.FormatBreakerStatusJSON(statuses))
	} else {
		fmt.Print(connector.FormatBreakerStatus(statuses))
	}
	return nil
}

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/statedb"
	"github.com/spf13/cobra"
)

var statedbPath = filepath.Join(os.Getenv("HOME"), ".apex", "runtime.db")

var statedbFormat string
var statedbLimit int

var statedbCmd = &cobra.Command{
	Use:   "statedb",
	Short: "Runtime state database",
}

var statedbStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show database status",
	RunE:  runStateDBStatus,
}

var statedbStateCmd = &cobra.Command{
	Use:   "state",
	Short: "State key-value store",
}

var statedbStateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all state entries",
	RunE:  runStateDBStateList,
}

var statedbRunsCmd = &cobra.Command{
	Use:   "runs",
	Short: "Run records",
}

var statedbRunsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent run records",
	RunE:  runStateDBRunsList,
}

func init() {
	statedbStateListCmd.Flags().StringVar(&statedbFormat, "format", "", "Output format (json)")
	statedbRunsListCmd.Flags().StringVar(&statedbFormat, "format", "", "Output format (json)")
	statedbRunsListCmd.Flags().IntVar(&statedbLimit, "limit", 10, "Number of runs to show")
	statedbStateCmd.AddCommand(statedbStateListCmd)
	statedbRunsCmd.AddCommand(statedbRunsListCmd)
	statedbCmd.AddCommand(statedbStatusCmd, statedbStateCmd, statedbRunsCmd)
}

func runStateDBStatus(cmd *cobra.Command, args []string) error {
	db, err := statedb.Open(statedbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	entries, err := db.ListState()
	if err != nil {
		return err
	}

	runs, err := db.ListRuns(0)
	if err != nil {
		return err
	}

	fmt.Print(statedb.FormatStatus(db.Path(), len(entries), len(runs)))
	return nil
}

func runStateDBStateList(cmd *cobra.Command, args []string) error {
	db, err := statedb.Open(statedbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	entries, err := db.ListState()
	if err != nil {
		return err
	}

	if statedbFormat == "json" {
		out, err := statedb.FormatStateListJSON(entries)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(statedb.FormatStateList(entries))
	}
	return nil
}

func runStateDBRunsList(cmd *cobra.Command, args []string) error {
	db, err := statedb.Open(statedbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	runs, err := db.ListRuns(statedbLimit)
	if err != nil {
		return err
	}

	if statedbFormat == "json" {
		out, err := statedb.FormatRunListJSON(runs)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(statedb.FormatRunList(runs))
	}
	return nil
}

package main

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"github.com/lyndonlyu/apex/internal/migration"
	"github.com/spf13/cobra"
)

var migrationFormat string

var migrationCmd = &cobra.Command{
	Use:   "migration",
	Short: "Manage schema migrations",
}

var migrationStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current vs latest schema version",
	RunE:  runMigrationStatus,
}

var migrationPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "List pending migrations",
	RunE:  runMigrationPlan,
}

func init() {
	migrationStatusCmd.Flags().StringVar(&migrationFormat, "format", "", "Output format (json)")
	migrationCmd.AddCommand(migrationStatusCmd, migrationPlanCmd)
}

func runMigrationStatus(cmd *cobra.Command, args []string) error {
	// Placeholder: use in-memory SQLite with empty registry since no
	// persistent DB path is wired yet.
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return fmt.Errorf("open in-memory db: %w", err)
	}
	defer db.Close()

	r := migration.NewRegistry()

	current, err := migration.GetVersion(db)
	if err != nil {
		return err
	}

	latest := r.Latest()

	if migrationFormat == "json" {
		fmt.Println(migration.FormatStatusJSON(current, latest))
	} else {
		fmt.Print(migration.FormatStatus(current, latest))
	}
	return nil
}

func runMigrationPlan(cmd *cobra.Command, args []string) error {
	// Placeholder: use in-memory SQLite with empty registry since no
	// persistent DB path is wired yet.
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return fmt.Errorf("open in-memory db: %w", err)
	}
	defer db.Close()

	r := migration.NewRegistry()

	pending, err := r.Plan(db)
	if err != nil {
		return err
	}

	fmt.Print(migration.FormatPlan(pending))
	return nil
}

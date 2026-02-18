package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/audit"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Verify system integrity",
	RunE:  runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	auditDir := filepath.Join(home, ".apex", "audit")

	fmt.Println("Apex Doctor")
	fmt.Println("===========")
	fmt.Println()

	fmt.Print("Audit hash chain... ")
	logger, err := audit.NewLogger(auditDir)
	if err != nil {
		fmt.Println("SKIP (no audit directory)")
		return nil
	}

	valid, brokenAt, err := logger.Verify()
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return nil
	}

	if valid {
		fmt.Println("OK")
	} else {
		fmt.Printf("BROKEN at record #%d\n", brokenAt)
		fmt.Println("  The audit log may have been tampered with.")
	}

	return nil
}

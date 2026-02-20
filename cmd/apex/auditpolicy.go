package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/lyndonlyu/apex/internal/audit"
	"github.com/spf13/cobra"
)

var auditPolicyCmd = &cobra.Command{
	Use:   "audit-policy",
	Short: "Show policy change history",
	Long:  "List detected configuration file changes from audit logs.",
	RunE:  showAuditPolicy,
}

func showAuditPolicy(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	auditDir := filepath.Join(home, ".apex", "audit")

	logger, err := audit.NewLogger(auditDir)
	if err != nil {
		return fmt.Errorf("audit init failed: %w", err)
	}

	records, err := logger.Recent(math.MaxInt)
	if err != nil {
		return fmt.Errorf("reading audit log: %w", err)
	}

	var changes []audit.PolicyChange
	for _, r := range records {
		if strings.HasPrefix(r.Task, "[policy_change]") {
			file := strings.TrimPrefix(r.Task, "[policy_change] ")
			parts := strings.SplitN(r.Error, "\u2192", 2)
			old, newC := "", ""
			if len(parts) == 2 {
				old = parts[0]
				newC = parts[1]
			}
			changes = append(changes, audit.PolicyChange{
				File:        file,
				OldChecksum: old,
				NewChecksum: newC,
				Timestamp:   r.Timestamp,
			})
		}
	}

	fmt.Print(audit.FormatPolicyChanges(changes))
	return nil
}

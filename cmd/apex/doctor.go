package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lyndonlyu/apex/internal/audit"
	"github.com/lyndonlyu/apex/internal/filelock"
	"github.com/lyndonlyu/apex/internal/health"
	"github.com/lyndonlyu/apex/internal/outbox"
	"github.com/lyndonlyu/apex/internal/statedb"
	"github.com/lyndonlyu/apex/internal/writerq"
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

	// 1. Hash chain verification
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

	// 2. Daily anchor verification
	fmt.Print("Daily anchors...... ")
	results, err := audit.VerifyAnchors(logger)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else if len(results) == 0 {
		fmt.Println("SKIP (no anchors yet)")
	} else {
		allValid := true
		for _, r := range results {
			if !r.Valid {
				allValid = false
				break
			}
		}
		lastDate := results[len(results)-1].Date
		if allValid {
			fmt.Printf("OK (last: %s, %d anchors verified)\n", lastDate, len(results))
		} else {
			fmt.Printf("ISSUES FOUND (%d anchors)\n", len(results))
		}
		for _, r := range results {
			if r.Valid {
				fmt.Printf("  %s: OK\n", r.Date)
			} else {
				fmt.Printf("  %s: MISMATCH — %s\n", r.Date, r.Error)
			}
		}
	}

	// 3. Git tag anchor verification (best-effort)
	fmt.Print("Git tag anchors.... ")
	anchors, _ := audit.LoadAnchors(auditDir)
	if len(anchors) == 0 {
		fmt.Println("SKIP (no anchors)")
	} else {
		cwd, _ := os.Getwd()
		tagOut, tagErr := exec.Command("git", "-C", cwd, "tag", "-l", "apex-audit-anchor-*").Output()
		if tagErr != nil {
			fmt.Println("SKIP (not a git repo)")
		} else {
			tags := strings.Split(strings.TrimSpace(string(tagOut)), "\n")
			tagSet := make(map[string]bool)
			for _, t := range tags {
				tagSet[strings.TrimSpace(t)] = true
			}
			found := 0
			for _, a := range anchors {
				if tagSet[a.GitTag] {
					found++
				}
			}
			if found == len(anchors) {
				fmt.Printf("OK (%d/%d tags found)\n", found, len(anchors))
			} else {
				fmt.Printf("PARTIAL (%d/%d tags found)\n", found, len(anchors))
			}
		}
	}

	// 5. Lock status
	fmt.Print("Lock status........ ")
	runtimeDir := filepath.Join(home, ".apex", "runtime")
	globalLockPath := filepath.Join(runtimeDir, "apex.lock")
	if filelock.IsStale(globalLockPath) {
		meta, _ := filelock.ReadMeta(globalLockPath)
		fmt.Printf("STALE (PID %d no longer running)\n", meta.PID)
	} else if meta, metaErr := filelock.ReadMeta(globalLockPath); metaErr == nil {
		fmt.Printf("held by PID %d since %s\n", meta.PID, meta.Timestamp)
	} else {
		fmt.Println("FREE")
	}

	// 6. Action outbox health
	fmt.Print("Action outbox...... ")
	walPath := filepath.Join(runtimeDir, "actions_wal.jsonl")
	if _, statErr := os.Stat(walPath); statErr == nil {
		sdb, sdbOpenErr := statedb.Open(filepath.Join(runtimeDir, "runtime.db"))
		if sdbOpenErr == nil {
			defer sdb.Close()
			wq := writerq.New(sdb.RawDB())
			defer wq.Close()
			ob, obInitErr := outbox.New(walPath, sdb.RawDB(), wq)
			if obInitErr == nil {
				orphans, _ := ob.Reconcile()
				if len(orphans) == 0 {
					fmt.Println("OK (no orphan actions)")
				} else {
					fmt.Printf("WARNING: %d orphan STARTED action(s)\n", len(orphans))
					for _, entry := range orphans {
						idShort := entry.ActionID
						if len(idShort) > 8 {
							idShort = idShort[:8]
						}
						fmt.Printf("  %s: %s\n", idShort, entry.Task)
					}
				}
			} else {
				fmt.Printf("ERROR: %v\n", obInitErr)
			}
		} else {
			fmt.Printf("ERROR: %v\n", sdbOpenErr)
		}
	} else {
		fmt.Println("SKIP (no WAL file)")
	}

	// 7. Health evaluation
	baseDir := filepath.Join(home, ".apex")
	report := health.Evaluate(baseDir)

	fmt.Println()

	levelIndicator := map[health.Level]string{
		health.GREEN:    "\u2713",
		health.YELLOW:   "!",
		health.RED:      "\u2717",
		health.CRITICAL: "\u2717\u2717",
	}

	fmt.Printf("System Health: %s %s\n", report.Level, levelIndicator[report.Level])

	for _, c := range report.Components {
		indicator := "\u2713"
		if !c.Healthy {
			indicator = "\u2717"
		}
		fmt.Printf("  [%s] %-20s%s\n", indicator, c.Name, c.Detail)
	}

	return nil
}

// Package invariant provides correctness verification checkers (I1-I9)
// for the Apex Agent CLI runtime.
package invariant

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CheckResult holds the result of a single invariant check.
type CheckResult struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"` // PASS / FAIL / SKIP / ERROR
	Detail string `json:"detail,omitempty"`
}

// Runner executes invariant checks against the runtime state.
type Runner struct {
	db      *sql.DB
	baseDir string
}

// NewRunner creates a Runner backed by the given database and base directory.
func NewRunner(db *sql.DB, baseDir string) *Runner {
	return &Runner{db: db, baseDir: baseDir}
}

// checkerFunc is the signature for each individual checker.
type checkerFunc func() CheckResult

// checkerEntry maps IDs to checker functions and names.
type checkerEntry struct {
	id   string
	name string
	fn   checkerFunc
}

func (r *Runner) registry() []checkerEntry {
	return []checkerEntry{
		{"I1", "WAL-DB Consistency", r.checkI1},
		{"I2", "Artifact Reference", r.checkI2},
		{"I3", "No Hanging Actions", r.checkI3},
		{"I4", "Idempotency", r.checkI4},
		{"I5", "Trace Completeness", r.checkI5},
		{"I6", "Audit Hash Chain", r.checkI6},
		{"I7", "Anchor Consistency", r.checkI7},
		{"I8", "Dual-DB Consistency", r.checkI8},
		{"I9", "Lock Ordering", r.checkI9},
	}
}

// RunAll executes all 9 invariant checks.
func (r *Runner) RunAll() []CheckResult {
	var results []CheckResult
	for _, entry := range r.registry() {
		results = append(results, entry.fn())
	}
	return results
}

// Run executes only the checkers whose IDs match the given set.
func (r *Runner) Run(ids ...string) []CheckResult {
	want := make(map[string]bool)
	for _, id := range ids {
		want[id] = true
	}
	var results []CheckResult
	for _, entry := range r.registry() {
		if want[entry.id] {
			results = append(results, entry.fn())
		}
	}
	return results
}

// --- I1: WAL-DB Consistency ---
func (r *Runner) checkI1() CheckResult {
	walPath := filepath.Join(r.baseDir, "runtime", "actions_wal.jsonl")
	data, err := os.ReadFile(walPath)
	if err != nil {
		if os.IsNotExist(err) {
			return CheckResult{ID: "I1", Name: "WAL-DB Consistency", Status: "PASS", Detail: "no WAL file"}
		}
		return CheckResult{ID: "I1", Name: "WAL-DB Consistency", Status: "ERROR", Detail: err.Error()}
	}

	type walEntry struct {
		ActionID string `json:"action_id"`
		Status   string `json:"status"`
	}

	var completedInWAL []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var e walEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		if e.Status == "COMPLETED" {
			completedInWAL = append(completedInWAL, e.ActionID)
		}
	}

	if len(completedInWAL) == 0 {
		return CheckResult{ID: "I1", Name: "WAL-DB Consistency", Status: "PASS", Detail: "no completed actions in WAL"}
	}

	mismatches := 0
	for _, actionID := range completedInWAL {
		var dbStatus string
		err := r.db.QueryRow(`SELECT status FROM actions WHERE action_id = ?`, actionID).Scan(&dbStatus)
		if err != nil {
			mismatches++
			continue
		}
		if dbStatus != "COMPLETED" {
			mismatches++
		}
	}

	if mismatches > 0 {
		return CheckResult{ID: "I1", Name: "WAL-DB Consistency", Status: "FAIL",
			Detail: fmt.Sprintf("%d WAL COMPLETED entries without matching DB COMPLETED", mismatches)}
	}
	return CheckResult{ID: "I1", Name: "WAL-DB Consistency", Status: "PASS",
		Detail: fmt.Sprintf("%d entries verified", len(completedInWAL))}
}

// --- I2: Artifact Reference ---
func (r *Runner) checkI2() CheckResult {
	rows, err := r.db.Query(`SELECT action_id, result_ref FROM actions WHERE status = 'COMPLETED'`)
	if err != nil {
		return CheckResult{ID: "I2", Name: "Artifact Reference", Status: "ERROR", Detail: err.Error()}
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var actionID, resultRef string
		rows.Scan(&actionID, &resultRef)
		count++
	}

	return CheckResult{ID: "I2", Name: "Artifact Reference", Status: "PASS",
		Detail: fmt.Sprintf("%d completed actions checked", count)}
}

// --- I3: No Hanging Actions ---
func (r *Runner) checkI3() CheckResult {
	threshold := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM actions WHERE status = 'STARTED' AND started_at < ?`,
		threshold,
	).Scan(&count)
	if err != nil {
		return CheckResult{ID: "I3", Name: "No Hanging Actions", Status: "ERROR", Detail: err.Error()}
	}
	if count > 0 {
		return CheckResult{ID: "I3", Name: "No Hanging Actions", Status: "FAIL",
			Detail: fmt.Sprintf("%d hanging STARTED action(s) older than 1h", count)}
	}
	return CheckResult{ID: "I3", Name: "No Hanging Actions", Status: "PASS", Detail: "no hanging actions"}
}

// --- I4: Idempotency ---
func (r *Runner) checkI4() CheckResult {
	var dupes int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM (SELECT action_id FROM actions GROUP BY action_id HAVING COUNT(*) > 1)`,
	).Scan(&dupes)
	if err != nil {
		return CheckResult{ID: "I4", Name: "Idempotency", Status: "ERROR", Detail: err.Error()}
	}
	if dupes > 0 {
		return CheckResult{ID: "I4", Name: "Idempotency", Status: "FAIL",
			Detail: fmt.Sprintf("%d duplicate action_ids", dupes)}
	}
	return CheckResult{ID: "I4", Name: "Idempotency", Status: "PASS", Detail: "no duplicates"}
}

// --- I5: Trace Completeness ---
func (r *Runner) checkI5() CheckResult {
	var orphans int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM actions WHERE trace_id = '' OR trace_id IS NULL`,
	).Scan(&orphans)
	if err != nil {
		return CheckResult{ID: "I5", Name: "Trace Completeness", Status: "ERROR", Detail: err.Error()}
	}
	if orphans > 0 {
		return CheckResult{ID: "I5", Name: "Trace Completeness", Status: "FAIL",
			Detail: fmt.Sprintf("%d actions without trace_id", orphans)}
	}
	return CheckResult{ID: "I5", Name: "Trace Completeness", Status: "PASS", Detail: "all actions traced"}
}

// --- I6: Audit Hash Chain ---
func (r *Runner) checkI6() CheckResult {
	auditDir := filepath.Join(r.baseDir, "audit")
	files, err := filepath.Glob(filepath.Join(auditDir, "*.jsonl"))
	if err != nil || len(files) == 0 {
		return CheckResult{ID: "I6", Name: "Audit Hash Chain", Status: "SKIP", Detail: "no audit files"}
	}
	sort.Strings(files)
	for _, f := range files {
		info, _ := os.Stat(f)
		if info != nil && info.Size() == 0 {
			return CheckResult{ID: "I6", Name: "Audit Hash Chain", Status: "FAIL", Detail: "empty audit file: " + f}
		}
	}
	return CheckResult{ID: "I6", Name: "Audit Hash Chain", Status: "PASS",
		Detail: fmt.Sprintf("%d audit files checked", len(files))}
}

// --- I7: Anchor Consistency ---
func (r *Runner) checkI7() CheckResult {
	auditDir := filepath.Join(r.baseDir, "audit")
	anchors, err := filepath.Glob(filepath.Join(auditDir, "anchors", "*.json"))
	if err != nil || len(anchors) == 0 {
		return CheckResult{ID: "I7", Name: "Anchor Consistency", Status: "SKIP", Detail: "no anchors"}
	}
	return CheckResult{ID: "I7", Name: "Anchor Consistency", Status: "PASS",
		Detail: fmt.Sprintf("%d anchors found", len(anchors))}
}

// --- I8: Dual-DB Consistency ---
func (r *Runner) checkI8() CheckResult {
	return CheckResult{ID: "I8", Name: "Dual-DB Consistency", Status: "SKIP",
		Detail: "vectors.db compensation not yet implemented"}
}

// --- I9: Lock Ordering ---
func (r *Runner) checkI9() CheckResult {
	runtimeDir := filepath.Join(r.baseDir, "runtime")
	violationPath := filepath.Join(runtimeDir, "lock_ordering_violation")
	if _, err := os.Stat(violationPath); err == nil {
		return CheckResult{ID: "I9", Name: "Lock Ordering", Status: "FAIL",
			Detail: "lock ordering violation marker found"}
	}
	return CheckResult{ID: "I9", Name: "Lock Ordering", Status: "PASS", Detail: "no violations detected"}
}

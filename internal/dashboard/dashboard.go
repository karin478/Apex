// Package dashboard aggregates data from health, manifest, metrics, and
// audit subsystems into a unified status view.
package dashboard

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/lyndonlyu/apex/internal/audit"
	"github.com/lyndonlyu/apex/internal/health"
	"github.com/lyndonlyu/apex/internal/manifest"
)

// Section represents one block of the dashboard output.
type Section struct {
	Title   string
	Content string
}

// Dashboard aggregates subsystem data rooted at baseDir.
type Dashboard struct {
	baseDir string
}

// New creates a Dashboard that reads data from the given base directory.
func New(baseDir string) *Dashboard {
	return &Dashboard{baseDir: baseDir}
}

// Generate collects data from all subsystems and returns five sections:
// System Health, Recent Runs, Metrics Summary, Policy Changes, and
// Audit Integrity.
func (d *Dashboard) Generate() ([]Section, error) {
	sections := make([]Section, 0, 5)

	s, err := d.healthSection()
	if err != nil {
		return nil, fmt.Errorf("health section: %w", err)
	}
	sections = append(sections, s)

	s, err = d.runsSection()
	if err != nil {
		return nil, fmt.Errorf("runs section: %w", err)
	}
	sections = append(sections, s)

	s, err = d.metricsSection()
	if err != nil {
		return nil, fmt.Errorf("metrics section: %w", err)
	}
	sections = append(sections, s)

	s, err = d.policySection()
	if err != nil {
		return nil, fmt.Errorf("policy section: %w", err)
	}
	sections = append(sections, s)

	s, err = d.auditSection()
	if err != nil {
		return nil, fmt.Errorf("audit section: %w", err)
	}
	sections = append(sections, s)

	return sections, nil
}

// healthSection evaluates system health and formats component status.
func (d *Dashboard) healthSection() (Section, error) {
	report := health.Evaluate(d.baseDir)

	var b strings.Builder
	fmt.Fprintf(&b, "Level: %s\n", report.Level.String())
	for _, c := range report.Components {
		status := "OK"
		if !c.Healthy {
			status = "FAIL"
		}
		fmt.Fprintf(&b, "  %s: %s (%s)\n", c.Name, status, c.Detail)
	}

	return Section{Title: "System Health", Content: b.String()}, nil
}

// runsSection loads the last 5 manifests and formats a table.
func (d *Dashboard) runsSection() (Section, error) {
	runsDir := filepath.Join(d.baseDir, "runs")
	store := manifest.NewStore(runsDir)

	runs, err := store.Recent(5)
	if err != nil {
		return Section{}, fmt.Errorf("load recent runs: %w", err)
	}

	if len(runs) == 0 {
		return Section{
			Title:   "Recent Runs",
			Content: "No runs recorded.",
		}, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%-12s %-30s %-10s %10s\n", "ID", "Task", "Outcome", "Duration")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 66))
	for _, m := range runs {
		dur := fmt.Sprintf("%dms", m.DurationMs)
		fmt.Fprintf(&b, "%-12s %-30s %-10s %10s\n", m.RunID, m.Task, m.Outcome, dur)
	}

	return Section{Title: "Recent Runs", Content: b.String()}, nil
}

// metricsSection computes aggregate statistics from all manifests.
func (d *Dashboard) metricsSection() (Section, error) {
	runsDir := filepath.Join(d.baseDir, "runs")
	store := manifest.NewStore(runsDir)

	// Load all manifests (use a large number).
	all, err := store.Recent(math.MaxInt32)
	if err != nil {
		return Section{}, fmt.Errorf("load all runs: %w", err)
	}

	if len(all) == 0 {
		return Section{
			Title:   "Metrics Summary",
			Content: "No data available.",
		}, nil
	}

	total := len(all)
	var successes int
	var totalDuration int64
	var totalNodes int
	for _, m := range all {
		if m.Outcome == "success" {
			successes++
		}
		totalDuration += m.DurationMs
		totalNodes += m.NodeCount
	}

	rate := float64(successes) / float64(total) * 100
	avgDur := float64(totalDuration) / float64(total)

	var b strings.Builder
	fmt.Fprintf(&b, "Total runs: %d\n", total)
	fmt.Fprintf(&b, "Success rate: %.1f%%\n", rate)
	fmt.Fprintf(&b, "Avg duration: %.0fms\n", avgDur)
	fmt.Fprintf(&b, "Total nodes: %d\n", totalNodes)

	return Section{Title: "Metrics Summary", Content: b.String()}, nil
}

// policySection counts audit entries with a [policy_change] task prefix.
func (d *Dashboard) policySection() (Section, error) {
	auditDir := filepath.Join(d.baseDir, "audit")
	logger, err := audit.NewLogger(auditDir)
	if err != nil {
		return Section{}, fmt.Errorf("open audit logger: %w", err)
	}

	// Load a generous number of recent records to scan.
	records, err := logger.Recent(1000)
	if err != nil {
		return Section{}, fmt.Errorf("load audit records: %w", err)
	}

	var count int
	for _, r := range records {
		if strings.HasPrefix(r.Task, "[policy_change]") {
			count++
		}
	}

	content := "No policy changes detected."
	if count > 0 {
		content = fmt.Sprintf("%d policy change(s) detected.", count)
	}

	return Section{Title: "Policy Changes", Content: content}, nil
}

// auditSection verifies the audit hash chain integrity.
func (d *Dashboard) auditSection() (Section, error) {
	auditDir := filepath.Join(d.baseDir, "audit")
	logger, err := audit.NewLogger(auditDir)
	if err != nil {
		return Section{}, fmt.Errorf("open audit logger: %w", err)
	}

	valid, idx, err := logger.Verify()
	if err != nil {
		return Section{}, fmt.Errorf("verify audit chain: %w", err)
	}

	content := "Chain verified."
	if !valid {
		content = fmt.Sprintf("TAMPER DETECTED at entry %d", idx)
	}

	return Section{Title: "Audit Integrity", Content: content}, nil
}

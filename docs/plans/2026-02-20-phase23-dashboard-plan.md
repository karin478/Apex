# Phase 23: System Dashboard — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Generate a system status overview via `apex dashboard` combining health, runs, metrics, policy changes, and audit integrity into one view.

**Architecture:** New `internal/dashboard` package reads from existing subsystems (health, manifest, metrics, audit) and renders Section-based output. Two formats: terminal (box-drawing) and markdown.

**Tech Stack:** Go, Cobra CLI, Testify, existing internal packages

---

## Task 1: Dashboard Core — Types + Generate

**Files:**
- Create: `internal/dashboard/dashboard.go`
- Create: `internal/dashboard/dashboard_test.go`

**Implementation:** `dashboard.go`

```go
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

// Dashboard aggregates system status from multiple subsystems.
type Dashboard struct {
	baseDir string
}

// New creates a Dashboard rooted at baseDir (~/.apex).
func New(baseDir string) *Dashboard {
	return &Dashboard{baseDir: baseDir}
}

// Generate collects data from all subsystems and returns sections.
func (d *Dashboard) Generate() ([]Section, error) {
	var sections []Section

	// 1. System Health
	report := health.Evaluate(d.baseDir)
	sections = append(sections, d.healthSection(report))

	// 2. Recent Runs
	runsDir := filepath.Join(d.baseDir, "runs")
	store := manifest.NewStore(runsDir)
	manifests, err := store.Recent(5)
	if err == nil {
		sections = append(sections, d.runsSection(manifests))
	}

	// 3. Metrics Summary
	all, err := store.Recent(math.MaxInt)
	if err == nil {
		sections = append(sections, d.metricsSection(all))
	}

	// 4. Policy Changes
	auditDir := filepath.Join(d.baseDir, "audit")
	logger, logErr := audit.NewLogger(auditDir)
	if logErr == nil {
		sections = append(sections, d.policySection(logger))
	}

	// 5. Audit Integrity
	if logger != nil {
		sections = append(sections, d.auditSection(logger))
	}

	return sections, nil
}

func (d *Dashboard) healthSection(report *health.Report) Section {
	var checks []string
	for _, c := range report.Components {
		status := "OK"
		if !c.Healthy {
			status = "FAIL"
		}
		checks = append(checks, fmt.Sprintf("%s %s", c.Name, status))
	}
	content := fmt.Sprintf("Level: %s\nChecks: %s", report.Level, strings.Join(checks, " | "))
	return Section{Title: "System Health", Content: content}
}

func (d *Dashboard) runsSection(manifests []*manifest.Manifest) Section {
	if len(manifests) == 0 {
		return Section{Title: "Recent Runs", Content: "No runs recorded."}
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("%-12s | %-25s | %-10s | %s", "ID", "Task", "Outcome", "Duration"))
	lines = append(lines, strings.Repeat("-", 65))
	for _, m := range manifests {
		id := m.RunID
		if len(id) > 10 {
			id = id[:10] + ".."
		}
		task := m.Task
		if len(task) > 23 {
			task = task[:23] + ".."
		}
		lines = append(lines, fmt.Sprintf("%-12s | %-25s | %-10s | %dms", id, task, m.Outcome, m.DurationMs))
	}
	return Section{Title: fmt.Sprintf("Recent Runs (%d)", len(manifests)), Content: strings.Join(lines, "\n")}
}

func (d *Dashboard) metricsSection(all []*manifest.Manifest) Section {
	total := len(all)
	if total == 0 {
		return Section{Title: "Metrics Summary", Content: "No data available."}
	}
	success := 0
	var totalDuration int64
	totalNodes := 0
	for _, m := range all {
		if m.Outcome == "success" {
			success++
		}
		totalDuration += m.DurationMs
		totalNodes += m.NodeCount
	}
	rate := float64(success) / float64(total) * 100
	avg := totalDuration / int64(total)
	content := fmt.Sprintf("Total runs: %d | Success rate: %.1f%%\nAvg duration: %dms | Total nodes: %d", total, rate, avg, totalNodes)
	return Section{Title: "Metrics Summary", Content: content}
}

func (d *Dashboard) policySection(logger *audit.Logger) Section {
	records, err := logger.Recent(math.MaxInt)
	if err != nil {
		return Section{Title: "Policy Changes", Content: "Unable to read audit log."}
	}
	var count int
	for _, r := range records {
		if strings.HasPrefix(r.Task, "[policy_change]") {
			count++
		}
	}
	if count == 0 {
		return Section{Title: "Policy Changes", Content: "No policy changes detected."}
	}
	return Section{Title: "Policy Changes", Content: fmt.Sprintf("%d policy change(s) recorded. Run 'apex audit-policy' for details.", count)}
}

func (d *Dashboard) auditSection(logger *audit.Logger) Section {
	valid, idx, err := logger.Verify()
	if err != nil {
		return Section{Title: "Audit Integrity", Content: fmt.Sprintf("Verification error: %v", err)}
	}
	if !valid {
		return Section{Title: "Audit Integrity", Content: fmt.Sprintf("TAMPER DETECTED at entry %d! Run 'apex doctor' to investigate.", idx)}
	}
	return Section{Title: "Audit Integrity", Content: "Chain verified, no tampering detected."}
}
```

**Tests (4):**

```go
package dashboard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"audit", "runs", "memory/decisions", "memory/facts", "memory/sessions"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, sub), 0755))
	}
	return dir
}

func TestGenerateEmpty(t *testing.T) {
	dir := setupTestDir(t)
	d := New(dir)
	sections, err := d.Generate()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(sections), 3, "should have at least health, runs, metrics sections")
}

func TestGenerateHealthSection(t *testing.T) {
	dir := setupTestDir(t)
	d := New(dir)
	sections, err := d.Generate()
	require.NoError(t, err)
	assert.Equal(t, "System Health", sections[0].Title)
	assert.Contains(t, sections[0].Content, "Level:")
}

func TestGenerateRunsEmpty(t *testing.T) {
	dir := setupTestDir(t)
	d := New(dir)
	sections, err := d.Generate()
	require.NoError(t, err)
	// Find runs section
	for _, s := range sections {
		if s.Title == "Recent Runs" || strings.HasPrefix(s.Title, "Recent Runs") {
			assert.Contains(t, s.Content, "No runs recorded")
			return
		}
	}
	t.Fatal("Runs section not found")
}

func TestGenerateWithManifest(t *testing.T) {
	dir := setupTestDir(t)
	// Write a fake manifest
	runDir := filepath.Join(dir, "runs", "run-test")
	require.NoError(t, os.MkdirAll(runDir, 0755))
	data := []byte(`{"run_id":"run-test","task":"hello","outcome":"success","duration_ms":100,"node_count":2,"nodes":[]}`)
	require.NoError(t, os.WriteFile(filepath.Join(runDir, "manifest.json"), data, 0644))

	d := New(dir)
	sections, err := d.Generate()
	require.NoError(t, err)

	// Check runs section
	var found bool
	for _, s := range sections {
		if strings.HasPrefix(s.Title, "Recent Runs") {
			assert.Contains(t, s.Content, "run-test")
			assert.Contains(t, s.Content, "hello")
			found = true
		}
	}
	assert.True(t, found, "should have runs section with manifest data")

	// Check metrics section
	for _, s := range sections {
		if s.Title == "Metrics Summary" {
			assert.Contains(t, s.Content, "Total runs: 1")
			assert.Contains(t, s.Content, "100.0%")
		}
	}
}
```

Note: tests need `"strings"` import.

**Commit:** `feat(dashboard): add Dashboard with Generate and section builders`

---

## Task 2: Format Functions — Terminal + Markdown

**Files:**
- Create: `internal/dashboard/format.go`
- Create: `internal/dashboard/format_test.go`

**Implementation:** `format.go`

```go
package dashboard

import (
	"fmt"
	"strings"
)

// FormatTerminal renders sections with box-drawing characters for terminal display.
func FormatTerminal(sections []Section) string {
	var sb strings.Builder

	sb.WriteString("╔══════════════════════════════════════╗\n")
	sb.WriteString("║       APEX SYSTEM DASHBOARD         ║\n")
	sb.WriteString("╚══════════════════════════════════════╝\n\n")

	for _, s := range sections {
		sb.WriteString(fmt.Sprintf("── %s %s\n", s.Title, strings.Repeat("─", max(0, 38-len(s.Title)))))
		sb.WriteString(s.Content)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// FormatMarkdown renders sections as Markdown.
func FormatMarkdown(sections []Section) string {
	var sb strings.Builder

	sb.WriteString("# Apex System Dashboard\n\n")

	for _, s := range sections {
		sb.WriteString(fmt.Sprintf("## %s\n\n", s.Title))
		sb.WriteString(s.Content)
		sb.WriteString("\n\n")
	}

	return sb.String()
}
```

**Tests (2):**

```go
package dashboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatTerminal(t *testing.T) {
	sections := []Section{
		{Title: "Health", Content: "Level: GREEN"},
		{Title: "Runs", Content: "No runs recorded."},
	}
	out := FormatTerminal(sections)
	assert.Contains(t, out, "APEX SYSTEM DASHBOARD")
	assert.Contains(t, out, "Health")
	assert.Contains(t, out, "GREEN")
	assert.Contains(t, out, "──")
}

func TestFormatMarkdown(t *testing.T) {
	sections := []Section{
		{Title: "Health", Content: "Level: GREEN"},
	}
	out := FormatMarkdown(sections)
	assert.Contains(t, out, "# Apex System Dashboard")
	assert.Contains(t, out, "## Health")
	assert.Contains(t, out, "GREEN")
}
```

**Commit:** `feat(dashboard): add FormatTerminal and FormatMarkdown functions`

---

## Task 3: CLI Command — `apex dashboard`

**Files:**
- Create: `cmd/apex/dashboard.go`
- Modify: `cmd/apex/main.go` (add `rootCmd.AddCommand(dashboardCmd)`)

**Implementation:** `dashboard.go`

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/dashboard"
	"github.com/spf13/cobra"
)

var dashboardFormat string

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Show system status overview",
	Long:  "Generate a dashboard with health, runs, metrics, policy changes, and audit integrity.",
	RunE:  showDashboard,
}

func init() {
	dashboardCmd.Flags().StringVar(&dashboardFormat, "format", "terminal", "Output format: terminal or md")
}

func showDashboard(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, ".apex")

	d := dashboard.New(baseDir)
	sections, err := d.Generate()
	if err != nil {
		return fmt.Errorf("dashboard generation failed: %w", err)
	}

	switch dashboardFormat {
	case "md":
		fmt.Print(dashboard.FormatMarkdown(sections))
	default:
		fmt.Print(dashboard.FormatTerminal(sections))
	}
	return nil
}
```

**Register in main.go:** Add `rootCmd.AddCommand(dashboardCmd)` after `auditPolicyCmd`.

**Commit:** `feat(cli): add apex dashboard command`

---

## Task 4: E2E Tests

**Files:**
- Create: `e2e/dashboard_test.go`

**Tests (3):**

```go
package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboardEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, code := env.runApex("dashboard")
	assert.Equal(t, 0, code, "apex dashboard should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "APEX SYSTEM DASHBOARD")
	assert.Contains(t, stdout, "System Health")
	assert.Contains(t, stdout, "No runs recorded")
}

func TestDashboardAfterRun(t *testing.T) {
	env := newTestEnv(t)

	env.runApex("run", "say hello")

	stdout, stderr, code := env.runApex("dashboard")
	assert.Equal(t, 0, code, "apex dashboard should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "APEX SYSTEM DASHBOARD")
	assert.Contains(t, stdout, "Recent Runs")
	assert.Contains(t, stdout, "say hello")
}

func TestDashboardMarkdown(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, code := env.runApex("dashboard", "--format", "md")
	assert.Equal(t, 0, code, "apex dashboard --format md should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "# Apex System Dashboard")
	assert.Contains(t, stdout, "## System Health")
}
```

**Commit:** `test(e2e): add dashboard E2E tests`

---

## Task 5: Update PROGRESS.md

Update Phase table, test counts, add dashboard package.

**Commit:** `docs: mark Phase 23 System Dashboard as complete`

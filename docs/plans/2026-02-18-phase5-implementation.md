# Phase 5 Observability Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add hash chain audit integrity, run manifests, `apex status` and `apex doctor` commands for system observability.

**Architecture:** Upgrade existing `internal/audit` with SHA-256 hash chain. New `internal/manifest` for run metadata. Two new CLI commands.

**Tech Stack:** Go, crypto/sha256, Testify, Cobra

---

### Task 1: Hash Chain Audit Upgrade

**Files:**
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/audit/logger.go`
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/audit/logger_test.go`

**Step 1: Write failing tests**

Add these tests to `logger_test.go`:

```go
func TestHashChain(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	// Log 3 entries
	for i := 0; i < 3; i++ {
		err = logger.Log(Entry{
			Task:      fmt.Sprintf("task-%d", i),
			RiskLevel: "LOW",
			Outcome:   "success",
			Duration:  time.Second,
			Model:     "claude-opus-4-6",
		})
		require.NoError(t, err)
	}

	// Read records and verify chain
	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(dir, today+".jsonl")
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 3)

	var records []Record
	for _, line := range lines {
		var r Record
		require.NoError(t, json.Unmarshal([]byte(line), &r))
		records = append(records, r)
	}

	// First record: prev_hash is empty
	assert.Empty(t, records[0].PrevHash)
	assert.NotEmpty(t, records[0].Hash)

	// Subsequent records: prev_hash matches previous hash
	assert.Equal(t, records[0].Hash, records[1].PrevHash)
	assert.Equal(t, records[1].Hash, records[2].PrevHash)

	// Each hash is unique
	assert.NotEqual(t, records[0].Hash, records[1].Hash)
}

func TestVerifyChainValid(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		logger.Log(Entry{
			Task: fmt.Sprintf("task-%d", i), RiskLevel: "LOW",
			Outcome: "success", Duration: time.Second, Model: "test",
		})
	}

	valid, brokenAt, err := logger.Verify()
	require.NoError(t, err)
	assert.True(t, valid)
	assert.Equal(t, -1, brokenAt)
}

func TestVerifyChainBroken(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		logger.Log(Entry{
			Task: fmt.Sprintf("task-%d", i), RiskLevel: "LOW",
			Outcome: "success", Duration: time.Second, Model: "test",
		})
	}

	// Tamper with the second record
	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(dir, today+".jsonl")
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	// Modify second line's task
	var r Record
	json.Unmarshal([]byte(lines[1]), &r)
	r.Task = "TAMPERED"
	tampered, _ := json.Marshal(r)
	lines[1] = string(tampered)
	os.WriteFile(logFile, []byte(strings.Join(lines, "\n")+"\n"), 0644)

	// Re-init logger to clear cached state
	logger2, _ := NewLogger(dir)
	valid, brokenAt, err := logger2.Verify()
	require.NoError(t, err)
	assert.False(t, valid)
	assert.Equal(t, 1, brokenAt) // broken at second record (index 1)
}

func TestHashChainPersistence(t *testing.T) {
	dir := t.TempDir()

	// First logger session
	logger1, _ := NewLogger(dir)
	logger1.Log(Entry{Task: "task-1", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})

	// Second logger session (simulates restart)
	logger2, _ := NewLogger(dir)
	logger2.Log(Entry{Task: "task-2", RiskLevel: "LOW", Outcome: "success", Duration: time.Second, Model: "test"})

	// Chain should be valid across sessions
	valid, _, err := logger2.Verify()
	require.NoError(t, err)
	assert.True(t, valid)
}
```

Add `"fmt"` to test imports.

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/audit/ -run "TestHashChain|TestVerify" -v`
Expected: FAIL — PrevHash/Hash fields undefined

**Step 3: Implement hash chain in logger.go**

Add to `Record` struct:
```go
PrevHash string `json:"prev_hash,omitempty"`
Hash     string `json:"hash,omitempty"`
```

Add `lastHash string` field to `Logger` struct. In `NewLogger()`, read last line of today's file to initialize `lastHash`.

Update `Log()`:
1. Set `record.PrevHash = l.lastHash`
2. Compute `record.Hash = computeHash(record)` (SHA-256 of JSON without hash field)
3. Update `l.lastHash = record.Hash`

Add `computeHash(r Record) string`:
```go
func computeHash(r Record) string {
	saved := r.Hash
	r.Hash = ""
	data, _ := json.Marshal(r)
	r.Hash = saved
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
```

Add `Verify() (bool, int, error)`:
- Read all JSONL files in date order
- For each record, verify `computeHash(r) == r.Hash` and `r.PrevHash == prevHash`
- Return `(true, -1, nil)` if valid, `(false, index, nil)` if broken

Add `initLastHash()` helper called from `NewLogger()`:
- Read today's JSONL file, parse last line, set `l.lastHash`

**Step 4: Run tests**

Run: `go test ./internal/audit/ -v`
Expected: ALL PASS (old + new tests)

**Step 5: Commit**

```bash
git add internal/audit/logger.go internal/audit/logger_test.go
git commit -m "feat(audit): add SHA-256 hash chain for tamper-evident audit logs

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: Run Manifest Package

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/manifest/manifest.go`
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/internal/manifest/manifest_test.go`

**Step 1: Write failing tests**

```go
package manifest

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	m := &Manifest{
		RunID:     "test-run-001",
		Task:      "build something",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Model:     "claude-opus-4-6",
		Effort:    "high",
		RiskLevel: "LOW",
		NodeCount: 2,
		DurationMs: 5000,
		Outcome:   "success",
		Nodes: []NodeResult{
			{ID: "step-1", Task: "do A", Status: "completed"},
			{ID: "step-2", Task: "do B", Status: "completed"},
		},
	}

	err := store.Save(m)
	require.NoError(t, err)

	// Verify file exists
	assert.FileExists(t, filepath.Join(dir, "test-run-001", "manifest.json"))

	// Load it back
	loaded, err := store.Load("test-run-001")
	require.NoError(t, err)
	assert.Equal(t, m.RunID, loaded.RunID)
	assert.Equal(t, m.Task, loaded.Task)
	assert.Equal(t, m.Outcome, loaded.Outcome)
	assert.Len(t, loaded.Nodes, 2)
}

func TestLoadNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_, err := store.Load("nonexistent")
	assert.Error(t, err)
}

func TestRecent(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Save 5 manifests with different timestamps
	for i := 0; i < 5; i++ {
		ts := time.Now().Add(time.Duration(i) * time.Second).UTC().Format(time.RFC3339)
		store.Save(&Manifest{
			RunID:     fmt.Sprintf("run-%d", i),
			Task:      fmt.Sprintf("task %d", i),
			Timestamp: ts,
			Outcome:   "success",
		})
	}

	recent, err := store.Recent(3)
	require.NoError(t, err)
	assert.Len(t, recent, 3)
	// Most recent first
	assert.Equal(t, "run-4", recent[0].RunID)
	assert.Equal(t, "run-3", recent[1].RunID)
	assert.Equal(t, "run-2", recent[2].RunID)
}

func TestRecentEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	recent, err := store.Recent(5)
	require.NoError(t, err)
	assert.Empty(t, recent)
}
```

Add `"fmt"` to test imports.

**Step 2: Run tests to verify they fail**

**Step 3: Implement manifest.go**

```go
package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

type NodeResult struct {
	ID     string `json:"id"`
	Task   string `json:"task"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type Manifest struct {
	RunID      string       `json:"run_id"`
	Task       string       `json:"task"`
	Timestamp  string       `json:"timestamp"`
	Model      string       `json:"model"`
	Effort     string       `json:"effort"`
	RiskLevel  string       `json:"risk_level"`
	NodeCount  int          `json:"node_count"`
	DurationMs int64        `json:"duration_ms"`
	Outcome    string       `json:"outcome"`
	Nodes      []NodeResult `json:"nodes"`
}

type Store struct {
	dir string
}

func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

func (s *Store) Save(m *Manifest) error {
	runDir := filepath.Join(s.dir, m.RunID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(runDir, "manifest.json"), data, 0644)
}

func (s *Store) Load(runID string) (*Manifest, error) {
	path := filepath.Join(s.dir, runID, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) Recent(n int) ([]*Manifest, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var manifests []*Manifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		m, err := s.Load(entry.Name())
		if err != nil {
			continue
		}
		manifests = append(manifests, m)
	}

	// Sort by timestamp descending
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].Timestamp > manifests[j].Timestamp
	})

	if len(manifests) > n {
		manifests = manifests[:n]
	}
	return manifests, nil
}
```

**Step 4: Run tests**

Run: `go test ./internal/manifest/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/manifest/manifest.go internal/manifest/manifest_test.go
git commit -m "feat(manifest): add run manifest storage for execution metadata

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: `apex status` Command

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/status.go`
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/main.go` — add `rootCmd.AddCommand(statusCmd)`

**Step 1: Implement status.go**

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/manifest"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show recent run history",
	RunE:  showStatus,
}

var statusLast int

func init() {
	statusCmd.Flags().IntVar(&statusLast, "last", 5, "Number of recent runs to show")
}

func showStatus(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	runsDir := filepath.Join(home, ".apex", "runs")
	store := manifest.NewStore(runsDir)

	recent, err := store.Recent(statusLast)
	if err != nil {
		return fmt.Errorf("failed to read runs: %w", err)
	}

	if len(recent) == 0 {
		fmt.Println("No runs found.")
		return nil
	}

	// Header
	fmt.Printf("%-10s %-40s %-10s %-10s %-6s %s\n",
		"RUN_ID", "TASK", "OUTCOME", "DURATION", "NODES", "TIMESTAMP")
	fmt.Println("---------- ---------------------------------------- ---------- ---------- ------ --------------------")

	for _, m := range recent {
		runID := m.RunID
		if len(runID) > 8 {
			runID = runID[:8]
		}
		task := m.Task
		if len(task) > 40 {
			task = task[:37] + "..."
		}
		duration := fmt.Sprintf("%.1fs", float64(m.DurationMs)/1000)
		fmt.Printf("%-10s %-40s %-10s %-10s %-6d %s\n",
			runID, task, m.Outcome, duration, m.NodeCount, m.Timestamp)
	}

	return nil
}
```

**Step 2: Register in main.go**

Add `rootCmd.AddCommand(statusCmd)` in init().

**Step 3: Build and test**

```bash
go build -o bin/apex ./cmd/apex/
./bin/apex status --help
```

**Step 4: Commit**

```bash
git add cmd/apex/status.go cmd/apex/main.go
git commit -m "feat: add apex status command for run history overview

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: `apex doctor` Command

**Files:**
- Create: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/doctor.go`
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/main.go` — add `rootCmd.AddCommand(doctorCmd)`

**Step 1: Implement doctor.go**

```go
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

	// Check audit chain
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
```

**Step 2: Register in main.go**

Add `rootCmd.AddCommand(doctorCmd)` in init().

**Step 3: Build and test**

```bash
go build -o bin/apex ./cmd/apex/
./bin/apex doctor --help
```

**Step 4: Commit**

```bash
git add cmd/apex/doctor.go cmd/apex/main.go
git commit -m "feat: add apex doctor command for integrity verification

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 5: Wire Manifest into Run

**Files:**
- Modify: `/Users/lyndonlyu/Downloads/ai_agent_cli_project/cmd/apex/run.go`

**Step 1: Integrate manifest write after execution**

Add import:
```go
"github.com/lyndonlyu/apex/internal/manifest"
"github.com/google/uuid"
```

After audit logging and before "Print results", add:

```go
// Save run manifest
runsDir := filepath.Join(cfg.BaseDir, "runs")
manifestStore := manifest.NewStore(runsDir)

outcome := "success"
if d.HasFailure() {
	if execErr != nil {
		outcome = "failure"
	} else {
		outcome = "partial_failure"
	}
}

var nodeResults []manifest.NodeResult
for _, n := range d.Nodes {
	nr := manifest.NodeResult{
		ID:     n.ID,
		Task:   n.Task,
		Status: n.Status.String(),
	}
	if n.Status == dag.Failed {
		nr.Error = n.Error
	}
	nodeResults = append(nodeResults, nr)
}

runManifest := &manifest.Manifest{
	RunID:      uuid.New().String(),
	Task:       task,
	Timestamp:  time.Now().UTC().Format(time.RFC3339),
	Model:      cfg.Claude.Model,
	Effort:     cfg.Claude.Effort,
	RiskLevel:  risk.String(),
	NodeCount:  len(d.Nodes),
	DurationMs: duration.Milliseconds(),
	Outcome:    outcome,
	Nodes:      nodeResults,
}

if saveErr := manifestStore.Save(runManifest); saveErr != nil {
	fmt.Fprintf(os.Stderr, "warning: manifest save failed: %v\n", saveErr)
}
```

**Step 2: Run all tests**

Run: `go test ./...`
Expected: ALL 14 packages PASS (13 existing + manifest)

**Step 3: Build**

Run: `go build -o bin/apex ./cmd/apex/`

**Step 4: Commit**

```bash
git add cmd/apex/run.go
git commit -m "feat: write run manifest after DAG execution

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 6: E2E Verification

**Step 1: Run all tests**

```bash
go test ./... -v 2>&1
```

Expected: All 14 packages pass

**Step 2: Build binary**

```bash
go build -o bin/apex ./cmd/apex/
```

**Step 3: Verify CLI**

```bash
./bin/apex --help
./bin/apex status --help
./bin/apex doctor --help
./bin/apex doctor
```

Expected: All commands work, doctor reports audit chain status

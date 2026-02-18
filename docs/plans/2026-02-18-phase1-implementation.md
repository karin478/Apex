# Phase 1 MVP Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the Apex Agent CLI that takes a user task, classifies risk, executes via Claude Code CLI, persists memory, and logs audit trail.

**Architecture:** Single Go module with `internal/` packages for governance, executor, memory, audit, and config. CLI built with cobra. All state stored in `~/.apex/`. Claude Code CLI invoked via `claude -p` with `--model claude-opus-4-6 --effort high`.

**Tech Stack:** Go 1.25, cobra (CLI), testify (assertions), YAML config, JSONL audit, Markdown memory files.

---

### Task 1: Project Scaffold

**Files:**
- Create: `go.mod`
- Create: `cmd/apex/main.go`
- Create: `Makefile`

**Step 1: Initialize Go module**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
go mod init github.com/lyndonlyu/apex
```
Expected: `go.mod` created

**Step 2: Install dependencies**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
go get github.com/spf13/cobra@latest
go get github.com/stretchr/testify@latest
go get gopkg.in/yaml.v3@latest
go get github.com/google/uuid@latest
```
Expected: `go.sum` created, packages downloaded

**Step 3: Create minimal main.go**

Create `cmd/apex/main.go`:
```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "apex",
	Short: "Apex Agent - Claude Code autonomous agent system",
	Long:  "Apex Agent is a CLI tool that orchestrates Claude Code for long-term memory autonomous agent tasks.",
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("apex v0.1.0")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
```

**Step 4: Create Makefile**

Create `Makefile`:
```makefile
.PHONY: build test clean install

BINARY=apex
BUILD_DIR=bin

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/apex/

test:
	go test ./... -v -count=1

test-coverage:
	go test ./... -v -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out

clean:
	rm -rf $(BUILD_DIR) coverage.out

install: build
	cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)
```

**Step 5: Build and verify**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
mkdir -p bin
make build
./bin/apex version
```
Expected: `apex v0.1.0`

**Step 6: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add go.mod go.sum cmd/ Makefile
git commit -m "feat: project scaffold with cobra CLI and version command"
```

---

### Task 2: Config Package

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write the failing tests**

Create `internal/config/config_test.go`:
```go
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()

	assert.Equal(t, "claude-opus-4-6", cfg.Claude.Model)
	assert.Equal(t, "high", cfg.Claude.Effort)
	assert.Equal(t, 600, cfg.Claude.Timeout)
	assert.Equal(t, 1800, cfg.Claude.LongTaskTimeout)
	assert.Contains(t, cfg.Governance.AutoApprove, "LOW")
	assert.Contains(t, cfg.Governance.Confirm, "MEDIUM")
	assert.Contains(t, cfg.Governance.Reject, "HIGH")
	assert.Contains(t, cfg.Governance.Reject, "CRITICAL")
}

func TestLoadConfigFromFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	content := []byte(`claude:
  model: claude-sonnet-4-6
  timeout: 900
`)
	require.NoError(t, os.WriteFile(configPath, content, 0644))

	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Equal(t, "claude-sonnet-4-6", cfg.Claude.Model)
	assert.Equal(t, 900, cfg.Claude.Timeout)
	// Defaults preserved for unset fields
	assert.Equal(t, "high", cfg.Claude.Effort)
	assert.Equal(t, 1800, cfg.Claude.LongTaskTimeout)
}

func TestLoadConfigFileNotFound(t *testing.T) {
	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err, "missing config file should return defaults, not error")
	assert.Equal(t, "claude-opus-4-6", cfg.Claude.Model)
}

func TestApexDir(t *testing.T) {
	cfg := Default()
	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, ".apex"), cfg.ApexDir())
}

func TestEnsureDirs(t *testing.T) {
	dir := t.TempDir()
	cfg := Default()
	cfg.BaseDir = dir

	require.NoError(t, cfg.EnsureDirs())

	assert.DirExists(t, filepath.Join(dir, "memory", "decisions"))
	assert.DirExists(t, filepath.Join(dir, "memory", "facts"))
	assert.DirExists(t, filepath.Join(dir, "memory", "sessions"))
	assert.DirExists(t, filepath.Join(dir, "audit"))
}
```

**Step 2: Run tests to verify they fail**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
go test ./internal/config/ -v
```
Expected: FAIL (package doesn't exist yet)

**Step 3: Write implementation**

Create `internal/config/config.go`:
```go
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ClaudeConfig struct {
	Model           string `yaml:"model"`
	Effort          string `yaml:"effort"`
	Timeout         int    `yaml:"timeout"`
	LongTaskTimeout int    `yaml:"long_task_timeout"`
}

type GovernanceConfig struct {
	AutoApprove []string `yaml:"auto_approve"`
	Confirm     []string `yaml:"confirm"`
	Reject      []string `yaml:"reject"`
}

type Config struct {
	Claude     ClaudeConfig     `yaml:"claude"`
	Governance GovernanceConfig `yaml:"governance"`
	BaseDir    string           `yaml:"-"`
}

func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Claude: ClaudeConfig{
			Model:           "claude-opus-4-6",
			Effort:          "high",
			Timeout:         600,
			LongTaskTimeout: 1800,
		},
		Governance: GovernanceConfig{
			AutoApprove: []string{"LOW"},
			Confirm:     []string{"MEDIUM"},
			Reject:      []string{"HIGH", "CRITICAL"},
		},
		BaseDir: filepath.Join(home, ".apex"),
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Ensure defaults for zero values
	if cfg.Claude.Model == "" {
		cfg.Claude.Model = "claude-opus-4-6"
	}
	if cfg.Claude.Effort == "" {
		cfg.Claude.Effort = "high"
	}
	if cfg.Claude.Timeout == 0 {
		cfg.Claude.Timeout = 600
	}
	if cfg.Claude.LongTaskTimeout == 0 {
		cfg.Claude.LongTaskTimeout = 1800
	}
	if cfg.BaseDir == "" {
		home, _ := os.UserHomeDir()
		cfg.BaseDir = filepath.Join(home, ".apex")
	}

	return cfg, nil
}

func (c *Config) ApexDir() string {
	return c.BaseDir
}

func (c *Config) EnsureDirs() error {
	dirs := []string{
		filepath.Join(c.BaseDir, "memory", "decisions"),
		filepath.Join(c.BaseDir, "memory", "facts"),
		filepath.Join(c.BaseDir, "memory", "sessions"),
		filepath.Join(c.BaseDir, "audit"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return nil
}
```

**Step 4: Run tests to verify they pass**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
go test ./internal/config/ -v
```
Expected: ALL PASS

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/config/
git commit -m "feat(config): add config package with YAML loading and defaults"
```

---

### Task 3: Governance Package

**Files:**
- Create: `internal/governance/risk.go`
- Create: `internal/governance/risk_test.go`

**Step 1: Write the failing tests**

Create `internal/governance/risk_test.go`:
```go
package governance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyLow(t *testing.T) {
	tests := []string{
		"read the README file",
		"explain this function",
		"search for TODO comments",
		"list all go files",
		"run the tests",
	}
	for _, task := range tests {
		assert.Equal(t, LOW, Classify(task), "task: %s", task)
	}
}

func TestClassifyMedium(t *testing.T) {
	tests := []string{
		"write a new function for parsing",
		"modify the config file",
		"install the cobra package",
		"update the README",
		"create a new test file",
		"修改配置文件",
		"安装依赖",
	}
	for _, task := range tests {
		assert.Equal(t, MEDIUM, Classify(task), "task: %s", task)
	}
}

func TestClassifyHigh(t *testing.T) {
	tests := []string{
		"delete the old migration files",
		"deploy to staging",
		"drop the users table",
		"migrate the database schema",
		"rm -rf the temp directory",
		"删除旧数据",
		"部署到生产",
	}
	for _, task := range tests {
		assert.Equal(t, HIGH, Classify(task), "task: %s", task)
	}
}

func TestClassifyCritical(t *testing.T) {
	tests := []string{
		"deploy to production with new encryption keys",
		"rotate the production API密钥",
	}
	for _, task := range tests {
		assert.Equal(t, CRITICAL, Classify(task), "task: %s", task)
	}
}

func TestClassifyCaseInsensitive(t *testing.T) {
	assert.Equal(t, HIGH, Classify("DELETE all temp files"))
	assert.Equal(t, MEDIUM, Classify("WRITE a new module"))
}

func TestRiskLevelString(t *testing.T) {
	assert.Equal(t, "LOW", LOW.String())
	assert.Equal(t, "MEDIUM", MEDIUM.String())
	assert.Equal(t, "HIGH", HIGH.String())
	assert.Equal(t, "CRITICAL", CRITICAL.String())
}

func TestShouldAutoApprove(t *testing.T) {
	assert.True(t, LOW.ShouldAutoApprove())
	assert.False(t, MEDIUM.ShouldAutoApprove())
	assert.False(t, HIGH.ShouldAutoApprove())
	assert.False(t, CRITICAL.ShouldAutoApprove())
}

func TestShouldConfirm(t *testing.T) {
	assert.False(t, LOW.ShouldConfirm())
	assert.True(t, MEDIUM.ShouldConfirm())
	assert.False(t, HIGH.ShouldConfirm())
	assert.False(t, CRITICAL.ShouldConfirm())
}

func TestShouldReject(t *testing.T) {
	assert.False(t, LOW.ShouldReject())
	assert.False(t, MEDIUM.ShouldReject())
	assert.True(t, HIGH.ShouldReject())
	assert.True(t, CRITICAL.ShouldReject())
}
```

**Step 2: Run tests to verify they fail**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
go test ./internal/governance/ -v
```
Expected: FAIL

**Step 3: Write implementation**

Create `internal/governance/risk.go`:
```go
package governance

import "strings"

type RiskLevel int

const (
	LOW RiskLevel = iota
	MEDIUM
	HIGH
	CRITICAL
)

func (r RiskLevel) String() string {
	switch r {
	case LOW:
		return "LOW"
	case MEDIUM:
		return "MEDIUM"
	case HIGH:
		return "HIGH"
	case CRITICAL:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

func (r RiskLevel) ShouldAutoApprove() bool {
	return r == LOW
}

func (r RiskLevel) ShouldConfirm() bool {
	return r == MEDIUM
}

func (r RiskLevel) ShouldReject() bool {
	return r >= HIGH
}

var criticalKeywords = []string{
	"production", "encryption key", "密钥",
}

var highKeywords = []string{
	"delete", "drop", "deploy", "migrate", "rm -rf",
	"删除", "部署", "生产", "迁移",
}

var mediumKeywords = []string{
	"write", "modify", "install", "update", "config",
	"create", "edit", "change", "add", "remove",
	"修改", "安装", "配置", "创建", "编辑",
}

func Classify(task string) RiskLevel {
	lower := strings.ToLower(task)

	for _, kw := range criticalKeywords {
		if strings.Contains(lower, kw) {
			return CRITICAL
		}
	}

	for _, kw := range highKeywords {
		if strings.Contains(lower, kw) {
			return HIGH
		}
	}

	for _, kw := range mediumKeywords {
		if strings.Contains(lower, kw) {
			return MEDIUM
		}
	}

	return LOW
}
```

**Step 4: Run tests to verify they pass**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
go test ./internal/governance/ -v
```
Expected: ALL PASS

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/governance/
git commit -m "feat(governance): add keyword-based risk classification"
```

---

### Task 4: Audit Package

**Files:**
- Create: `internal/audit/logger.go`
- Create: `internal/audit/logger_test.go`

**Step 1: Write the failing tests**

Create `internal/audit/logger_test.go`:
```go
package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)
	assert.NotNil(t, logger)
	assert.DirExists(t, dir)
}

func TestLogEntry(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	entry := Entry{
		Task:      "test task",
		RiskLevel: "LOW",
		Outcome:   "success",
		Duration:  100 * time.Millisecond,
		Model:     "claude-opus-4-6",
	}

	err = logger.Log(entry)
	require.NoError(t, err)

	// Verify file was created with today's date
	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(dir, today+".jsonl")
	assert.FileExists(t, logFile)

	// Read and parse the entry
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	var record Record
	require.NoError(t, json.Unmarshal(data, &record))

	assert.Equal(t, "test task", record.Task)
	assert.Equal(t, "LOW", record.RiskLevel)
	assert.Equal(t, "success", record.Outcome)
	assert.NotEmpty(t, record.ActionID)
	assert.NotEmpty(t, record.Timestamp)
	assert.Equal(t, "claude-opus-4-6", record.Model)
	assert.Equal(t, int64(100), record.DurationMs)
}

func TestLogMultipleEntries(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		err = logger.Log(Entry{
			Task:      "task",
			RiskLevel: "LOW",
			Outcome:   "success",
			Duration:  time.Second,
			Model:     "claude-opus-4-6",
		})
		require.NoError(t, err)
	}

	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(dir, today+".jsonl")
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 3)
}

func TestRecentEntries(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		err = logger.Log(Entry{
			Task:      "task",
			RiskLevel: "LOW",
			Outcome:   "success",
			Duration:  time.Second,
			Model:     "claude-opus-4-6",
		})
		require.NoError(t, err)
	}

	entries, err := logger.Recent(3)
	require.NoError(t, err)
	assert.Len(t, entries, 3)
}
```

**Step 2: Run tests to verify they fail**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
go test ./internal/audit/ -v
```
Expected: FAIL

**Step 3: Write implementation**

Create `internal/audit/logger.go`:
```go
package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Entry struct {
	Task      string
	RiskLevel string
	Outcome   string
	Duration  time.Duration
	Model     string
	Error     string
}

type Record struct {
	Timestamp  string `json:"timestamp"`
	ActionID   string `json:"action_id"`
	Task       string `json:"task"`
	RiskLevel  string `json:"risk_level"`
	Outcome    string `json:"outcome"`
	DurationMs int64  `json:"duration_ms"`
	Model      string `json:"model"`
	Error      string `json:"error,omitempty"`
}

type Logger struct {
	dir string
}

func NewLogger(dir string) (*Logger, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &Logger{dir: dir}, nil
}

func (l *Logger) Log(entry Entry) error {
	record := Record{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		ActionID:   uuid.New().String(),
		Task:       entry.Task,
		RiskLevel:  entry.RiskLevel,
		Outcome:    entry.Outcome,
		DurationMs: entry.Duration.Milliseconds(),
		Model:      entry.Model,
		Error:      entry.Error,
	}

	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	filename := time.Now().Format("2006-01-02") + ".jsonl"
	path := filepath.Join(l.dir, filename)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

func (l *Logger) Recent(n int) ([]Record, error) {
	files, err := filepath.Glob(filepath.Join(l.dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))

	var records []Record
	for _, f := range files {
		if len(records) >= n {
			break
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			if len(records) >= n {
				break
			}
			var r Record
			if err := json.Unmarshal([]byte(lines[i]), &r); err != nil {
				continue
			}
			records = append(records, r)
		}
	}
	return records, nil
}
```

**Step 4: Run tests to verify they pass**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
go test ./internal/audit/ -v
```
Expected: ALL PASS

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/audit/
git commit -m "feat(audit): add JSONL audit logger with append-only writes"
```

---

### Task 5: Memory Package

**Files:**
- Create: `internal/memory/store.go`
- Create: `internal/memory/store_test.go`

**Step 1: Write the failing tests**

Create `internal/memory/store_test.go`:
```go
package memory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)
	assert.NotNil(t, store)
}

func TestSaveDecision(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)

	err = store.SaveDecision("auth-refactor", "Chose JWT over session-based auth for stateless scaling.")
	require.NoError(t, err)

	// Verify file exists in decisions/
	files, _ := filepath.Glob(filepath.Join(dir, "decisions", "*-auth-refactor.md"))
	assert.Len(t, files, 1)

	content, _ := os.ReadFile(files[0])
	assert.Contains(t, string(content), "auth-refactor")
	assert.Contains(t, string(content), "JWT over session-based")
}

func TestSaveFact(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)

	err = store.SaveFact("go-version", "Project uses Go 1.25")
	require.NoError(t, err)

	files, _ := filepath.Glob(filepath.Join(dir, "facts", "*-go-version.md"))
	assert.Len(t, files, 1)
}

func TestSaveSession(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)

	err = store.SaveSession("sess-001", "refactor auth", "Done. Used JWT.")
	require.NoError(t, err)

	path := filepath.Join(dir, "sessions", "sess-001.jsonl")
	assert.FileExists(t, path)

	content, _ := os.ReadFile(path)
	assert.Contains(t, string(content), "refactor auth")
}

func TestSearch(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)

	store.SaveDecision("redis-migration", "Migrated from Redis 6 to Redis 7 for ACL support.")
	store.SaveFact("redis-version", "Production runs Redis 7.2")
	store.SaveDecision("db-choice", "Chose PostgreSQL for main database.")

	results, err := store.Search("redis")
	require.NoError(t, err)
	assert.Len(t, results, 2, "should find 2 files mentioning redis")
}

func TestSearchNoResults(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	require.NoError(t, err)

	results, err := store.Search("nonexistent-term-xyz")
	require.NoError(t, err)
	assert.Empty(t, results)
}
```

**Step 2: Run tests to verify they fail**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
go test ./internal/memory/ -v
```
Expected: FAIL

**Step 3: Write implementation**

Create `internal/memory/store.go`:
```go
package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Store struct {
	dir string
}

type SearchResult struct {
	Path    string
	Type    string
	Snippet string
}

type sessionRecord struct {
	Timestamp string `json:"timestamp"`
	Task      string `json:"task"`
	Result    string `json:"result"`
}

func NewStore(dir string) (*Store, error) {
	for _, sub := range []string{"decisions", "facts", "sessions"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			return nil, err
		}
	}
	return &Store{dir: dir}, nil
}

func (s *Store) SaveDecision(slug string, content string) error {
	return s.saveMarkdown("decisions", slug, "decision", content)
}

func (s *Store) SaveFact(slug string, content string) error {
	return s.saveMarkdown("facts", slug, "fact", content)
}

func (s *Store) saveMarkdown(subdir, slug, memType, content string) error {
	ts := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.md", ts, slug)
	path := filepath.Join(s.dir, subdir, filename)

	md := fmt.Sprintf(`---
type: %s
created: %s
slug: %s
---

# %s

%s
`, memType, time.Now().UTC().Format(time.RFC3339), slug, slug, content)

	return os.WriteFile(path, []byte(md), 0644)
}

func (s *Store) SaveSession(sessionID, task, result string) error {
	path := filepath.Join(s.dir, "sessions", sessionID+".jsonl")

	record := sessionRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Task:      task,
		Result:    result,
	}

	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

func (s *Store) Search(keyword string) ([]SearchResult, error) {
	var results []SearchResult
	lower := strings.ToLower(keyword)

	err := filepath.Walk(s.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") && !strings.HasSuffix(path, ".jsonl") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		content := strings.ToLower(string(data))
		if strings.Contains(content, lower) {
			// Determine type from parent dir
			rel, _ := filepath.Rel(s.dir, path)
			parts := strings.SplitN(rel, string(filepath.Separator), 2)
			memType := "unknown"
			if len(parts) > 0 {
				memType = parts[0]
			}

			// Extract snippet (first line containing keyword, up to 120 chars)
			snippet := ""
			for _, line := range strings.Split(string(data), "\n") {
				if strings.Contains(strings.ToLower(line), lower) {
					snippet = line
					if len(snippet) > 120 {
						snippet = snippet[:120] + "..."
					}
					break
				}
			}

			results = append(results, SearchResult{
				Path:    path,
				Type:    memType,
				Snippet: snippet,
			})
		}
		return nil
	})

	return results, err
}
```

**Step 4: Run tests to verify they pass**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
go test ./internal/memory/ -v
```
Expected: ALL PASS

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/memory/
git commit -m "feat(memory): add filesystem-based memory store with search"
```

---

### Task 6: Executor Package

**Files:**
- Create: `internal/executor/claude.go`
- Create: `internal/executor/claude_test.go`

**Step 1: Write the failing tests**

Create `internal/executor/claude_test.go`:
```go
package executor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExecutor(t *testing.T) {
	exec := New(Options{
		Model:   "claude-opus-4-6",
		Effort:  "high",
		Timeout: 600 * time.Second,
	})
	assert.NotNil(t, exec)
}

func TestBuildArgs(t *testing.T) {
	exec := New(Options{
		Model:   "claude-opus-4-6",
		Effort:  "high",
		Timeout: 600 * time.Second,
	})

	args := exec.buildArgs("explain this code")
	assert.Contains(t, args, "-p")
	assert.Contains(t, args, "--model")
	assert.Contains(t, args, "claude-opus-4-6")
	assert.Contains(t, args, "--effort")
	assert.Contains(t, args, "high")
	assert.Contains(t, args, "--output-format")
	assert.Contains(t, args, "json")
}

func TestBuildArgsContainsPrompt(t *testing.T) {
	exec := New(Options{
		Model:   "claude-opus-4-6",
		Effort:  "high",
		Timeout: 600 * time.Second,
	})

	args := exec.buildArgs("do something")
	// Last arg should be the prompt
	assert.Equal(t, "do something", args[len(args)-1])
}

func TestExecuteWithMockBinary(t *testing.T) {
	// Use echo as a mock for claude CLI
	exec := New(Options{
		Model:   "claude-opus-4-6",
		Effort:  "high",
		Timeout: 10 * time.Second,
		Binary:  "echo",
	})

	result, err := exec.Run(context.Background(), "hello world")
	require.NoError(t, err)
	assert.NotEmpty(t, result.Output)
	assert.Equal(t, 0, result.ExitCode)
}

func TestExecuteTimeout(t *testing.T) {
	exec := New(Options{
		Model:   "claude-opus-4-6",
		Effort:  "high",
		Timeout: 100 * time.Millisecond,
		Binary:  "sleep",
	})

	result, err := exec.Run(context.Background(), "10")
	assert.Error(t, err)
	assert.True(t, result.TimedOut)
}

func TestResultDuration(t *testing.T) {
	exec := New(Options{
		Model:   "claude-opus-4-6",
		Effort:  "high",
		Timeout: 10 * time.Second,
		Binary:  "echo",
	})

	result, err := exec.Run(context.Background(), "fast task")
	require.NoError(t, err)
	assert.True(t, result.Duration > 0)
	assert.True(t, result.Duration < 5*time.Second)
}
```

**Step 2: Run tests to verify they fail**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
go test ./internal/executor/ -v
```
Expected: FAIL

**Step 3: Write implementation**

Create `internal/executor/claude.go`:
```go
package executor

import (
	"bytes"
	"context"
	"os/exec"
	"time"
)

type Options struct {
	Model   string
	Effort  string
	Timeout time.Duration
	Binary  string // defaults to "claude"
}

type Result struct {
	Output   string
	Stderr   string
	ExitCode int
	Duration time.Duration
	TimedOut bool
}

type Executor struct {
	opts Options
}

func New(opts Options) *Executor {
	if opts.Binary == "" {
		opts.Binary = "claude"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 600 * time.Second
	}
	return &Executor{opts: opts}
}

func (e *Executor) buildArgs(task string) []string {
	args := []string{
		"-p",
		"--model", e.opts.Model,
		"--effort", e.opts.Effort,
		"--output-format", "json",
		task,
	}
	return args
}

func (e *Executor) Run(ctx context.Context, task string) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, e.opts.Timeout)
	defer cancel()

	args := e.buildArgs(task)
	cmd := exec.CommandContext(ctx, e.opts.Binary, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := Result{
		Output:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		return result, ctx.Err()
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		return result, err
	}

	return result, nil
}
```

**Step 4: Run tests to verify they pass**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
go test ./internal/executor/ -v
```
Expected: ALL PASS

**Step 5: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add internal/executor/
git commit -m "feat(executor): add Claude CLI executor with timeout support"
```

---

### Task 7: Wire Everything — `apex run` Command

**Files:**
- Modify: `cmd/apex/main.go`
- Create: `cmd/apex/run.go`
- Create: `cmd/apex/history.go`
- Create: `cmd/apex/memory.go`

**Step 1: Write integration test**

Create `cmd/apex/run_test.go`:
```go
package main

import (
	"testing"

	"github.com/lyndonlyu/apex/internal/governance"
	"github.com/stretchr/testify/assert"
)

func TestRunCommandRiskGating(t *testing.T) {
	// Verify governance integration
	assert.True(t, governance.Classify("read the README").ShouldAutoApprove())
	assert.True(t, governance.Classify("modify the config").ShouldConfirm())
	assert.True(t, governance.Classify("delete the database").ShouldReject())
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
go test ./cmd/apex/ -v
```
Expected: FAIL

**Step 3: Create `cmd/apex/run.go`**

```go
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lyndonlyu/apex/internal/audit"
	"github.com/lyndonlyu/apex/internal/config"
	"github.com/lyndonlyu/apex/internal/executor"
	"github.com/lyndonlyu/apex/internal/governance"
	"github.com/lyndonlyu/apex/internal/memory"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [task]",
	Short: "Execute a task via Claude Code",
	Long:  "Classify risk, get approval if needed, then execute the task via Claude Code CLI.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runTask,
}

func runTask(cmd *cobra.Command, args []string) error {
	task := args[0]

	// Load config
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".apex", "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	// Ensure directories
	if err := cfg.EnsureDirs(); err != nil {
		return fmt.Errorf("failed to create dirs: %w", err)
	}

	// Classify risk
	risk := governance.Classify(task)
	fmt.Printf("[%s] Risk level: %s\n", task, risk)

	// Gate by risk level
	if risk.ShouldReject() {
		fmt.Printf("❌ %s risk task rejected. Break it into smaller, safer steps.\n", risk)
		return nil
	}

	if risk.ShouldConfirm() {
		fmt.Printf("⚠ %s risk detected. Proceed? (y/n): ", risk)
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Execute
	exec := executor.New(executor.Options{
		Model:   cfg.Claude.Model,
		Effort:  cfg.Claude.Effort,
		Timeout: time.Duration(cfg.Claude.Timeout) * time.Second,
	})

	fmt.Println("⏳ Executing task...")
	start := time.Now()
	result, err := exec.Run(context.Background(), task)
	duration := time.Since(start)

	// Audit
	auditDir := filepath.Join(cfg.BaseDir, "audit")
	logger, auditErr := audit.NewLogger(auditDir)
	if auditErr != nil {
		fmt.Fprintf(os.Stderr, "warning: audit init failed: %v\n", auditErr)
	}

	outcome := "success"
	errMsg := ""
	if err != nil {
		outcome = "failure"
		errMsg = err.Error()
		if result.TimedOut {
			outcome = "timeout"
		}
	}

	if logger != nil {
		logger.Log(audit.Entry{
			Task:      task,
			RiskLevel: risk.String(),
			Outcome:   outcome,
			Duration:  duration,
			Model:     cfg.Claude.Model,
			Error:     errMsg,
		})
	}

	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	// Output result
	fmt.Println("\n--- Result ---")
	fmt.Println(result.Output)

	// Save to memory
	memDir := filepath.Join(cfg.BaseDir, "memory")
	store, memErr := memory.NewStore(memDir)
	if memErr != nil {
		fmt.Fprintf(os.Stderr, "warning: memory init failed: %v\n", memErr)
	} else {
		store.SaveSession("run", task, result.Output)
	}

	fmt.Printf("\n✅ Done (%.1fs, %s risk)\n", duration.Seconds(), risk)
	return nil
}
```

**Step 4: Create `cmd/apex/history.go`**

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/audit"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show recent task history",
	RunE:  showHistory,
}

var historyCount int

func init() {
	historyCmd.Flags().IntVarP(&historyCount, "count", "n", 10, "Number of entries to show")
}

func showHistory(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	auditDir := filepath.Join(home, ".apex", "audit")

	logger, err := audit.NewLogger(auditDir)
	if err != nil {
		return fmt.Errorf("audit error: %w", err)
	}

	entries, err := logger.Recent(historyCount)
	if err != nil {
		return fmt.Errorf("failed to read history: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No history yet.")
		return nil
	}

	for _, e := range entries {
		icon := "✅"
		if e.Outcome == "failure" {
			icon = "❌"
		} else if e.Outcome == "timeout" {
			icon = "⏰"
		} else if e.Outcome == "rejected" {
			icon = "🚫"
		}
		fmt.Printf("%s %s [%s] %s (%dms)\n", icon, e.Timestamp[:19], e.RiskLevel, e.Task, e.DurationMs)
	}
	return nil
}
```

**Step 5: Create `cmd/apex/memory.go`**

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/memory"
	"github.com/spf13/cobra"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage memory",
}

var memorySearchCmd = &cobra.Command{
	Use:   "search [keyword]",
	Short: "Search memory for a keyword",
	Args:  cobra.ExactArgs(1),
	RunE:  searchMemory,
}

func init() {
	memoryCmd.AddCommand(memorySearchCmd)
}

func searchMemory(cmd *cobra.Command, args []string) error {
	keyword := args[0]
	home, _ := os.UserHomeDir()
	memDir := filepath.Join(home, ".apex", "memory")

	store, err := memory.NewStore(memDir)
	if err != nil {
		return fmt.Errorf("memory error: %w", err)
	}

	results, err := store.Search(keyword)
	if err != nil {
		return fmt.Errorf("search error: %w", err)
	}

	if len(results) == 0 {
		fmt.Printf("No memories found for '%s'\n", keyword)
		return nil
	}

	fmt.Printf("Found %d result(s) for '%s':\n\n", len(results), keyword)
	for _, r := range results {
		fmt.Printf("  [%s] %s\n", r.Type, r.Path)
		if r.Snippet != "" {
			fmt.Printf("         %s\n", r.Snippet)
		}
		fmt.Println()
	}
	return nil
}
```

**Step 6: Update `cmd/apex/main.go` to register all commands**

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "apex",
	Short: "Apex Agent - Claude Code autonomous agent system",
	Long:  "Apex Agent is a CLI tool that orchestrates Claude Code for long-term memory autonomous agent tasks.",
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("apex v0.1.0")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(memoryCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
```

**Step 7: Run all tests**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
go test ./... -v
```
Expected: ALL PASS

**Step 8: Build and verify CLI**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
make build
./bin/apex version
./bin/apex --help
./bin/apex run --help
./bin/apex memory search --help
./bin/apex history --help
```
Expected: All commands show help text

**Step 9: Commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add cmd/apex/
git commit -m "feat: wire all components into apex CLI (run, history, memory search)"
```

---

### Task 8: End-to-End Smoke Test

**Step 1: Build and install**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
make build
```

**Step 2: Test the full loop with a LOW risk task**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
./bin/apex run "explain what Go interfaces are"
```
Expected: Risk classified as LOW, auto-approved, Claude output displayed

**Step 3: Check audit log**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
./bin/apex history
```
Expected: Shows the previous run entry

**Step 4: Check memory**

Run:
```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
./bin/apex memory search "interface"
```
Expected: Finds the session record

**Step 5: Final commit**

```bash
cd /Users/lyndonlyu/Downloads/ai_agent_cli_project
git add -A
git commit -m "feat: Phase 1 MVP complete - core loop functional"
```

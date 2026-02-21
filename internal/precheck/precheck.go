package precheck

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// Check is the interface for environment validation checks.
type Check interface {
	Name() string
	Run() CheckResult
}

// CheckResult holds the outcome of a single check.
type CheckResult struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// RunResult holds the aggregate outcome of all checks.
type RunResult struct {
	AllPassed bool          `json:"all_passed"`
	Results   []CheckResult `json:"results"`
	Duration  string        `json:"duration"`
}

// Runner manages and executes a collection of checks.
type Runner struct {
	mu     sync.RWMutex
	checks []Check
}

// NewRunner creates an empty runner.
func NewRunner() *Runner {
	return &Runner{}
}

// Add appends a check to the runner (thread-safe).
func (r *Runner) Add(c Check) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checks = append(r.checks, c)
}

// Run executes all checks sequentially, times execution, and returns RunResult.
func (r *Runner) Run() RunResult {
	r.mu.RLock()
	checks := make([]Check, len(r.checks))
	copy(checks, r.checks)
	r.mu.RUnlock()

	start := time.Now()
	var results []CheckResult
	allPassed := true
	for _, c := range checks {
		result := c.Run()
		results = append(results, result)
		if !result.Passed {
			allPassed = false
		}
	}
	return RunResult{
		AllPassed: allPassed,
		Results:   results,
		Duration:  time.Since(start).String(),
	}
}

// Checks returns the names of all registered checks.
func (r *Runner) Checks() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, len(r.checks))
	for i, c := range r.checks {
		names[i] = c.Name()
	}
	return names
}

// DefaultRunner creates a runner with standard checks for the given home directory.
func DefaultRunner(home string) *Runner {
	r := NewRunner()
	r.Add(DirCheck{Dir: filepath.Join(home, ".apex")})
	r.Add(DirCheck{Dir: filepath.Join(home, ".apex", "audit")})
	r.Add(DirCheck{Dir: filepath.Join(home, ".apex", "runs")})
	r.Add(DirCheck{Dir: filepath.Join(home, ".apex", "memory")})
	r.Add(FileCheck{Path: filepath.Join(home, ".apex", "config.yaml"), Desc: "config"})
	return r
}

// ---------- Built-in checks ----------

// DirCheck validates that a directory exists.
type DirCheck struct {
	Dir string
}

func (c DirCheck) Name() string { return "dir:" + c.Dir }
func (c DirCheck) Run() CheckResult {
	info, err := os.Stat(c.Dir)
	if err != nil {
		return CheckResult{Name: c.Name(), Passed: false, Message: fmt.Sprintf("directory not found: %s", c.Dir)}
	}
	if !info.IsDir() {
		return CheckResult{Name: c.Name(), Passed: false, Message: fmt.Sprintf("not a directory: %s", c.Dir)}
	}
	return CheckResult{Name: c.Name(), Passed: true, Message: "OK"}
}

// FileCheck validates that a file exists.
type FileCheck struct {
	Path string
	Desc string
}

func (c FileCheck) Name() string { return "file:" + c.Desc }
func (c FileCheck) Run() CheckResult {
	_, err := os.Stat(c.Path)
	if err != nil {
		return CheckResult{Name: c.Name(), Passed: false, Message: fmt.Sprintf("file not found: %s", c.Path)}
	}
	return CheckResult{Name: c.Name(), Passed: true, Message: "OK"}
}

// BinaryCheck validates that an executable binary is available in PATH.
type BinaryCheck struct {
	Binary string
}

func (c BinaryCheck) Name() string { return "binary:" + c.Binary }
func (c BinaryCheck) Run() CheckResult {
	path, err := exec.LookPath(c.Binary)
	if err != nil {
		return CheckResult{Name: c.Name(), Passed: false, Message: fmt.Sprintf("%s not found in PATH", c.Binary)}
	}
	return CheckResult{Name: c.Name(), Passed: true, Message: fmt.Sprintf("found at %s", path)}
}

// CustomCheck wraps an arbitrary function as a check.
type CustomCheck struct {
	CheckName string
	Fn        func() CheckResult
}

func (c CustomCheck) Name() string { return c.CheckName }
func (c CustomCheck) Run() CheckResult { return c.Fn() }

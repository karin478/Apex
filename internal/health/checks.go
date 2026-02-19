package health

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/audit"
	"github.com/lyndonlyu/apex/internal/config"
	"github.com/lyndonlyu/apex/internal/sandbox"
)

// CheckAuditChain verifies the integrity of the audit hash chain.
func CheckAuditChain(baseDir string) ComponentStatus {
	cs := ComponentStatus{
		Name:     "audit_chain",
		Category: "critical",
	}

	logger, err := audit.NewLogger(filepath.Join(baseDir, "audit"))
	if err != nil {
		cs.Healthy = false
		cs.Detail = fmt.Sprintf("Failed to open audit log: %v", err)
		return cs
	}

	valid, _, err := logger.Verify()
	if err != nil {
		cs.Healthy = false
		cs.Detail = fmt.Sprintf("Verification error: %v", err)
		return cs
	}

	if !valid {
		cs.Healthy = false
		cs.Detail = "Hash chain broken"
		return cs
	}

	cs.Healthy = true
	cs.Detail = "Hash chain intact"
	return cs
}

// CheckSandbox detects the available sandbox and reports its status.
func CheckSandbox() ComponentStatus {
	cs := ComponentStatus{
		Name:     "sandbox_available",
		Category: "critical",
	}

	sb := sandbox.Detect()

	switch sb.(type) {
	case *sandbox.NoneSandbox:
		cs.Healthy = false
		cs.Detail = "No sandbox available"
	default:
		cs.Healthy = true
		switch sb.Level() {
		case sandbox.Docker:
			cs.Detail = "Docker sandbox available"
		case sandbox.Ulimit:
			cs.Detail = "Ulimit sandbox available"
		default:
			cs.Detail = fmt.Sprintf("%s sandbox available", sb.Level().String())
		}
	}

	return cs
}

// CheckConfig verifies that the configuration file can be loaded.
func CheckConfig(baseDir string) ComponentStatus {
	cs := ComponentStatus{
		Name:     "config",
		Category: "critical",
	}

	_, err := config.Load(filepath.Join(baseDir, "config.yaml"))
	if err != nil {
		cs.Healthy = false
		cs.Detail = fmt.Sprintf("Configuration error: %v", err)
		return cs
	}

	cs.Healthy = true
	cs.Detail = "Configuration loaded"
	return cs
}

// CheckKillSwitch checks whether the kill switch file is present.
func CheckKillSwitch(baseDir string) ComponentStatus {
	home, err := os.UserHomeDir()
	if err != nil {
		return ComponentStatus{
			Name:     "kill_switch",
			Category: "important",
			Healthy:  false,
			Detail:   fmt.Sprintf("Cannot determine home directory: %v", err),
		}
	}
	ksPath := filepath.Join(home, ".claude", "KILL_SWITCH")
	return checkKillSwitchAt(ksPath)
}

// checkKillSwitchAt is the internal implementation that checks a specific path.
// Exported for testing via the health package tests.
func checkKillSwitchAt(path string) ComponentStatus {
	cs := ComponentStatus{
		Name:     "kill_switch",
		Category: "important",
	}

	_, err := os.Stat(path)
	if err == nil {
		// File exists — kill switch is active
		cs.Healthy = false
		cs.Detail = "ACTIVE — use 'apex resume' to deactivate"
		return cs
	}

	cs.Healthy = true
	cs.Detail = "Not active"
	return cs
}

// CheckAuditDir checks that the audit directory exists and is writable.
func CheckAuditDir(baseDir string) ComponentStatus {
	return checkDirWritable(filepath.Join(baseDir, "audit"), "audit_dir", "important")
}

// CheckMemoryDir checks that the memory directory exists and is writable.
func CheckMemoryDir(baseDir string) ComponentStatus {
	return checkDirWritable(filepath.Join(baseDir, "memory"), "memory_dir", "important")
}

// checkDirWritable tests whether a directory exists and is writable by creating
// and immediately removing a temp file.
func checkDirWritable(dir, name, category string) ComponentStatus {
	cs := ComponentStatus{
		Name:     name,
		Category: category,
	}

	info, err := os.Stat(dir)
	if err != nil {
		cs.Healthy = false
		if os.IsNotExist(err) {
			cs.Detail = "Missing"
		} else {
			cs.Detail = fmt.Sprintf("Stat error: %v", err)
		}
		return cs
	}

	if !info.IsDir() {
		cs.Healthy = false
		cs.Detail = "Not a directory"
		return cs
	}

	// Try writing a temp file to verify writability
	tmp := filepath.Join(dir, ".health_check_tmp")
	if err := os.WriteFile(tmp, []byte("ok"), 0644); err != nil {
		cs.Healthy = false
		cs.Detail = "Not writable"
		return cs
	}
	os.Remove(tmp)

	cs.Healthy = true
	cs.Detail = "Writable"
	return cs
}

// CheckGitRepo checks whether the current working directory is inside a git repository.
func CheckGitRepo() ComponentStatus {
	cs := ComponentStatus{
		Name:     "git_repo",
		Category: "optional",
	}

	cmd := exec.Command("git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		cs.Healthy = false
		cs.Detail = "Not a git repository"
		return cs
	}

	cs.Healthy = true
	cs.Detail = "Git repository detected"
	return cs
}

// Evaluate runs ALL component health checks and returns a comprehensive Report.
func Evaluate(baseDir string) *Report {
	components := []ComponentStatus{
		CheckAuditChain(baseDir),
		CheckSandbox(),
		CheckConfig(baseDir),
		CheckKillSwitch(baseDir),
		CheckAuditDir(baseDir),
		CheckMemoryDir(baseDir),
		CheckGitRepo(),
	}

	return NewReport(components)
}

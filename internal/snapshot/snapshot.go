package snapshot

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const messagePrefix = "apex-"

type Snapshot struct {
	Index   int
	Message string
	RunID   string
}

type Manager struct {
	workDir string
}

func New(workDir string) *Manager {
	return &Manager{workDir: workDir}
}

func (m *Manager) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = m.workDir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (m *Manager) Create(runID string) (*Snapshot, error) {
	msg := fmt.Sprintf("%s%s-%s", messagePrefix, runID, time.Now().UTC().Format("20060102T150405Z"))
	out, err := m.git("stash", "push", "--include-untracked", "-m", msg)
	if err != nil {
		return nil, fmt.Errorf("git stash push failed: %w: %s", err, out)
	}
	if strings.Contains(out, "No local changes") {
		return nil, nil
	}
	return &Snapshot{Index: 0, Message: msg, RunID: runID}, nil
}

// Apply restores the stashed changes to the working tree without removing the
// stash entry. Use this right after Create to make the snapshot transparent
// (working tree keeps its original state, stash serves as a pure backup).
func (m *Manager) Apply(runID string) error {
	idx, err := m.findStash(runID)
	if err != nil {
		return err
	}
	out, err := m.git("stash", "apply", fmt.Sprintf("stash@{%d}", idx))
	if err != nil {
		return fmt.Errorf("git stash apply failed: %w: %s", err, out)
	}
	return nil
}

func (m *Manager) Restore(runID string) error {
	idx, err := m.findStash(runID)
	if err != nil {
		return err
	}
	// Save current working tree state before restoring, so user work isn't lost.
	safeMsg := fmt.Sprintf("apex-pre-restore-%s-%s", runID, time.Now().UTC().Format("20060102T150405Z"))
	stashOut, stashErr := m.git("stash", "push", "--include-untracked", "-m", safeMsg)
	hadChanges := stashErr == nil && !strings.Contains(stashOut, "No local changes")
	if hadChanges {
		// The stash indices shifted by 1 since we just pushed a new stash.
		idx++
	}
	out, err := m.git("stash", "pop", fmt.Sprintf("stash@{%d}", idx))
	if err != nil {
		// If restore fails and we stashed, recover the saved state.
		if hadChanges {
			m.git("stash", "pop", "stash@{0}")
		}
		return fmt.Errorf("git stash pop failed: %w: %s", err, out)
	}
	// If we saved current state, inform the user how to recover it.
	if hadChanges {
		fmt.Fprintf(os.Stderr, "Pre-restore working state saved as stash: %s\n", safeMsg)
	}
	return nil
}

func (m *Manager) List() ([]Snapshot, error) {
	out, err := m.git("stash", "list")
	if err != nil {
		return nil, fmt.Errorf("git stash list failed: %w", err)
	}
	if out == "" {
		return nil, nil
	}
	var snaps []Snapshot
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, messagePrefix) {
			continue
		}
		idx := parseStashIndex(line)
		msg := parseStashMessage(line)
		runID := parseRunID(msg)
		snaps = append(snaps, Snapshot{Index: idx, Message: msg, RunID: runID})
	}
	return snaps, nil
}

func (m *Manager) Drop(runID string) error {
	idx, err := m.findStash(runID)
	if err != nil {
		return err
	}
	out, err := m.git("stash", "drop", fmt.Sprintf("stash@{%d}", idx))
	if err != nil {
		return fmt.Errorf("git stash drop failed: %w: %s", err, out)
	}
	return nil
}

func (m *Manager) findStash(runID string) (int, error) {
	snaps, err := m.List()
	if err != nil {
		return -1, err
	}
	prefix := messagePrefix + runID
	for _, s := range snaps {
		if strings.HasPrefix(s.Message, prefix) {
			return s.Index, nil
		}
	}
	return -1, fmt.Errorf("no snapshot found for run %s", runID)
}

func parseStashIndex(line string) int {
	start := strings.Index(line, "{")
	end := strings.Index(line, "}")
	if start == -1 || end == -1 {
		return 0
	}
	var idx int
	fmt.Sscanf(line[start+1:end], "%d", &idx)
	return idx
}

func parseStashMessage(line string) string {
	idx := strings.Index(line, messagePrefix)
	if idx == -1 {
		return ""
	}
	return strings.TrimSpace(line[idx:])
}

func parseRunID(msg string) string {
	if !strings.HasPrefix(msg, messagePrefix) {
		return ""
	}
	rest := msg[len(messagePrefix):]
	lastDash := strings.LastIndex(rest, "-")
	if lastDash == -1 {
		return rest
	}
	return rest[:lastDash]
}

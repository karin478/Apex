package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"
)

// PolicyFile represents a tracked configuration file and its checksum.
type PolicyFile struct {
	Path     string `json:"path"`
	Checksum string `json:"checksum"`
}

// PolicyChange represents a detected change in a tracked file.
type PolicyChange struct {
	File        string `json:"file"`
	OldChecksum string `json:"old_checksum"`
	NewChecksum string `json:"new_checksum"`
	Timestamp   string `json:"timestamp"`
}

// PolicyTracker monitors configuration files for changes via SHA-256 checksums.
type PolicyTracker struct {
	stateDir string
}

// NewPolicyTracker creates a new PolicyTracker that persists state in the given directory.
func NewPolicyTracker(stateDir string) *PolicyTracker {
	return &PolicyTracker{stateDir: stateDir}
}

// Check computes SHA-256 checksums for each file path, compares with stored state,
// returns any changes detected, and updates the persisted state.
// First-time files get an empty OldChecksum. Missing files are silently skipped.
func (t *PolicyTracker) Check(files []string) ([]PolicyChange, error) {
	// Load existing state
	existing, err := t.State()
	if err != nil {
		return nil, err
	}

	// Build lookup map from existing state
	stateMap := make(map[string]string)
	for _, pf := range existing {
		stateMap[pf.Path] = pf.Checksum
	}

	var changes []PolicyChange
	now := time.Now().UTC().Format(time.RFC3339)

	for _, file := range files {
		checksum, err := fileChecksum(file)
		if err != nil {
			// Silently skip missing or unreadable files
			continue
		}

		oldChecksum, tracked := stateMap[file]
		if !tracked {
			// New file — record change with empty OldChecksum
			changes = append(changes, PolicyChange{
				File:        file,
				OldChecksum: "",
				NewChecksum: checksum,
				Timestamp:   now,
			})
		} else if oldChecksum != checksum {
			// Existing file changed
			changes = append(changes, PolicyChange{
				File:        file,
				OldChecksum: oldChecksum,
				NewChecksum: checksum,
				Timestamp:   now,
			})
		}

		// Update state map
		stateMap[file] = checksum
	}

	// Rebuild state slice from map
	var updated []PolicyFile
	for path, checksum := range stateMap {
		updated = append(updated, PolicyFile{Path: path, Checksum: checksum})
	}

	if err := t.saveState(updated); err != nil {
		return nil, err
	}

	return changes, nil
}

// State loads the current tracked file states from policy-state.json.
// Returns nil, nil if the state file does not exist.
func (t *PolicyTracker) State() ([]PolicyFile, error) {
	statePath := filepath.Join(t.stateDir, "policy-state.json")

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []PolicyFile
	if err := json.Unmarshal(data, &files); err != nil {
		return nil, err
	}

	return files, nil
}

// saveState persists the given policy file states to policy-state.json.
func (t *PolicyTracker) saveState(files []PolicyFile) error {
	if err := os.MkdirAll(t.stateDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return err
	}

	statePath := filepath.Join(t.stateDir, "policy-state.json")
	return os.WriteFile(statePath, data, 0o644)
}

// fileChecksum computes the SHA-256 checksum of the file at the given path.
func fileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

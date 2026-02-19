package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Anchor struct {
	Date        string `json:"date"`
	ChainHash   string `json:"chain_hash"`
	RecordCount int    `json:"record_count"`
	CreatedAt   string `json:"created_at"`
	GitTag      string `json:"git_tag,omitempty"`
}

const anchorsFile = "anchors.jsonl"

func LoadAnchors(auditDir string) ([]Anchor, error) {
	path := filepath.Join(auditDir, anchorsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil, nil
	}
	lines := strings.Split(content, "\n")
	var anchors []Anchor
	for _, line := range lines {
		var a Anchor
		if err := json.Unmarshal([]byte(line), &a); err != nil {
			return nil, fmt.Errorf("parse anchor: %w", err)
		}
		anchors = append(anchors, a)
	}
	return anchors, nil
}

func WriteAnchor(auditDir string, anchor Anchor) error {
	existing, err := LoadAnchors(auditDir)
	if err != nil {
		return err
	}
	found := false
	for i, a := range existing {
		if a.Date == anchor.Date {
			existing[i] = anchor
			found = true
			break
		}
	}
	if !found {
		existing = append(existing, anchor)
	}
	path := filepath.Join(auditDir, anchorsFile)
	tmp := path + ".tmp"
	var buf strings.Builder
	for _, a := range existing {
		data, err := json.Marshal(a)
		if err != nil {
			return err
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(tmp, []byte(buf.String()), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// createGitTag creates an annotated git tag in the working directory.
// Returns true if successful, false otherwise (best-effort, never errors).
func createGitTag(workDir, tagName, chainHash string, recordCount int) bool {
	msg := fmt.Sprintf("Daily audit anchor: %s (%d records)", chainHash, recordCount)
	cmd := exec.Command("git", "tag", "-f", "-a", tagName, "-m", msg)
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// MaybeCreateAnchor checks today's audit records and creates/updates the daily anchor.
// workDir is used for git tag creation (empty string = skip git tag).
// Returns (true, nil) if an anchor was created or updated.
func MaybeCreateAnchor(logger *Logger, workDir string) (bool, error) {
	today := time.Now().Format("2006-01-02")
	hash, count, err := logger.LastHashForDate(today)
	if err != nil {
		return false, fmt.Errorf("read audit records: %w", err)
	}
	if count == 0 {
		return false, nil
	}
	existing, err := LoadAnchors(logger.Dir())
	if err != nil {
		return false, fmt.Errorf("load anchors: %w", err)
	}
	for _, a := range existing {
		if a.Date == today && a.ChainHash == hash {
			return false, nil
		}
	}
	tagName := "apex-audit-anchor-" + today
	anchor := Anchor{
		Date:        today,
		ChainHash:   hash,
		RecordCount: count,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		GitTag:      tagName,
	}
	if err := WriteAnchor(logger.Dir(), anchor); err != nil {
		return false, fmt.Errorf("write anchor: %w", err)
	}
	if workDir != "" {
		createGitTag(workDir, tagName, hash, count)
	}
	return true, nil
}

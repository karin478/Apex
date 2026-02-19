package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

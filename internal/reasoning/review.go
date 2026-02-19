package reasoning

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Step struct {
	Role   string `json:"role"`
	Prompt string `json:"prompt"`
	Output string `json:"output"`
}

type Verdict struct {
	Decision string   `json:"decision"`
	Summary  string   `json:"summary"`
	Risks    []string `json:"risks"`
	Actions  []string `json:"suggested_actions"`
}

type ReviewResult struct {
	ID         string  `json:"id"`
	Proposal   string  `json:"proposal"`
	CreatedAt  string  `json:"created_at"`
	Steps      []Step  `json:"steps"`
	Verdict    Verdict `json:"verdict"`
	DurationMs int64   `json:"duration_ms"`
}

func SaveReview(dir string, result *ReviewResult) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, result.ID+".json")
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadReview(dir, id string) (*ReviewResult, error) {
	path := filepath.Join(dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load review %s: %w", id, err)
	}
	var result ReviewResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse review %s: %w", id, err)
	}
	return &result, nil
}

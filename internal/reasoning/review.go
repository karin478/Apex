package reasoning

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

var jsonBlockRe = regexp.MustCompile("(?s)```(?:json)?\\s*\\n(\\{.*?\\})\\s*\\n```")

func parseVerdict(output string) (Verdict, error) {
	var v Verdict

	// Try direct JSON parse
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &v); err == nil && v.Decision != "" {
		return v, nil
	}

	// Try extracting from markdown code block
	if matches := jsonBlockRe.FindStringSubmatch(output); len(matches) > 1 {
		if err := json.Unmarshal([]byte(matches[1]), &v); err == nil && v.Decision != "" {
			return v, nil
		}
	}

	// Fallback: use full output as summary
	return Verdict{
		Decision: "revise",
		Summary:  strings.TrimSpace(output),
	}, nil
}

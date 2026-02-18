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
				Path:    rel,
				Type:    memType,
				Snippet: snippet,
			})
		}
		return nil
	})

	return results, err
}

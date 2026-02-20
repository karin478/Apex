package memport

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExportEntry represents a single memory file in the export payload.
type ExportEntry struct {
	Key       string `json:"key"`        // relative path within memory dir
	Value     string `json:"value"`      // full file content
	Category  string `json:"category"`   // subdirectory name (decisions/facts/sessions)
	CreatedAt string `json:"created_at"` // file mod time RFC3339
}

// ExportData is the top-level envelope for an exported memory archive.
type ExportData struct {
	Version    string        `json:"version"`     // always "1"
	ExportedAt string        `json:"exported_at"` // export timestamp RFC3339
	Count      int           `json:"count"`       // len(Entries)
	Entries    []ExportEntry `json:"entries"`
}

// MergeStrategy controls how Import handles existing files.
type MergeStrategy string

const (
	MergeSkip      MergeStrategy = "skip"
	MergeOverwrite MergeStrategy = "overwrite"
)

// ImportResult summarises what Import did.
type ImportResult struct {
	Added       int `json:"added"`
	Skipped     int `json:"skipped"`
	Overwritten int `json:"overwritten"`
}

// Export walks memDir and collects all .md and .jsonl files into an ExportData.
// If category is non-empty, only files under that subdirectory are collected.
// A non-existent memDir returns an empty ExportData (not an error).
func Export(memDir string, category string) (*ExportData, error) {
	data := &ExportData{
		Version:    "1",
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Entries:    []ExportEntry{},
	}

	// Non-existent dir is not an error — just return empty.
	if _, err := os.Stat(memDir); os.IsNotExist(err) {
		return data, nil
	}

	err := filepath.Walk(memDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".jsonl" {
			return nil
		}

		rel, err := filepath.Rel(memDir, path)
		if err != nil {
			return nil
		}

		// Determine category from first path component.
		parts := strings.SplitN(rel, string(filepath.Separator), 2)
		cat := ""
		if len(parts) > 1 {
			cat = parts[0]
		}

		// Filter by category if requested.
		if category != "" && cat != category {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		data.Entries = append(data.Entries, ExportEntry{
			Key:       rel,
			Value:     string(content),
			Category:  cat,
			CreatedAt: info.ModTime().UTC().Format(time.RFC3339),
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	data.Count = len(data.Entries)
	return data, nil
}

// Import writes the entries from data into memDir according to the given
// merge strategy. Returns counts of added, skipped, and overwritten files.
func Import(memDir string, data *ExportData, strategy MergeStrategy) (*ImportResult, error) {
	if data == nil {
		return nil, errors.New("memport: nil export data")
	}

	result := &ImportResult{}

	for _, entry := range data.Entries {
		fullPath := filepath.Join(memDir, entry.Key)

		_, err := os.Stat(fullPath)
		exists := err == nil

		if exists {
			switch strategy {
			case MergeSkip:
				result.Skipped++
				continue
			case MergeOverwrite:
				if err := os.WriteFile(fullPath, []byte(entry.Value), 0644); err != nil {
					return nil, err
				}
				result.Overwritten++
			}
		} else {
			// File doesn't exist — create parent dirs and write.
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(fullPath, []byte(entry.Value), 0644); err != nil {
				return nil, err
			}
			result.Added++
		}
	}

	return result, nil
}

// WriteFile serialises data as indented JSON and writes it to path.
func WriteFile(path string, data *ExportData) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

// ReadFile reads a JSON file at path and deserialises it into ExportData.
func ReadFile(path string) (*ExportData, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data ExportData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

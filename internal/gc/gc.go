package gc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Policy defines retention rules for garbage collection.
type Policy struct {
	MaxAgeDays   int  // delete runs older than N days (default: 30)
	MaxRuns      int  // keep at most N recent runs (default: 100)
	MaxAuditDays int  // keep audit logs for N days (default: 90)
	DryRun       bool // report without deleting
}

// Result tracks what was cleaned up.
type Result struct {
	RunsRemoved       int
	AuditFilesRemoved int
	BytesFreed        int64
}

// DefaultPolicy returns the default GC policy.
func DefaultPolicy() Policy {
	return Policy{
		MaxAgeDays:   30,
		MaxRuns:      100,
		MaxAuditDays: 90,
	}
}

// Run performs garbage collection on the apex base directory.
func Run(baseDir string, policy Policy) (*Result, error) {
	result := &Result{}

	// Clean runs
	if err := cleanRuns(filepath.Join(baseDir, "runs"), policy, result); err != nil {
		return result, fmt.Errorf("run cleanup: %w", err)
	}

	// Clean audit logs
	if err := cleanAudit(filepath.Join(baseDir, "audit"), policy, result); err != nil {
		return result, fmt.Errorf("audit cleanup: %w", err)
	}

	return result, nil
}

type runInfo struct {
	dir       string
	timestamp time.Time
	size      int64
}

func cleanRuns(runsDir string, policy Policy, result *Result) error {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var runs []runInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(runsDir, entry.Name(), "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue // skip invalid run dirs
		}

		var m struct {
			Timestamp string `json:"timestamp"`
		}
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}

		ts, err := time.Parse(time.RFC3339, m.Timestamp)
		if err != nil {
			continue
		}

		size := dirSize(filepath.Join(runsDir, entry.Name()))
		runs = append(runs, runInfo{dir: filepath.Join(runsDir, entry.Name()), timestamp: ts, size: size})
	}

	// Sort by timestamp descending (newest first)
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].timestamp.After(runs[j].timestamp)
	})

	cutoff := time.Now().AddDate(0, 0, -policy.MaxAgeDays)

	for i, r := range runs {
		// Keep if within MaxRuns AND within MaxAgeDays
		if i < policy.MaxRuns && r.timestamp.After(cutoff) {
			continue
		}
		// Delete
		if !policy.DryRun {
			if err := os.RemoveAll(r.dir); err != nil {
				return err
			}
		}
		result.RunsRemoved++
		result.BytesFreed += r.size
	}

	return nil
}

func cleanAudit(auditDir string, policy Policy, result *Result) error {
	entries, err := os.ReadDir(auditDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	cutoff := time.Now().AddDate(0, 0, -policy.MaxAuditDays)

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}

		// Parse date from filename (YYYY-MM-DD.jsonl)
		datePart := strings.TrimSuffix(name, ".jsonl")
		fileDate, err := time.Parse("2006-01-02", datePart)
		if err != nil {
			continue // skip non-date files (e.g., anchors.jsonl)
		}

		if fileDate.Before(cutoff) {
			path := filepath.Join(auditDir, name)
			info, _ := entry.Info()
			var fileSize int64
			if info != nil {
				fileSize = info.Size()
			}

			if !policy.DryRun {
				if err := os.Remove(path); err != nil {
					return err
				}
			}
			result.AuditFilesRemoved++
			result.BytesFreed += fileSize
		}
	}

	return nil
}

func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

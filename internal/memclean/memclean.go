// Package memclean implements memory cleanup: scan, evaluate, and execute
// removal of stale, low-confidence memory entries.
package memclean

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MemoryEntry represents a single memory file discovered during a scan.
type MemoryEntry struct {
	Path       string    `json:"path"`       // relative path in memDir
	Category   string    `json:"category"`   // subdirectory name
	Size       int64     `json:"size"`
	ModTime    time.Time `json:"mod_time"`
	Confidence float64   `json:"confidence"` // default 0.5 for files without explicit confidence
}

// CleanupConfig controls the cleanup behaviour.
type CleanupConfig struct {
	CapacityThreshold float64  `json:"capacity_threshold"`  // fraction of MaxEntries that triggers cleanup
	MaxEntries        int      `json:"max_entries"`         // maximum number of entries before cleanup
	ConfidenceMin     float64  `json:"confidence_min"`      // entries below this confidence may be removed
	StaleAfterDays    int      `json:"stale_after_days"`    // entries older than this are considered stale
	ExemptCategories  []string `json:"exempt_categories"`   // categories that are never removed
}

// CleanupResult summarises the outcome of a cleanup operation.
type CleanupResult struct {
	Scanned   int `json:"scanned"`
	Removed   int `json:"removed"`
	Exempted  int `json:"exempted"`
	Remaining int `json:"remaining"`
}

// DefaultConfig returns a CleanupConfig with sensible defaults.
func DefaultConfig() CleanupConfig {
	return CleanupConfig{
		CapacityThreshold: 0.8,
		MaxEntries:        1000,
		ConfidenceMin:     0.3,
		StaleAfterDays:    30,
		ExemptCategories:  []string{"decisions", "preferences"},
	}
}

// Scan walks memDir and collects all .md and .jsonl files as MemoryEntry values.
// Category is determined by the first path component (subdirectory name).
// Files without explicit confidence metadata receive a default of 0.5.
func Scan(memDir string) ([]MemoryEntry, error) {
	var entries []MemoryEntry

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

		// Determine category from the first path component.
		parts := strings.SplitN(rel, string(filepath.Separator), 2)
		category := ""
		if len(parts) > 1 {
			category = parts[0]
		}

		entries = append(entries, MemoryEntry{
			Path:       rel,
			Category:   category,
			Size:       info.Size(),
			ModTime:    info.ModTime(),
			Confidence: 0.5,
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// buildExemptSet returns a set of category names that should be exempt from cleanup.
func buildExemptSet(cats []string) map[string]bool {
	m := make(map[string]bool, len(cats))
	for _, cat := range cats {
		m[cat] = true
	}
	return m
}

// Evaluate determines which entries should be removed and which should be kept.
// If the number of entries is below the capacity threshold (MaxEntries * CapacityThreshold),
// no entries are marked for removal. Otherwise, entries that are:
//   - NOT in an exempt category, AND
//   - have confidence below ConfidenceMin, AND
//   - have a ModTime before now minus StaleAfterDays
//
// are marked for removal.
func Evaluate(entries []MemoryEntry, cfg CleanupConfig, now time.Time) (toRemove, toKeep []MemoryEntry) {
	threshold := int(float64(cfg.MaxEntries) * cfg.CapacityThreshold)

	if len(entries) < threshold {
		return nil, entries
	}

	exempt := buildExemptSet(cfg.ExemptCategories)

	staleCutoff := now.AddDate(0, 0, -cfg.StaleAfterDays)

	for _, entry := range entries {
		if exempt[entry.Category] {
			toKeep = append(toKeep, entry)
			continue
		}
		if entry.Confidence < cfg.ConfidenceMin && entry.ModTime.Before(staleCutoff) {
			toRemove = append(toRemove, entry)
		} else {
			toKeep = append(toKeep, entry)
		}
	}

	return toRemove, toKeep
}

// Execute removes the files listed in toRemove from memDir using a best-effort
// approach: it processes every entry even if some removals fail, and returns a
// partial CleanupResult with the count of successfully removed files. If any
// non-NotExist errors occurred, they are collected and returned as a single
// combined error after all entries have been attempted.
func Execute(memDir string, toRemove []MemoryEntry) (*CleanupResult, int, error) {
	removed := 0
	var errs []error

	for _, entry := range toRemove {
		fullPath := filepath.Join(memDir, entry.Path)
		if err := os.Remove(fullPath); err != nil {
			if !os.IsNotExist(err) {
				errs = append(errs, fmt.Errorf("remove %s: %w", entry.Path, err))
				continue
			}
			// File already gone — count it as removed.
		}
		removed++
	}

	result := &CleanupResult{
		Removed: removed,
	}

	if len(errs) > 0 {
		return result, removed, fmt.Errorf("failed to remove %d of %d entries: %v", len(errs), len(toRemove), errs)
	}

	return result, removed, nil
}

// DryRun performs Scan + Evaluate without deleting anything. It returns the
// full CleanupResult and the list of entries that would be removed.
func DryRun(memDir string, cfg CleanupConfig) (*CleanupResult, []MemoryEntry, error) {
	entries, err := Scan(memDir)
	if err != nil {
		return nil, nil, err
	}

	toRemove, toKeep := Evaluate(entries, cfg, time.Now())

	// Count exempted entries (entries in exempt categories).
	exempt := buildExemptSet(cfg.ExemptCategories)
	exempted := 0
	for _, entry := range toKeep {
		if exempt[entry.Category] {
			exempted++
		}
	}

	result := &CleanupResult{
		Scanned:   len(entries),
		Removed:   len(toRemove),
		Exempted:  exempted,
		Remaining: len(toKeep),
	}

	return result, toRemove, nil
}

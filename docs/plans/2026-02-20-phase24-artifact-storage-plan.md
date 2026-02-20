# Phase 24: Content-Addressed Artifact Storage — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** SHA-256 content-addressed artifact storage with save, query, lineage tracking, orphan detection, and CLI management commands.

**Architecture:** New `internal/artifact` package with `Store` that persists an `index.json` metadata file and content blobs under `blobs/{hash[:2]}/{hash}`. CLI commands `apex artifact list/info/gc` for management.

**Tech Stack:** Go, Cobra CLI, Testify, crypto/sha256, encoding/json

---

## Task 1: Artifact Store Core — Save, Get, Data, List

**Files:**
- Create: `internal/artifact/store.go`
- Create: `internal/artifact/store_test.go`

**Implementation:** `store.go`

```go
package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Artifact represents a stored content-addressed artifact.
type Artifact struct {
	Hash      string `json:"hash"`
	Name      string `json:"name"`
	RunID     string `json:"run_id"`
	NodeID    string `json:"node_id"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
}

// Store manages content-addressed artifact storage.
type Store struct {
	dir string
}

// NewStore creates a Store rooted at dir.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

func (s *Store) indexPath() string {
	return filepath.Join(s.dir, "index.json")
}

func (s *Store) blobPath(hash string) string {
	return filepath.Join(s.dir, "blobs", hash[:2], hash)
}

// loadIndex reads the index from disk.
func (s *Store) loadIndex() ([]*Artifact, error) {
	data, err := os.ReadFile(s.indexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var artifacts []*Artifact
	if err := json.Unmarshal(data, &artifacts); err != nil {
		return nil, err
	}
	return artifacts, nil
}

// saveIndex writes the index to disk.
func (s *Store) saveIndex(artifacts []*Artifact) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(artifacts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.indexPath(), data, 0o644)
}

// Save stores artifact data with content-addressing. Returns existing artifact if content already stored.
func (s *Store) Save(name string, data []byte, runID, nodeID string) (*Artifact, error) {
	h := sha256.Sum256(data)
	hash := hex.EncodeToString(h[:])

	artifacts, err := s.loadIndex()
	if err != nil {
		return nil, err
	}

	// Check for duplicate content
	for _, a := range artifacts {
		if a.Hash == hash {
			return a, nil
		}
	}

	// Write blob
	blobDir := filepath.Join(s.dir, "blobs", hash[:2])
	if err := os.MkdirAll(blobDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(s.blobPath(hash), data, 0o644); err != nil {
		return nil, err
	}

	art := &Artifact{
		Hash:      hash,
		Name:      name,
		RunID:     runID,
		NodeID:    nodeID,
		Size:      int64(len(data)),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	artifacts = append(artifacts, art)
	if err := s.saveIndex(artifacts); err != nil {
		return nil, err
	}

	return art, nil
}

// Get returns the artifact metadata for the given hash.
func (s *Store) Get(hash string) (*Artifact, error) {
	artifacts, err := s.loadIndex()
	if err != nil {
		return nil, err
	}
	for _, a := range artifacts {
		if a.Hash == hash {
			return a, nil
		}
	}
	return nil, fmt.Errorf("artifact not found: %s", hash)
}

// Data reads the artifact content for the given hash.
func (s *Store) Data(hash string) ([]byte, error) {
	return os.ReadFile(s.blobPath(hash))
}

// List returns all stored artifacts.
func (s *Store) List() ([]*Artifact, error) {
	return s.loadIndex()
}

// ListByRun returns artifacts belonging to a specific run.
func (s *Store) ListByRun(runID string) ([]*Artifact, error) {
	all, err := s.loadIndex()
	if err != nil {
		return nil, err
	}
	var filtered []*Artifact
	for _, a := range all {
		if a.RunID == runID {
			filtered = append(filtered, a)
		}
	}
	return filtered, nil
}

// Remove deletes an artifact's blob and removes it from the index.
func (s *Store) Remove(hash string) error {
	artifacts, err := s.loadIndex()
	if err != nil {
		return err
	}
	var updated []*Artifact
	found := false
	for _, a := range artifacts {
		if a.Hash == hash {
			found = true
			continue
		}
		updated = append(updated, a)
	}
	if !found {
		return fmt.Errorf("artifact not found: %s", hash)
	}
	os.Remove(s.blobPath(hash))
	return s.saveIndex(updated)
}

// FindOrphans returns artifacts whose RunID is not in validRunIDs.
func (s *Store) FindOrphans(validRunIDs map[string]bool) ([]*Artifact, error) {
	all, err := s.loadIndex()
	if err != nil {
		return nil, err
	}
	var orphans []*Artifact
	for _, a := range all {
		if !validRunIDs[a.RunID] {
			orphans = append(orphans, a)
		}
	}
	return orphans, nil
}
```

**Tests (8):**

```go
package artifact

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndGet(t *testing.T) {
	store := NewStore(t.TempDir())
	art, err := store.Save("test.txt", []byte("hello"), "run-1", "node-1")
	require.NoError(t, err)
	assert.NotEmpty(t, art.Hash)
	assert.Equal(t, "test.txt", art.Name)
	assert.Equal(t, int64(5), art.Size)

	got, err := store.Get(art.Hash)
	require.NoError(t, err)
	assert.Equal(t, art.Hash, got.Hash)
}

func TestSaveDeduplicates(t *testing.T) {
	store := NewStore(t.TempDir())
	a1, _ := store.Save("file1", []byte("same"), "run-1", "n1")
	a2, _ := store.Save("file2", []byte("same"), "run-2", "n2")
	assert.Equal(t, a1.Hash, a2.Hash)

	all, _ := store.List()
	assert.Len(t, all, 1) // only stored once
}

func TestData(t *testing.T) {
	store := NewStore(t.TempDir())
	art, _ := store.Save("f", []byte("content"), "r", "n")
	data, err := store.Data(art.Hash)
	require.NoError(t, err)
	assert.Equal(t, []byte("content"), data)
}

func TestListEmpty(t *testing.T) {
	store := NewStore(t.TempDir())
	list, err := store.List()
	require.NoError(t, err)
	assert.Nil(t, list)
}

func TestListByRun(t *testing.T) {
	store := NewStore(t.TempDir())
	store.Save("a", []byte("aa"), "run-1", "n1")
	store.Save("b", []byte("bb"), "run-2", "n1")
	store.Save("c", []byte("cc"), "run-1", "n2")

	filtered, err := store.ListByRun("run-1")
	require.NoError(t, err)
	assert.Len(t, filtered, 2)
}

func TestRemove(t *testing.T) {
	store := NewStore(t.TempDir())
	art, _ := store.Save("f", []byte("data"), "r", "n")
	err := store.Remove(art.Hash)
	require.NoError(t, err)

	_, err = store.Get(art.Hash)
	assert.Error(t, err)
}

func TestRemoveNotFound(t *testing.T) {
	store := NewStore(t.TempDir())
	err := store.Remove("nonexistent")
	assert.Error(t, err)
}

func TestFindOrphans(t *testing.T) {
	store := NewStore(t.TempDir())
	store.Save("a", []byte("aa"), "run-1", "n1")
	store.Save("b", []byte("bb"), "run-2", "n1")

	orphans, err := store.FindOrphans(map[string]bool{"run-1": true})
	require.NoError(t, err)
	assert.Len(t, orphans, 1)
	assert.Equal(t, "run-2", orphans[0].RunID)
}
```

**Commit:** `feat(artifact): add content-addressed artifact Store with Save, Get, List, Remove, FindOrphans`

---

## Task 2: CLI Command — `apex artifact`

**Files:**
- Create: `cmd/apex/artifact.go`
- Modify: `cmd/apex/main.go` (add `rootCmd.AddCommand(artifactCmd)`)

**Implementation:** `artifact.go` with subcommands: list, info, gc.

```go
package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/lyndonlyu/apex/internal/artifact"
	"github.com/lyndonlyu/apex/internal/manifest"
	"github.com/spf13/cobra"
)

var artifactRunFilter string
var artifactGCDryRun bool

var artifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Manage content-addressed artifacts",
}

var artifactListCmd = &cobra.Command{
	Use:   "list",
	Short: "List stored artifacts",
	RunE:  listArtifacts,
}

var artifactInfoCmd = &cobra.Command{
	Use:   "info <hash>",
	Short: "Show artifact details",
	Args:  cobra.ExactArgs(1),
	RunE:  infoArtifact,
}

var artifactGCCmd = &cobra.Command{
	Use:   "gc",
	Short: "Remove orphan artifacts",
	RunE:  gcArtifacts,
}

func init() {
	artifactListCmd.Flags().StringVar(&artifactRunFilter, "run", "", "Filter by run ID")
	artifactGCCmd.Flags().BoolVar(&artifactGCDryRun, "dry-run", false, "Preview without deleting")
	artifactCmd.AddCommand(artifactListCmd, artifactInfoCmd, artifactGCCmd)
}

func artifactStore() *artifact.Store {
	home, _ := os.UserHomeDir()
	return artifact.NewStore(filepath.Join(home, ".apex", "artifacts"))
}

func listArtifacts(cmd *cobra.Command, args []string) error {
	store := artifactStore()
	var list []*artifact.Artifact
	var err error
	if artifactRunFilter != "" {
		list, err = store.ListByRun(artifactRunFilter)
	} else {
		list, err = store.List()
	}
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("No artifacts stored.")
		return nil
	}
	fmt.Printf("%-12s | %-20s | %-12s | %s\n", "HASH", "NAME", "RUN", "SIZE")
	fmt.Println(strings.Repeat("-", 60))
	for _, a := range list {
		hash := a.Hash
		if len(hash) > 10 {
			hash = hash[:10] + ".."
		}
		runID := a.RunID
		if len(runID) > 10 {
			runID = runID[:10] + ".."
		}
		fmt.Printf("%-12s | %-20s | %-12s | %d B\n", hash, a.Name, runID, a.Size)
	}
	return nil
}

func infoArtifact(cmd *cobra.Command, args []string) error {
	store := artifactStore()
	art, err := store.Get(args[0])
	if err != nil {
		return err
	}
	fmt.Printf("Hash:       %s\n", art.Hash)
	fmt.Printf("Name:       %s\n", art.Name)
	fmt.Printf("Run ID:     %s\n", art.RunID)
	fmt.Printf("Node ID:    %s\n", art.NodeID)
	fmt.Printf("Size:       %d bytes\n", art.Size)
	fmt.Printf("Created At: %s\n", art.CreatedAt)
	return nil
}

func gcArtifacts(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, ".apex")
	store := artifact.NewStore(filepath.Join(baseDir, "artifacts"))

	// Build valid run IDs from manifest store
	runsDir := filepath.Join(baseDir, "runs")
	mStore := manifest.NewStore(runsDir)
	manifests, err := mStore.Recent(math.MaxInt)
	if err != nil {
		return fmt.Errorf("reading runs: %w", err)
	}
	validRuns := make(map[string]bool)
	for _, m := range manifests {
		validRuns[m.RunID] = true
	}

	orphans, err := store.FindOrphans(validRuns)
	if err != nil {
		return err
	}
	if len(orphans) == 0 {
		fmt.Println("No orphan artifacts found.")
		return nil
	}

	fmt.Printf("Found %d orphan artifact(s):\n", len(orphans))
	for _, a := range orphans {
		fmt.Printf("  %s  %s  (run: %s)\n", a.Hash[:10], a.Name, a.RunID)
	}

	if artifactGCDryRun {
		fmt.Println("\n[DRY RUN] No artifacts removed.")
		return nil
	}

	for _, a := range orphans {
		store.Remove(a.Hash)
	}
	fmt.Printf("\nRemoved %d orphan artifact(s).\n", len(orphans))
	return nil
}
```

**Register in main.go:** Add `rootCmd.AddCommand(artifactCmd)` after `dashboardCmd`.

**Commit:** `feat(cli): add apex artifact list/info/gc commands`

---

## Task 3: E2E Tests

**Files:**
- Create: `e2e/artifact_test.go`

**Tests (3):**

```go
package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArtifactListEmpty(t *testing.T) {
	env := newTestEnv(t)
	stdout, stderr, code := env.runApex("artifact", "list")
	assert.Equal(t, 0, code, "stderr=%s", stderr)
	assert.Contains(t, stdout, "No artifacts stored")
}

func TestArtifactInfoNotFound(t *testing.T) {
	env := newTestEnv(t)
	_, stderr, code := env.runApex("artifact", "info", "nonexistent")
	assert.NotEqual(t, 0, code)
	assert.Contains(t, stderr, "not found")
}

func TestArtifactGCDryRun(t *testing.T) {
	env := newTestEnv(t)

	// Create a fake artifact index with an orphan run
	artDir := filepath.Join(env.Home, ".apex", "artifacts")
	require.NoError(t, os.MkdirAll(filepath.Join(artDir, "blobs", "ab"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(artDir, "blobs", "ab", "ab1234567890"), []byte("data"), 0644))

	index := []map[string]interface{}{
		{"hash": "ab1234567890", "name": "test.txt", "run_id": "orphan-run", "node_id": "n1", "size": 4, "created_at": "2026-01-01T00:00:00Z"},
	}
	data, _ := json.Marshal(index)
	require.NoError(t, os.WriteFile(filepath.Join(artDir, "index.json"), data, 0644))

	stdout, stderr, code := env.runApex("artifact", "gc", "--dry-run")
	assert.Equal(t, 0, code, "stderr=%s", stderr)
	assert.Contains(t, stdout, "orphan")
	assert.Contains(t, stdout, "DRY RUN")
}
```

**Commit:** `test(e2e): add artifact command E2E tests`

---

## Task 4: Update PROGRESS.md

Update Phase table, test counts, add artifact package.

**Commit:** `docs: mark Phase 24 Content-Addressed Artifact Storage as complete`

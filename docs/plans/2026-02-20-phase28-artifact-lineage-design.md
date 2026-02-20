# Phase 28: Artifact Lineage — Design Document

**Date:** 2026-02-20
**Status:** Approved (recommendation credit 6/10)

## Overview

Add dependency tracking and impact analysis to the artifact storage system. Enables `apex artifact impact <hash>` to show all downstream artifacts affected by a change, and `apex artifact deps <hash>` for direct dependencies.

## Architecture

### Storage

Dependencies stored in `{artifact_dir}/deps.json` as a JSON array:

```json
[
  {"from_hash": "abc123", "to_hash": "def456"},
  {"from_hash": "ghi789", "to_hash": "def456"}
]
```

Meaning: `abc123` depends on `def456`, and `ghi789` depends on `def456`. If `def456` changes, both `abc123` and `ghi789` are affected.

### Core Types

```go
type Dependency struct {
    FromHash string `json:"from_hash"` // the artifact that depends
    ToHash   string `json:"to_hash"`   // the artifact being depended on
}

type LineageGraph struct {
    dir   string
    store *Store
    deps  []Dependency
}

type ImpactResult struct {
    Root     *Artifact   `json:"root"`
    Affected []*Artifact `json:"affected"`
    Depth    int         `json:"depth"`
}
```

### Operations

```go
func NewLineageGraph(dir string, store *Store) (*LineageGraph, error) // Load deps.json
func (lg *LineageGraph) AddDependency(fromHash, toHash string) error  // Add + dedup
func (lg *LineageGraph) RemoveDependency(fromHash, toHash string)     // Remove
func (lg *LineageGraph) DirectDeps(hash string) []string              // Direct dependencies (toHash list)
func (lg *LineageGraph) DirectDependents(hash string) []string        // Who depends on this (fromHash list)
func (lg *LineageGraph) Impact(hash string) (*ImpactResult, error)    // BFS all downstream dependents
func (lg *LineageGraph) Save() error                                  // Persist deps.json
```

### Impact Algorithm

BFS starting from hash:
1. Find all artifacts where `ToHash == hash` (direct dependents)
2. For each dependent, recursively find their dependents
3. Track visited to avoid cycles
4. Collect all unique affected artifacts
5. Depth = max BFS level reached

### CLI Commands

```
apex artifact impact <hash>    Show all downstream artifacts affected by this artifact
apex artifact deps <hash>      Show direct dependencies of this artifact
```

### Format

Impact output:
```
Artifact: <name> (<hash[:12]>)

Impact chain (N affected):
  L1: <name> (<hash[:12]>)
  L1: <name> (<hash[:12]>)
  L2: <name> (<hash[:12]>)
```

## Testing

| Test | Description |
|------|-------------|
| TestAddDependency | Add dependency and verify stored |
| TestAddDependencyDedup | Same pair not duplicated |
| TestDirectDeps | Returns correct direct dependencies |
| TestDirectDependents | Returns correct direct dependents |
| TestImpact | BFS finds all downstream artifacts |
| TestImpactNoCycle | Handles circular dependencies without infinite loop |
| TestFormatImpact | Human-readable impact output |
| TestFormatImpactJSON | JSON output |
| E2E: TestArtifactImpactEmpty | No deps returns clean message |
| E2E: TestArtifactDepsEmpty | No deps shows "no dependencies" |
| E2E: TestArtifactImpactNotFound | Missing hash returns error |

# Phase 28: Artifact Lineage — Implementation Plan

**Date:** 2026-02-20
**Design:** `2026-02-20-phase28-artifact-lineage-design.md`
**Method:** Subagent-Driven Development

## Tasks

### Task 1: Lineage Core (Graph + Impact + Deps)

**Files:** `internal/artifact/lineage.go`, `internal/artifact/lineage_test.go`
**Tests (6):**
1. `TestAddDependency` — Add dependency, verify in deps list
2. `TestAddDependencyDedup` — Same (from, to) pair not duplicated
3. `TestDirectDeps` — Returns toHash list for given artifact
4. `TestDirectDependents` — Returns fromHash list for given artifact
5. `TestImpact` — BFS chain: A depends B, B depends C → Impact(C) finds A and B
6. `TestImpactNoCycle` — Circular deps (A→B→C→A) don't cause infinite loop

**Spec:**
- `Dependency` struct: FromHash, ToHash (json tags)
- `LineageGraph` struct: dir, store (*Store), deps ([]Dependency)
- `ImpactResult` struct: Root (*Artifact), Affected ([]*Artifact), Depth (int)
- `NewLineageGraph(dir, store)` loads deps.json (or empty if not exists)
- `AddDependency(from, to)` appends + dedup check
- `RemoveDependency(from, to)` removes matching pair
- `DirectDeps(hash)` returns toHash values where FromHash==hash
- `DirectDependents(hash)` returns fromHash values where ToHash==hash
- `Impact(hash)` BFS: find all who depend on hash, recursively. Use visited set.
- `Save()` writes deps.json

### Task 2: Format + CLI Commands

**Files:** `internal/artifact/lineage_format.go`, `internal/artifact/lineage_format_test.go`, update `cmd/apex/artifact.go`
**Tests (2):**
1. `TestFormatImpact` — Human-readable impact chain with levels
2. `TestFormatImpactJSON` — JSON indented output

**Spec:**
- `FormatImpact(result *ImpactResult) string` — Shows root + affected by BFS level
- `FormatImpactJSON(result *ImpactResult) string` — JSON indent
- Add `artifactImpactCmd` and `artifactDepsCmd` subcommands to existing `artifactCmd`
- `apex artifact impact <hash>` — loads lineage graph, runs Impact, formats
- `apex artifact deps <hash>` — loads lineage graph, shows DirectDeps

### Task 3: E2E Tests

**Files:** `e2e/lineage_test.go`
**Tests (3):**
1. `TestArtifactImpactEmpty` — No deps.json, shows "No downstream impact"
2. `TestArtifactDepsEmpty` — No deps, shows "No dependencies"
3. `TestArtifactImpactNotFound` — Missing artifact hash returns error

### Task 4: PROGRESS.md Update

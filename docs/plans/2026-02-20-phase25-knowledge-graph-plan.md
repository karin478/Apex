# Phase 25: Knowledge Graph — Implementation Plan

**Date:** 2026-02-20
**Design:** `2026-02-20-phase25-knowledge-graph-design.md`
**Method:** Subagent-Driven Development

## Tasks

### Task 1: KG Core (Graph + Entity + Relationship)

**Files:** `internal/kg/graph.go`, `internal/kg/graph_test.go`
**Tests (8):**
1. `TestAddEntity` — Add entity, verify fields
2. `TestAddEntityDedup` — Same canonical key returns existing entity
3. `TestAddRelationship` — Link entities, verify relationship stored
4. `TestQueryByName` — Partial name match returns matches
5. `TestQueryRelated` — BFS depth=2 returns connected entities
6. `TestQueryRelatedMaxNodes` — Stops at maxNodes limit
7. `TestRemoveEntity` — Removes entity and cascading relationships
8. `TestStats` — Correct counts by type

**Spec:**
- `Entity` with ID (UUID), Type (EntityType const), CanonicalName, Project, Namespace, CreatedAt, UpdatedAt
- `Relationship` with ID, FromID, ToID, RelType (RelType const), Evidence, CreatedAt
- `Graph` with in-memory maps + JSON persistence
- `New(dir)` loads existing graph.json or creates empty
- `AddEntity` deduplicates by (CanonicalName, Project, Namespace)
- `QueryRelated` uses BFS with depth limit and maxNodes cap
- `RemoveEntity` cascades to remove all relationships referencing the entity
- `Save()` persists to `{dir}/graph.json`

### Task 2: Format Functions

**Files:** `internal/kg/format.go`, `internal/kg/format_test.go`
**Tests (3):**
1. `TestFormatEntitiesTable` — Table output with type, name, project columns
2. `TestFormatQueryResult` — Entity detail + relationship list
3. `TestFormatJSON` — JSON-indented output

**Spec:**
- `FormatEntitiesTable(entities []*Entity) string` — Tabular format
- `FormatQueryResult(center *Entity, entities []*Entity, rels []*Relationship) string` — Center entity + related graph
- `FormatJSON(v interface{}) string` — Indented JSON

### Task 3: CLI Commands

**Files:** `cmd/apex/kg.go`, update `cmd/apex/main.go`
**Spec:**
- `apex kg list [--type TYPE] [--project PROJECT]` — Calls Graph.List, formats as table
- `apex kg query <name> [--depth N] [--format json]` — Calls QueryByName + QueryRelated
- `apex kg stats` — Calls Stats, displays counts
- KG dir: `~/.claude/kg/`
- Register `kgCmd` in rootCmd

### Task 4: E2E Tests

**Files:** `e2e/kg_test.go`
**Tests (3):**
1. `TestKGListEmpty` — Empty graph shows "No entities"
2. `TestKGQueryNotFound` — Query nonexistent shows "not found"
3. `TestKGStats` — Stats on empty shows zeros

### Task 5: PROGRESS.md Update

Update PROGRESS.md with Phase 25 completion info.

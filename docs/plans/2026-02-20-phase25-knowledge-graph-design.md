# Phase 25: Knowledge Graph — Design Document

**Date:** 2026-02-20
**Status:** Approved (recommendation credit 3/10)

## Overview

Implement a lightweight Knowledge Graph (`internal/kg`) for entity-relationship tracking. The KG stores typed entities as 4-tuples `(Type, CanonicalName, Project, Namespace)` with typed relationships and evidence references.

## Architecture

### Storage

JSON file persistence at `~/.claude/kg/graph.json`, loaded into memory maps for O(1) entity lookup and O(n) relationship traversal. Consistent with project's existing JSON-based storage pattern (memory, audit, manifest, artifact).

### Core Types

```go
type EntityType string

const (
    EntityFile        EntityType = "file"
    EntityFunction    EntityType = "function"
    EntityClass       EntityType = "class"
    EntityAPIEndpoint EntityType = "api_endpoint"
    EntityConfigKey   EntityType = "config_key"
    EntityService     EntityType = "service"
    EntityPackage     EntityType = "package"
    EntityDecision    EntityType = "decision"
    EntityPattern     EntityType = "pattern"
    EntityIncident    EntityType = "incident"
)

type RelType string

const (
    RelDependsOn  RelType = "depends_on"
    RelImports    RelType = "imports"
    RelCalls      RelType = "calls"
    RelContains   RelType = "contains"
    RelConfigures RelType = "configures"
    RelProduces   RelType = "produces"
    RelConsumes   RelType = "consumes"
    RelRelatesTo  RelType = "relates_to"
)

type Entity struct {
    ID            string     `json:"id"`
    Type          EntityType `json:"type"`
    CanonicalName string     `json:"canonical_name"`
    Project       string     `json:"project"`
    Namespace     string     `json:"namespace"`
    CreatedAt     time.Time  `json:"created_at"`
    UpdatedAt     time.Time  `json:"updated_at"`
}

type Relationship struct {
    ID        string    `json:"id"`
    FromID    string    `json:"from_id"`
    ToID      string    `json:"to_id"`
    RelType   RelType   `json:"rel_type"`
    Evidence  string    `json:"evidence"`
    CreatedAt time.Time `json:"created_at"`
}
```

### Graph Operations

```go
type Graph struct {
    dir      string
    entities map[string]*Entity       // by ID
    rels     map[string]*Relationship // by ID
    byName   map[string][]*Entity     // by canonical_name for lookup
}

func New(dir string) (*Graph, error)                                         // Load or create
func (g *Graph) AddEntity(e Entity) (*Entity, error)                         // Dedup by name+project+namespace
func (g *Graph) AddRelationship(fromID, toID string, rt RelType, evidence string) (*Relationship, error)
func (g *Graph) GetEntity(id string) *Entity
func (g *Graph) QueryByName(name string) []*Entity                           // Partial match
func (g *Graph) QueryRelated(entityID string, depth int, maxNodes int) ([]*Entity, []*Relationship)  // BFS traversal
func (g *Graph) RemoveEntity(id string) error                                // Cascades relationships
func (g *Graph) List(entityType EntityType, project string) []*Entity        // Filter by type/project
func (g *Graph) Stats() Stats
func (g *Graph) Save() error                                                 // Persist to JSON
```

### Query Depth Limits

Per architecture spec:
- Default depth: 2
- Max nodes returned: 200
- BFS traversal, stops at depth limit or max nodes

### CLI Commands

```
apex kg list [--type TYPE] [--project PROJECT]     List entities with optional filters
apex kg query <name> [--depth N] [--format json]   Query entity and related graph
apex kg stats                                       Show entity/relationship counts
```

### Stats Type

```go
type Stats struct {
    TotalEntities      int            `json:"total_entities"`
    TotalRelationships int            `json:"total_relationships"`
    EntitiesByType     map[string]int `json:"entities_by_type"`
    RelsByType         map[string]int `json:"relationships_by_type"`
}
```

## Testing

| Test | Description |
|------|-------------|
| TestAddEntity | Add entity and verify retrieval |
| TestAddEntityDedup | Same name+project+namespace returns existing |
| TestAddRelationship | Link two entities and verify |
| TestQueryByName | Partial name matching |
| TestQueryRelated | BFS depth traversal |
| TestQueryRelatedMaxNodes | Stops at max_nodes limit |
| TestRemoveEntity | Cascades relationship removal |
| TestStats | Verify counts by type |
| TestFormatEntitiesTable | Human-readable entity table |
| TestFormatQueryResult | Graph traversal output |
| TestFormatJSON | JSON output of query |

## Risk Mitigation

- **R10 KG Corruption**: Entity deduplication by canonical key, typed constants prevent invalid types
- **Depth limit**: BFS with maxNodes=200 default prevents explosion

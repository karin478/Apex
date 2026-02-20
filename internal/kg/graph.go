package kg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Entity types
// ---------------------------------------------------------------------------

// EntityType classifies what an entity represents in the knowledge graph.
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

// ---------------------------------------------------------------------------
// Relationship types
// ---------------------------------------------------------------------------

// RelType classifies what a relationship represents between two entities.
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

// ---------------------------------------------------------------------------
// Domain structs
// ---------------------------------------------------------------------------

// Entity is a node in the knowledge graph.
type Entity struct {
	ID            string     `json:"id"`
	Type          EntityType `json:"type"`
	CanonicalName string     `json:"canonical_name"`
	Project       string     `json:"project"`
	Namespace     string     `json:"namespace"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Relationship is a directed edge in the knowledge graph.
type Relationship struct {
	ID        string    `json:"id"`
	FromID    string    `json:"from_id"`
	ToID      string    `json:"to_id"`
	RelType   RelType   `json:"rel_type"`
	Evidence  string    `json:"evidence"`
	CreatedAt time.Time `json:"created_at"`
}

// Stats holds aggregate counts for the graph.
type Stats struct {
	TotalEntities      int            `json:"total_entities"`
	TotalRelationships int            `json:"total_relationships"`
	EntitiesByType     map[string]int `json:"entities_by_type"`
	RelsByType         map[string]int `json:"relationships_by_type"`
}

// ---------------------------------------------------------------------------
// Persistence envelope
// ---------------------------------------------------------------------------

type graphFile struct {
	Entities      []*Entity       `json:"entities"`
	Relationships []*Relationship `json:"relationships"`
}

// ---------------------------------------------------------------------------
// Graph
// ---------------------------------------------------------------------------

// Graph is an in-memory knowledge graph with JSON persistence.
type Graph struct {
	mu   sync.RWMutex
	dir  string
	ents map[string]*Entity       // id -> entity
	rels map[string]*Relationship // id -> relationship
}

// New loads an existing graph.json from dir or creates an empty graph.
func New(dir string) (*Graph, error) {
	g := &Graph{
		dir:  dir,
		ents: make(map[string]*Entity),
		rels: make(map[string]*Relationship),
	}
	if err := g.load(); err != nil {
		return nil, err
	}
	return g, nil
}

// ---------------------------------------------------------------------------
// AddEntity
// ---------------------------------------------------------------------------

// AddEntity adds an entity to the graph. If an entity with the same
// (CanonicalName, Project, Namespace) already exists, it is returned instead.
func (g *Graph) AddEntity(etype EntityType, canonicalName, project, namespace string) (*Entity, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Dedup by canonical key.
	for _, e := range g.ents {
		if e.CanonicalName == canonicalName && e.Project == project && e.Namespace == namespace {
			return e, nil
		}
	}

	now := time.Now().UTC()
	ent := &Entity{
		ID:            uuid.New().String(),
		Type:          etype,
		CanonicalName: canonicalName,
		Project:       project,
		Namespace:     namespace,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	g.ents[ent.ID] = ent
	return ent, nil
}

// ---------------------------------------------------------------------------
// GetEntity
// ---------------------------------------------------------------------------

// GetEntity returns the entity with the given ID, or nil if not found.
func (g *Graph) GetEntity(id string) *Entity {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.ents[id]
}

// ---------------------------------------------------------------------------
// AddRelationship
// ---------------------------------------------------------------------------

// AddRelationship creates a directed relationship between two entities.
// Both fromID and toID must reference existing entities.
func (g *Graph) AddRelationship(fromID, toID string, relType RelType, evidence string) (*Relationship, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.ents[fromID]; !ok {
		return nil, fmt.Errorf("kg: entity not found: %s", fromID)
	}
	if _, ok := g.ents[toID]; !ok {
		return nil, fmt.Errorf("kg: entity not found: %s", toID)
	}

	rel := &Relationship{
		ID:        uuid.New().String(),
		FromID:    fromID,
		ToID:      toID,
		RelType:   relType,
		Evidence:  evidence,
		CreatedAt: time.Now().UTC(),
	}
	g.rels[rel.ID] = rel
	return rel, nil
}

// ---------------------------------------------------------------------------
// QueryByName
// ---------------------------------------------------------------------------

// QueryByName returns all entities whose CanonicalName contains the given
// substring (case-insensitive).
func (g *Graph) QueryByName(name string) []*Entity {
	g.mu.RLock()
	defer g.mu.RUnlock()

	lower := strings.ToLower(name)
	var results []*Entity
	for _, e := range g.ents {
		if strings.Contains(strings.ToLower(e.CanonicalName), lower) {
			results = append(results, e)
		}
	}
	return results
}

// ---------------------------------------------------------------------------
// QueryRelated — BFS
// ---------------------------------------------------------------------------

// QueryRelated performs a BFS from the given entity up to depth hops,
// returning at most maxNodes connected entities (excluding the start node)
// along with the relationships traversed during the search.
// Defaults: depth=2, maxNodes=200 when zero values are passed.
func (g *Graph) QueryRelated(entityID string, depth, maxNodes int) ([]*Entity, []*Relationship) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if depth <= 0 {
		depth = 2
	}
	if maxNodes <= 0 {
		maxNodes = 200
	}

	visited := map[string]bool{entityID: true}
	visitedRels := map[string]bool{}
	queue := []string{entityID}
	var result []*Entity
	var rels []*Relationship

	for d := 0; d < depth && len(queue) > 0; d++ {
		var next []string
		for _, id := range queue {
			for _, r := range g.rels {
				var neighbour string
				if r.FromID == id {
					neighbour = r.ToID
				} else if r.ToID == id {
					neighbour = r.FromID
				} else {
					continue
				}
				if visited[neighbour] {
					continue
				}
				visited[neighbour] = true
				if !visitedRels[r.ID] {
					visitedRels[r.ID] = true
					rels = append(rels, r)
				}
				if e, ok := g.ents[neighbour]; ok {
					result = append(result, e)
					if len(result) >= maxNodes {
						return result, rels
					}
					next = append(next, neighbour)
				}
			}
		}
		queue = next
	}
	return result, rels
}

// ---------------------------------------------------------------------------
// RemoveEntity
// ---------------------------------------------------------------------------

// RemoveEntity removes the entity with the given ID and all relationships
// that reference it (cascading delete).
func (g *Graph) RemoveEntity(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.ents[id]; !ok {
		return fmt.Errorf("kg: entity not found: %s", id)
	}
	delete(g.ents, id)

	// Cascade: remove every relationship referencing this entity.
	for rid, r := range g.rels {
		if r.FromID == id || r.ToID == id {
			delete(g.rels, rid)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

// List returns entities optionally filtered by type and/or project.
// Pass empty string to skip a filter.
func (g *Graph) List(entityType EntityType, project string) []*Entity {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var results []*Entity
	for _, e := range g.ents {
		if entityType != "" && e.Type != entityType {
			continue
		}
		if project != "" && e.Project != project {
			continue
		}
		results = append(results, e)
	}
	return results
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

// Stats returns aggregate counts for the graph.
func (g *Graph) Stats() Stats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	s := Stats{
		TotalEntities:      len(g.ents),
		TotalRelationships: len(g.rels),
		EntitiesByType:     make(map[string]int),
		RelsByType:         make(map[string]int),
	}
	for _, e := range g.ents {
		s.EntitiesByType[string(e.Type)]++
	}
	for _, r := range g.rels {
		s.RelsByType[string(r.RelType)]++
	}
	return s
}

// ---------------------------------------------------------------------------
// Persistence
// ---------------------------------------------------------------------------

// Save persists the graph to {dir}/graph.json.
func (g *Graph) Save() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if err := os.MkdirAll(g.dir, 0o755); err != nil {
		return fmt.Errorf("kg: mkdir: %w", err)
	}

	gf := graphFile{
		Entities:      make([]*Entity, 0, len(g.ents)),
		Relationships: make([]*Relationship, 0, len(g.rels)),
	}
	for _, e := range g.ents {
		gf.Entities = append(gf.Entities, e)
	}
	for _, r := range g.rels {
		gf.Relationships = append(gf.Relationships, r)
	}

	data, err := json.MarshalIndent(gf, "", "  ")
	if err != nil {
		return fmt.Errorf("kg: marshal: %w", err)
	}
	return os.WriteFile(g.graphPath(), data, 0o644)
}

// load reads graph.json from disk if it exists.
func (g *Graph) load() error {
	data, err := os.ReadFile(g.graphPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("kg: read graph: %w", err)
	}

	var gf graphFile
	if err := json.Unmarshal(data, &gf); err != nil {
		return fmt.Errorf("kg: unmarshal graph: %w", err)
	}

	for _, e := range gf.Entities {
		g.ents[e.ID] = e
	}
	for _, r := range gf.Relationships {
		g.rels[r.ID] = r
	}
	return nil
}

func (g *Graph) graphPath() string {
	return filepath.Join(g.dir, "graph.json")
}

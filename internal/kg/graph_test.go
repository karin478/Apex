package kg

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testGraph creates a new Graph in a temporary directory.
func testGraph(t *testing.T) *Graph {
	t.Helper()
	g, err := New(t.TempDir())
	require.NoError(t, err)
	return g
}

// ---------------------------------------------------------------------------
// 1. TestAddEntity — Add entity, verify fields
// ---------------------------------------------------------------------------

func TestAddEntity(t *testing.T) {
	g := testGraph(t)

	ent, err := g.AddEntity(EntityFunction, "handleRequest", "apex", "internal/server")
	require.NoError(t, err)

	assert.NotEmpty(t, ent.ID)
	assert.Equal(t, EntityFunction, ent.Type)
	assert.Equal(t, "handleRequest", ent.CanonicalName)
	assert.Equal(t, "apex", ent.Project)
	assert.Equal(t, "internal/server", ent.Namespace)
	assert.False(t, ent.CreatedAt.IsZero())
	assert.False(t, ent.UpdatedAt.IsZero())
}

// ---------------------------------------------------------------------------
// 2. TestAddEntityDedup — Same canonical key returns existing entity
// ---------------------------------------------------------------------------

func TestAddEntityDedup(t *testing.T) {
	g := testGraph(t)

	e1, err := g.AddEntity(EntityFile, "main.go", "apex", "cmd")
	require.NoError(t, err)

	e2, err := g.AddEntity(EntityFile, "main.go", "apex", "cmd")
	require.NoError(t, err)

	// Same entity returned — identical ID.
	assert.Equal(t, e1.ID, e2.ID)

	// Only one entity in graph.
	stats := g.Stats()
	assert.Equal(t, 1, stats.TotalEntities)
}

// ---------------------------------------------------------------------------
// 3. TestAddRelationship — Link entities, verify relationship stored
// ---------------------------------------------------------------------------

func TestAddRelationship(t *testing.T) {
	g := testGraph(t)

	e1, err := g.AddEntity(EntityService, "api-gateway", "apex", "")
	require.NoError(t, err)
	e2, err := g.AddEntity(EntityService, "auth-service", "apex", "")
	require.NoError(t, err)

	rel, err := g.AddRelationship(e1.ID, e2.ID, RelDependsOn, "gateway routes to auth")
	require.NoError(t, err)

	assert.NotEmpty(t, rel.ID)
	assert.Equal(t, e1.ID, rel.FromID)
	assert.Equal(t, e2.ID, rel.ToID)
	assert.Equal(t, RelDependsOn, rel.RelType)
	assert.Equal(t, "gateway routes to auth", rel.Evidence)
	assert.False(t, rel.CreatedAt.IsZero())

	// Verify stats reflect the relationship.
	stats := g.Stats()
	assert.Equal(t, 1, stats.TotalRelationships)

	// Adding a relationship with a missing entity should fail.
	_, err = g.AddRelationship("nonexistent", e2.ID, RelCalls, "")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// 4. TestQueryByName — Partial name match returns matches
// ---------------------------------------------------------------------------

func TestQueryByName(t *testing.T) {
	g := testGraph(t)

	_, err := g.AddEntity(EntityFunction, "HandleRequest", "apex", "server")
	require.NoError(t, err)
	_, err = g.AddEntity(EntityFunction, "handleResponse", "apex", "server")
	require.NoError(t, err)
	_, err = g.AddEntity(EntityFunction, "parseConfig", "apex", "config")
	require.NoError(t, err)

	// Case-insensitive partial match on "handle".
	results := g.QueryByName("handle")
	assert.Len(t, results, 2)

	// No match.
	results = g.QueryByName("zzzzz")
	assert.Empty(t, results)
}

// ---------------------------------------------------------------------------
// 5. TestQueryRelated — BFS depth=2 returns connected entities
// ---------------------------------------------------------------------------

func TestQueryRelated(t *testing.T) {
	g := testGraph(t)

	// Build chain: A -> B -> C -> D
	a, _ := g.AddEntity(EntityService, "A", "p", "")
	b, _ := g.AddEntity(EntityService, "B", "p", "")
	c, _ := g.AddEntity(EntityService, "C", "p", "")
	d, _ := g.AddEntity(EntityService, "D", "p", "")

	_, err := g.AddRelationship(a.ID, b.ID, RelDependsOn, "")
	require.NoError(t, err)
	_, err = g.AddRelationship(b.ID, c.ID, RelDependsOn, "")
	require.NoError(t, err)
	_, err = g.AddRelationship(c.ID, d.ID, RelDependsOn, "")
	require.NoError(t, err)

	// depth=2 from A should reach B (depth 1) and C (depth 2), but NOT D.
	related, rels := g.QueryRelated(a.ID, 2, 0)
	ids := map[string]bool{}
	for _, e := range related {
		ids[e.CanonicalName] = true
	}
	assert.True(t, ids["B"], "B should be reachable at depth 1")
	assert.True(t, ids["C"], "C should be reachable at depth 2")
	assert.False(t, ids["D"], "D should NOT be reachable at depth 2")
	assert.Len(t, related, 2)

	// Verify relationships are returned for the traversed edges.
	assert.Len(t, rels, 2, "should return 2 relationships (A->B and B->C)")
	relFromTo := map[string]string{}
	for _, r := range rels {
		relFromTo[r.FromID] = r.ToID
	}
	assert.Equal(t, b.ID, relFromTo[a.ID], "should contain A->B relationship")
	assert.Equal(t, c.ID, relFromTo[b.ID], "should contain B->C relationship")
}

// ---------------------------------------------------------------------------
// 6. TestQueryRelatedMaxNodes — Stops at maxNodes limit
// ---------------------------------------------------------------------------

func TestQueryRelatedMaxNodes(t *testing.T) {
	g := testGraph(t)

	// Hub entity connected to many leaves.
	hub, _ := g.AddEntity(EntityService, "hub", "p", "")
	for i := 0; i < 10; i++ {
		leaf, err := g.AddEntity(EntityService, fmt.Sprintf("leaf-%d", i), "p", fmt.Sprintf("ns-%d", i))
		require.NoError(t, err)
		_, err = g.AddRelationship(hub.ID, leaf.ID, RelContains, "")
		require.NoError(t, err)
	}

	// maxNodes=3 should return at most 3 entities.
	related, rels := g.QueryRelated(hub.ID, 2, 3)
	assert.Len(t, related, 3)
	assert.Len(t, rels, 3, "should return one relationship per discovered node")
}

// ---------------------------------------------------------------------------
// 7. TestRemoveEntity — Removes entity and cascading relationships
// ---------------------------------------------------------------------------

func TestRemoveEntity(t *testing.T) {
	g := testGraph(t)

	a, _ := g.AddEntity(EntityFile, "a.go", "apex", "pkg")
	b, _ := g.AddEntity(EntityFile, "b.go", "apex", "pkg")
	c, _ := g.AddEntity(EntityFile, "c.go", "apex", "pkg")

	_, err := g.AddRelationship(a.ID, b.ID, RelImports, "")
	require.NoError(t, err)
	_, err = g.AddRelationship(b.ID, c.ID, RelImports, "")
	require.NoError(t, err)

	// Remove B — should cascade both relationships.
	err = g.RemoveEntity(b.ID)
	require.NoError(t, err)

	stats := g.Stats()
	assert.Equal(t, 2, stats.TotalEntities)
	assert.Equal(t, 0, stats.TotalRelationships)

	// Removing again should error.
	err = g.RemoveEntity(b.ID)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// 8. TestStats — Correct counts by type
// ---------------------------------------------------------------------------

func TestStats(t *testing.T) {
	g := testGraph(t)

	f1, _ := g.AddEntity(EntityFile, "main.go", "apex", "cmd")
	f2, _ := g.AddEntity(EntityFile, "util.go", "apex", "pkg")
	svc, _ := g.AddEntity(EntityService, "api", "apex", "")

	_, err := g.AddRelationship(f1.ID, f2.ID, RelImports, "")
	require.NoError(t, err)
	_, err = g.AddRelationship(svc.ID, f1.ID, RelContains, "")
	require.NoError(t, err)

	stats := g.Stats()
	assert.Equal(t, 3, stats.TotalEntities)
	assert.Equal(t, 2, stats.TotalRelationships)

	// Entities by type.
	assert.Equal(t, 2, stats.EntitiesByType["file"])
	assert.Equal(t, 1, stats.EntitiesByType["service"])

	// Relationships by type.
	assert.Equal(t, 1, stats.RelsByType["imports"])
	assert.Equal(t, 1, stats.RelsByType["contains"])
}

// ---------------------------------------------------------------------------
// Persistence round-trip
// ---------------------------------------------------------------------------

func TestSaveAndReload(t *testing.T) {
	dir := t.TempDir()

	g, err := New(dir)
	require.NoError(t, err)

	e1, _ := g.AddEntity(EntityFile, "main.go", "apex", "cmd")
	e2, _ := g.AddEntity(EntityFunction, "Run", "apex", "cmd")
	_, err = g.AddRelationship(e1.ID, e2.ID, RelContains, "main contains Run")
	require.NoError(t, err)

	require.NoError(t, g.Save())

	// Verify file exists.
	_, err = os.Stat(filepath.Join(dir, "graph.json"))
	require.NoError(t, err)

	// Reload from disk.
	g2, err := New(dir)
	require.NoError(t, err)

	stats := g2.Stats()
	assert.Equal(t, 2, stats.TotalEntities)
	assert.Equal(t, 1, stats.TotalRelationships)

	// Verify the entity data survived round-trip.
	results := g2.QueryByName("main.go")
	require.Len(t, results, 1)
	assert.Equal(t, e1.ID, results[0].ID)
}

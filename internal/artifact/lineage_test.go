package artifact

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLineageGraph(t *testing.T) *LineageGraph {
	t.Helper()
	lg, err := NewLineageGraph(t.TempDir())
	require.NoError(t, err)
	return lg
}

func TestAddDependency(t *testing.T) {
	lg := testLineageGraph(t)

	err := lg.AddDependency("aaa", "bbb")
	require.NoError(t, err)

	assert.Len(t, lg.deps, 1)
	assert.Equal(t, "aaa", lg.deps[0].FromHash)
	assert.Equal(t, "bbb", lg.deps[0].ToHash)
}

func TestAddDependencyDedup(t *testing.T) {
	lg := testLineageGraph(t)

	err := lg.AddDependency("aaa", "bbb")
	require.NoError(t, err)

	// Adding the same pair again should be a no-op.
	err = lg.AddDependency("aaa", "bbb")
	require.NoError(t, err)

	assert.Len(t, lg.deps, 1)
}

func TestDirectDeps(t *testing.T) {
	lg := testLineageGraph(t)

	_ = lg.AddDependency("aaa", "bbb")
	_ = lg.AddDependency("aaa", "ccc")
	_ = lg.AddDependency("ddd", "bbb")

	deps := lg.DirectDeps("aaa")
	assert.ElementsMatch(t, []string{"bbb", "ccc"}, deps)

	// ddd depends on bbb only.
	deps = lg.DirectDeps("ddd")
	assert.Equal(t, []string{"bbb"}, deps)

	// Unknown hash returns nil.
	deps = lg.DirectDeps("zzz")
	assert.Nil(t, deps)
}

func TestDirectDependents(t *testing.T) {
	lg := testLineageGraph(t)

	_ = lg.AddDependency("aaa", "ccc")
	_ = lg.AddDependency("bbb", "ccc")
	_ = lg.AddDependency("ddd", "aaa")

	dependents := lg.DirectDependents("ccc")
	assert.ElementsMatch(t, []string{"aaa", "bbb"}, dependents)

	dependents = lg.DirectDependents("aaa")
	assert.Equal(t, []string{"ddd"}, dependents)

	// Unknown hash returns nil.
	dependents = lg.DirectDependents("zzz")
	assert.Nil(t, dependents)
}

func TestImpact(t *testing.T) {
	lg := testLineageGraph(t)

	// A depends on C, B depends on C.
	_ = lg.AddDependency("A", "C")
	_ = lg.AddDependency("B", "C")

	result := lg.Impact("C")
	assert.Equal(t, "C", result.RootHash)
	assert.ElementsMatch(t, []string{"A", "B"}, result.Affected)
	assert.Equal(t, 1, result.Depth)

	// Now add D depends on A → Impact(C) should find A, B, D.
	_ = lg.AddDependency("D", "A")

	result = lg.Impact("C")
	assert.Equal(t, "C", result.RootHash)
	assert.ElementsMatch(t, []string{"A", "B", "D"}, result.Affected)
	assert.Equal(t, 2, result.Depth)
}

func TestImpactNoCycle(t *testing.T) {
	lg := testLineageGraph(t)

	// Circular: A→B→C→A (A depends on B, B depends on C, C depends on A).
	_ = lg.AddDependency("A", "B")
	_ = lg.AddDependency("B", "C")
	_ = lg.AddDependency("C", "A")

	// Impact(A) should terminate and find B, C (they transitively depend on A).
	// C depends on A (direct), B depends on C (transitive).
	result := lg.Impact("A")
	assert.Equal(t, "A", result.RootHash)
	assert.ElementsMatch(t, []string{"C", "B"}, result.Affected)
	// Should not hang or panic — the visited set prevents infinite loops.
}

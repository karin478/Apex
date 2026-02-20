package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestArtifactImpactEmpty verifies that when no deps.json exists,
// "apex artifact impact" reports no downstream impact and exits 0.
func TestArtifactImpactEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("artifact", "impact", "abc123456789def")

	assert.Equal(t, 0, exitCode, "apex artifact impact should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "No downstream impact", "empty lineage graph should report no downstream impact")
}

// TestArtifactDepsEmpty verifies that when no deps.json exists,
// "apex artifact deps" reports no dependencies and exits 0.
func TestArtifactDepsEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("artifact", "deps", "abc123456789def")

	assert.Equal(t, 0, exitCode, "apex artifact deps should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "No dependencies", "empty lineage graph should report no dependencies")
}

// TestArtifactImpactNotFound verifies that a missing/nonexistent artifact hash
// still works gracefully — the lineage graph does not validate that hashes
// exist in the artifact store, so it simply reports no downstream impact.
func TestArtifactImpactNotFound(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, exitCode := env.runApex("artifact", "impact", "nonexistent")

	assert.Equal(t, 0, exitCode, "apex artifact impact on missing hash should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "No downstream impact", "nonexistent hash should report no downstream impact")
}

package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewHappyPath(t *testing.T) {
	env := newTestEnv(t)

	// The mock returns '{"result":"mock ok"}' for each step.
	// The judge output won't be valid JSON verdict, so it falls back to "revise".
	stdout, stderr, code := env.runApex("review", "Use Redis for caching")
	require.Equal(t, 0, code, "apex review should succeed; stdout=%s stderr=%s", stdout, stderr)

	assert.Contains(t, stdout, "Adversarial Review")
	assert.Contains(t, stdout, "Verdict:")
	assert.Contains(t, stdout, "Full report:")

	// Review file should be saved
	reviewsDir := filepath.Join(env.Home, ".apex", "reviews")
	entries, err := os.ReadDir(reviewsDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.True(t, strings.HasSuffix(entries[0].Name(), ".json"))
}

func TestReviewNoArgs(t *testing.T) {
	env := newTestEnv(t)
	_, stderr, code := env.runApex("review")
	assert.NotEqual(t, 0, code)
	assert.Contains(t, stderr, "requires at least 1 arg")
}

func TestReviewCreatesAuditEntry(t *testing.T) {
	env := newTestEnv(t)

	env.runApex("review", "Test proposal for audit")

	// Check audit directory has a file for today
	auditDir := env.auditDir()
	entries, err := os.ReadDir(auditDir)
	require.NoError(t, err)
	assert.Greater(t, len(entries), 0, "audit directory should have at least one file")
}

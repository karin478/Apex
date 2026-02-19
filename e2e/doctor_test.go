package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// containsAny returns true if s contains any of the given substrings.
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// TestDoctorHealthy runs a task to populate audit entries, then verifies
// that "apex doctor" reports a healthy hash chain (exit 0, stdout contains "OK").
func TestDoctorHealthy(t *testing.T) {
	env := newTestEnv(t)

	// First, run a task so that audit entries are created.
	stdout, stderr, code := env.runApex("run", "say hello")
	require.Equal(t, 0, code, "apex run should succeed; stdout=%s stderr=%s", stdout, stderr)

	// Verify audit files were created.
	files, err := filepath.Glob(filepath.Join(env.auditDir(), "*.jsonl"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "audit files should exist after a run")

	// Now run doctor.
	stdout, stderr, code = env.runApex("doctor")
	assert.Equal(t, 0, code, "apex doctor should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "OK", "doctor should report OK for a healthy chain")
}

// TestDoctorNoAudit removes the audit directory before running doctor,
// verifying it handles the missing directory gracefully (exit 0, stdout
// contains "Apex Doctor").
func TestDoctorNoAudit(t *testing.T) {
	env := newTestEnv(t)

	// Remove the audit directory entirely.
	err := os.RemoveAll(env.auditDir())
	require.NoError(t, err)

	// Run doctor.
	stdout, stderr, code := env.runApex("doctor")
	assert.Equal(t, 0, code, "apex doctor should exit 0 even without audit dir; stderr=%s", stderr)
	assert.Contains(t, stdout, "Apex Doctor", "doctor should print its header")
}

// TestDoctorCorruptedChain runs a task to create audit entries, then corrupts
// the audit file by prepending "CORRUPTED" to its content. Doctor should detect
// the corruption (exit 0, stdout contains "BROKEN" or "ERROR").
func TestDoctorCorruptedChain(t *testing.T) {
	env := newTestEnv(t)

	// Run a task to generate audit entries.
	stdout, stderr, code := env.runApex("run", "say hello")
	require.Equal(t, 0, code, "apex run should succeed; stdout=%s stderr=%s", stdout, stderr)

	// Find audit files.
	files, err := filepath.Glob(filepath.Join(env.auditDir(), "*.jsonl"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "audit files should exist after a run")

	// Corrupt the first audit file by prepending "CORRUPTED" to its content.
	auditFile := files[0]
	original, err := os.ReadFile(auditFile)
	require.NoError(t, err)
	corrupted := append([]byte("CORRUPTED"), original...)
	err = os.WriteFile(auditFile, corrupted, 0644)
	require.NoError(t, err)

	// Run doctor — should detect corruption.
	stdout, stderr, code = env.runApex("doctor")
	assert.Equal(t, 0, code, "apex doctor should exit 0 even with corruption; stderr=%s", stderr)
	assert.True(t, containsAny(stdout, "BROKEN", "ERROR"),
		"doctor should report BROKEN or ERROR for corrupted chain; got stdout=%s", stdout)
}

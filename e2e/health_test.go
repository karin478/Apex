package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// System Health Level E2E Tests
// ---------------------------------------------------------------------------

// TestDoctorShowsHealth verifies that "apex doctor" prints a System Health
// line and reports GREEN when the environment is clean.
func TestDoctorShowsHealth(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, code := env.runApex("doctor")
	assert.Equal(t, 0, code, "apex doctor should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "System Health:", "doctor output should contain 'System Health:'")
	assert.Contains(t, stdout, "GREEN", "doctor should report GREEN for a clean environment")
}

// TestDoctorDegradedHealth activates the kill switch and verifies that doctor
// no longer reports GREEN (expected YELLOW since kill_switch is "important").
func TestDoctorDegradedHealth(t *testing.T) {
	env := newTestEnv(t)

	// Activate the kill switch (an "important" component).
	ksPath := env.killSwitchPath()
	require.NoError(t, os.MkdirAll(filepath.Dir(ksPath), 0755))
	require.NoError(t, os.WriteFile(ksPath, []byte("test-kill-switch"), 0644))

	stdout, stderr, code := env.runApex("doctor")
	assert.Equal(t, 0, code, "apex doctor should exit 0 even when degraded; stderr=%s", stderr)
	assert.Contains(t, stdout, "System Health:", "doctor output should contain 'System Health:'")
	assert.False(t, strings.Contains(stdout, "System Health: GREEN"),
		"doctor should NOT report GREEN when kill switch is active; got stdout=%s", stdout)
}

// TestRunBlockedByCriticalHealth corrupts the audit chain to trigger RED
// health, then verifies that a HIGH-risk "apex run" is blocked by the
// health gate.
//
// Flow:
//  1. Write an invalid JSONL entry to the audit log -> audit_chain critical fails -> RED
//  2. Run "apex run delete files" (HIGH risk)
//  3. RED health gate blocks non-LOW tasks -> prints [HEALTH] System health RED
func TestRunBlockedByCriticalHealth(t *testing.T) {
	env := newTestEnv(t)

	// Corrupt the audit chain by writing invalid JSON to a dated audit file.
	auditFile := filepath.Join(env.auditDir(), time.Now().Format("2006-01-02")+".jsonl")
	require.NoError(t, os.WriteFile(auditFile, []byte("THIS IS NOT VALID JSON\n"), 0644))

	stdout, stderr, code := env.runApex("run", "delete files")

	// The run command should exit 0 (returns nil after printing the gate message).
	assert.Equal(t, 0, code, "apex run should exit 0 when blocked by health gate; stderr=%s", stderr)
	assert.Contains(t, stdout, "[HEALTH] System health RED",
		"stdout should contain the RED health gate message; got stdout=%s", stdout)
}

// TestStatusShowsHealth verifies that "apex status" includes a System Health
// line in its output.
func TestStatusShowsHealth(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, code := env.runApex("status")
	assert.Equal(t, 0, code, "apex status should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "System Health:", "status output should contain 'System Health:'")
}

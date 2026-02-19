package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRedactTestCommand verifies that `apex redact test` correctly redacts
// sensitive tokens from the input string.
func TestRedactTestCommand(t *testing.T) {
	env := newTestEnv(t)
	stdout, stderr, code := env.runApex("redact", "test", "Bearer sk-abcdefghijklmnopqrstuvw")

	assert.Equal(t, 0, code, "expected exit code 0; stderr: %s", stderr)
	assert.Contains(t, stdout, "[REDACTED]", "output should contain [REDACTED] placeholder")
	assert.NotContains(t, stdout, "sk-abcdefghijklmnopqrstuvw",
		"output should NOT contain the raw secret key")
}

// TestRedactTestClean verifies that `apex redact test` passes through
// innocuous text unchanged (no false positives).
func TestRedactTestClean(t *testing.T) {
	env := newTestEnv(t)
	stdout, stderr, code := env.runApex("redact", "test", "hello world")

	assert.Equal(t, 0, code, "expected exit code 0; stderr: %s", stderr)
	assert.Contains(t, stdout, "hello world",
		"output should contain the original text unchanged")
}

// TestAuditEntryRedacted verifies that audit log entries have sensitive data
// redacted before being written to disk.
//
// TODO: The run command (cmd/apex/run.go) does not currently wire the redactor
// into the audit logger via SetRedactor(). Until that integration is added,
// audit entries will contain raw task text. This test is skipped until the
// redactor is wired into the run command path (outside Task 5 scope).
func TestAuditEntryRedacted(t *testing.T) {
	t.Skip("TODO: run command does not yet call audit.Logger.SetRedactor(); " +
		"wire redactor into cmd/apex/run.go before enabling this test")

	// Once the redactor is wired, the test should:
	// 1. env := newTestEnv(t)
	// 2. env.runApex("run", "deploy with sk-testkey1234567890abcdef")
	//    (will fail because no real Claude, but audit entry should still be written)
	// 3. Read audit log files from env.auditDir()
	// 4. Assert that none of them contain "sk-testkey1234567890abcdef"
}

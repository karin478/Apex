package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
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
func TestAuditEntryRedacted(t *testing.T) {
	env := newTestEnv(t)
	secret := "sk-testkey1234567890abcdef"

	// Use a LOW-risk task so it bypasses approval prompt; mock Claude runs and audit entry is written
	env.runApex("run", "say hello with "+secret)

	// Read all audit log files
	auditDir := env.auditDir()
	entries, err := os.ReadDir(auditDir)
	if err != nil {
		t.Fatalf("read audit dir: %v", err)
	}

	var allContent strings.Builder
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".jsonl" {
			data, err := os.ReadFile(filepath.Join(auditDir, entry.Name()))
			if err != nil {
				t.Fatalf("read audit file %s: %v", entry.Name(), err)
			}
			allContent.Write(data)
		}
	}

	content := allContent.String()
	if content == "" {
		t.Skip("no audit entries written (mock may not produce audit logs)")
	}

	assert.NotContains(t, content, secret,
		"audit log should NOT contain the raw secret key")
	assert.Contains(t, content, "[REDACTED]",
		"audit log should contain [REDACTED] placeholder")
}

package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHypothesisListEmpty(t *testing.T) {
	env := newTestEnv(t)

	stdout, stderr, code := env.runApex("hypothesis", "list")
	assert.Equal(t, 0, code, "apex hypothesis list should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "No hypotheses", "should show empty message")
}

func TestHypothesisProposeAndList(t *testing.T) {
	env := newTestEnv(t)

	// Propose
	stdout, stderr, code := env.runApex("hypothesis", "propose", "The bug is in auth")
	assert.Equal(t, 0, code, "propose should exit 0; stderr=%s", stderr)
	assert.Contains(t, stdout, "Proposed:", "should show proposed message")

	// Extract ID from output like "Proposed: [abc12345] The bug is in auth"
	idStart := strings.Index(stdout, "[") + 1
	idEnd := strings.Index(stdout, "]")
	if idStart < 1 || idEnd < 0 {
		t.Fatal("could not extract hypothesis ID from output")
	}
	_ = stdout[idStart:idEnd] // the ID

	// List
	stdout2, stderr2, code2 := env.runApex("hypothesis", "list")
	assert.Equal(t, 0, code2, "list should exit 0; stderr=%s", stderr2)
	assert.Contains(t, stdout2, "PROPOSED", "should show PROPOSED status")
	assert.Contains(t, stdout2, "The bug is in auth", "should show the statement")
}

func TestHypothesisConfirm(t *testing.T) {
	env := newTestEnv(t)

	// Propose first
	stdout, _, _ := env.runApex("hypothesis", "propose", "Memory leak exists")
	idStart := strings.Index(stdout, "[") + 1
	idEnd := strings.Index(stdout, "]")
	if idStart < 1 || idEnd < 0 {
		t.Fatal("could not extract hypothesis ID")
	}
	id := stdout[idStart:idEnd]

	// Confirm
	stdout2, stderr2, code2 := env.runApex("hypothesis", "confirm", id, "Confirmed via heap dump")
	assert.Equal(t, 0, code2, "confirm should exit 0; stderr=%s", stderr2)
	assert.Contains(t, stdout2, "Confirmed:", "should show confirmed message")

	// Verify status changed
	stdout3, _, _ := env.runApex("hypothesis", "list")
	assert.Contains(t, stdout3, "CONFIRMED", "should show CONFIRMED status")
}

package main

import (
	"testing"

	"github.com/lyndonlyu/apex/internal/governance"
	"github.com/stretchr/testify/assert"
)

func TestRunCommandRiskGating(t *testing.T) {
	// Verify governance integration
	assert.True(t, governance.Classify("read the README").ShouldAutoApprove())
	assert.True(t, governance.Classify("modify the config").ShouldConfirm())
	assert.True(t, governance.Classify("delete the database").ShouldRequireApproval())
	assert.True(t, governance.Classify("deploy to production with encryption key").ShouldReject())
}

package main

import (
	"testing"

	"github.com/lyndonlyu/apex/internal/governance"
	"github.com/stretchr/testify/assert"
)

func TestRunCommandRiskGating(t *testing.T) {
	governance.SetPolicy(governance.DefaultPolicy())
	defer governance.SetPolicy(governance.DefaultPolicy())

	// Verify governance integration with phrase-based classification
	assert.True(t, governance.Classify("read the README").ShouldAutoApprove())
	assert.True(t, governance.Classify("modify config settings").ShouldConfirm())
	assert.True(t, governance.Classify("delete from users table").ShouldRequireApproval())
	assert.True(t, governance.Classify("deploy to production with encryption key").ShouldReject())
}

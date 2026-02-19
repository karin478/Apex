package health

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{GREEN, "GREEN"},
		{YELLOW, "YELLOW"},
		{RED, "RED"},
		{CRITICAL, "CRITICAL"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.expected, tc.level.String(), "Level %d should stringify correctly", tc.level)
	}
}

func TestDetermineAllHealthy(t *testing.T) {
	components := []ComponentStatus{
		{Name: "audit_chain", Category: "critical", Healthy: true, Detail: "ok"},
		{Name: "sandbox_available", Category: "important", Healthy: true, Detail: "ok"},
		{Name: "metrics", Category: "optional", Healthy: true, Detail: "ok"},
	}
	assert.Equal(t, GREEN, Determine(components))
}

func TestDetermineOneImportantFailed(t *testing.T) {
	components := []ComponentStatus{
		{Name: "audit_chain", Category: "critical", Healthy: true, Detail: "ok"},
		{Name: "sandbox_available", Category: "important", Healthy: false, Detail: "sandbox unavailable"},
		{Name: "metrics", Category: "optional", Healthy: true, Detail: "ok"},
	}
	assert.Equal(t, YELLOW, Determine(components))
}

func TestDetermineTwoImportantFailed(t *testing.T) {
	components := []ComponentStatus{
		{Name: "audit_chain", Category: "critical", Healthy: true, Detail: "ok"},
		{Name: "sandbox_available", Category: "important", Healthy: false, Detail: "sandbox unavailable"},
		{Name: "rate_limiter", Category: "important", Healthy: false, Detail: "rate limiter down"},
		{Name: "metrics", Category: "optional", Healthy: true, Detail: "ok"},
	}
	assert.Equal(t, RED, Determine(components))
}

func TestDetermineOneCriticalFailed(t *testing.T) {
	components := []ComponentStatus{
		{Name: "audit_chain", Category: "critical", Healthy: false, Detail: "chain broken"},
		{Name: "sandbox_available", Category: "important", Healthy: true, Detail: "ok"},
		{Name: "metrics", Category: "optional", Healthy: true, Detail: "ok"},
	}
	assert.Equal(t, RED, Determine(components))
}

func TestDetermineTwoCriticalFailed(t *testing.T) {
	components := []ComponentStatus{
		{Name: "audit_chain", Category: "critical", Healthy: false, Detail: "chain broken"},
		{Name: "governance_engine", Category: "critical", Healthy: false, Detail: "engine down"},
		{Name: "sandbox_available", Category: "important", Healthy: true, Detail: "ok"},
	}
	assert.Equal(t, CRITICAL, Determine(components))
}

func TestDetermineMixedFailures(t *testing.T) {
	components := []ComponentStatus{
		{Name: "audit_chain", Category: "critical", Healthy: false, Detail: "chain broken"},
		{Name: "sandbox_available", Category: "important", Healthy: false, Detail: "sandbox unavailable"},
		{Name: "metrics", Category: "optional", Healthy: true, Detail: "ok"},
	}
	// 1 critical + 1 important → RED (critical takes precedence, not CRITICAL since only 1 critical)
	assert.Equal(t, RED, Determine(components))
}

func TestDetermineOptionalIgnored(t *testing.T) {
	components := []ComponentStatus{
		{Name: "audit_chain", Category: "critical", Healthy: true, Detail: "ok"},
		{Name: "sandbox_available", Category: "important", Healthy: true, Detail: "ok"},
		{Name: "metrics", Category: "optional", Healthy: false, Detail: "metrics down"},
		{Name: "logging", Category: "optional", Healthy: false, Detail: "logging down"},
		{Name: "tracing", Category: "optional", Healthy: false, Detail: "tracing down"},
	}
	// Optional failures should not affect the level
	assert.Equal(t, GREEN, Determine(components))
}

func TestNewReport(t *testing.T) {
	components := []ComponentStatus{
		{Name: "audit_chain", Category: "critical", Healthy: false, Detail: "chain broken"},
		{Name: "sandbox_available", Category: "important", Healthy: true, Detail: "ok"},
	}
	report := NewReport(components)
	assert.NotNil(t, report)
	assert.Equal(t, Determine(components), report.Level)
	assert.Equal(t, components, report.Components)
}

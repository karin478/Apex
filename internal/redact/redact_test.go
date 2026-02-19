package redact

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactBearerToken(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	r := New(cfg)

	input := "Authorization: Bearer sk-abc123def456ghi789jkl"
	result := r.Redact(input)

	assert.NotContains(t, result, "sk-abc123def456ghi789jkl")
	assert.Contains(t, result, "[REDACTED]")
}

func TestRedactGitHubPAT(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	r := New(cfg)

	input := "ghp_aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789"
	result := r.Redact(input)

	assert.Equal(t, "[REDACTED]", result)
}

func TestRedactOpenAIKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	r := New(cfg)

	input := "sk-proj-abcdef1234567890abcdef"
	result := r.Redact(input)

	assert.Equal(t, "[REDACTED]", result)
}

func TestRedactAWSAccessKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	r := New(cfg)

	input := "AKIAIOSFODNN7EXAMPLE"
	result := r.Redact(input)

	assert.Equal(t, "[REDACTED]", result)
}

func TestRedactSlackToken(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	r := New(cfg)

	input := "xoxb-1234-5678-abcdefgh"
	result := r.Redact(input)

	assert.Equal(t, "[REDACTED]", result)
}

func TestRedactStructuredSecret(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	r := New(cfg)

	input := "DB_PASSWORD=hunter2"
	result := r.Redact(input)

	assert.Equal(t, "DB_PASSWORD=[REDACTED]", result)
}

func TestRedactPrivateIP(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.RedactIPs = "private_only"
	r := New(cfg)

	input := "connecting to 192.168.1.100"
	result := r.Redact(input)

	assert.NotContains(t, result, "192.168.1.100")
	assert.Contains(t, result, "[REDACTED]")
}

func TestRedactPublicIPSkipped(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.RedactIPs = "private_only"
	r := New(cfg)

	input := "connecting to 8.8.8.8"
	result := r.Redact(input)

	assert.Equal(t, "connecting to 8.8.8.8", result)
}

func TestRedactAllIPs(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.RedactIPs = "all"
	r := New(cfg)

	input := "private 192.168.1.100 and public 8.8.8.8"
	result := r.Redact(input)

	assert.NotContains(t, result, "192.168.1.100")
	assert.NotContains(t, result, "8.8.8.8")
}

func TestRedactIPNone(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.RedactIPs = "none"
	r := New(cfg)

	input := "connecting to 192.168.1.100 and 8.8.8.8"
	result := r.Redact(input)

	assert.Equal(t, "connecting to 192.168.1.100 and 8.8.8.8", result)
}

func TestRedactCustomPattern(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.CustomPatterns = []string{`CUSTOM-\d{4}-[A-Z]+`}
	r := New(cfg)

	input := "id is CUSTOM-1234-ABCD here"
	result := r.Redact(input)

	assert.Equal(t, "id is [REDACTED] here", result)
}

func TestRedactMultipleSecrets(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.RedactIPs = "all"
	r := New(cfg)

	input := "key=ghp_aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789 host=10.0.0.1 token=xoxb-111-222-abc"
	result := r.Redact(input)

	assert.NotContains(t, result, "ghp_aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789")
	assert.NotContains(t, result, "10.0.0.1")
	assert.NotContains(t, result, "xoxb-111-222-abc")
}

func TestRedactCleanString(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	r := New(cfg)

	input := "hello world, no secrets here"
	result := r.Redact(input)

	assert.Equal(t, "hello world, no secrets here", result)
}

func TestRedactDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false
	r := New(cfg)

	input := "DB_PASSWORD=hunter2 ghp_aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789"
	result := r.Redact(input)

	assert.Equal(t, input, result, "disabled redactor should be passthrough")
}

func TestRedactEmptyString(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	r := New(cfg)

	result := r.Redact("")
	assert.Equal(t, "", result)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.False(t, cfg.Enabled)
	assert.Equal(t, "private_only", cfg.RedactIPs)
	assert.Equal(t, "[REDACTED]", cfg.Placeholder)
	assert.Empty(t, cfg.CustomPatterns)
}

func TestNewRedactorSortsByPriority(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	r := New(cfg)

	// Verify rules are sorted by priority (ascending)
	require.Greater(t, len(r.rules), 1)
	for i := 1; i < len(r.rules); i++ {
		assert.LessOrEqual(t, r.rules[i-1].priority, r.rules[i].priority,
			"rules should be sorted by priority: %s (pri %d) should come before %s (pri %d)",
			r.rules[i-1].name, r.rules[i-1].priority,
			r.rules[i].name, r.rules[i].priority)
	}
}

func TestRedactCustomPlaceholder(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Placeholder = "***"
	r := New(cfg)

	input := "ghp_aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789"
	result := r.Redact(input)

	assert.Equal(t, "***", result)
}

func TestRedactStructuredSecretVariants(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	r := New(cfg)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "password with equals",
			input:    "DB_PASSWORD=hunter2",
			expected: "DB_PASSWORD=[REDACTED]",
		},
		{
			name:     "secret with colon",
			input:    "my_secret: supersecretvalue",
			expected: "my_secret: [REDACTED]",
		},
		{
			name:     "api_key with equals",
			input:    "api_key=abc123xyz",
			expected: "api_key=[REDACTED]",
		},
		{
			name:     "auth_token with colon space",
			input:    "auth_token: mytoken123",
			expected: "auth_token: [REDACTED]",
		},
		{
			name:     "TOKEN uppercase",
			input:    "MY_TOKEN=secret_value",
			expected: "MY_TOKEN=[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.Redact(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

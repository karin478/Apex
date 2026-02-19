package redact

import "regexp"

// rule represents a single redaction rule with a compiled regex pattern.
type rule struct {
	name     string
	priority int
	pattern  *regexp.Regexp
	replace  func(match string) string // nil means use Redactor.placeholder
}

// Compiled patterns — allocated once at package init time via MustCompile.
var (
	// Structured secrets: key=value or key: value where key contains
	// password, secret, token, api_key, api-key, auth_token, auth-token.
	structuredSecretRe = regexp.MustCompile(
		`(?i)([\w]*(?:password|secret|token|api[_-]?key|auth[_-]?token))\s*([=:])\s*(\S+)`,
	)

	// Bearer tokens in Authorization headers.
	bearerTokenRe = regexp.MustCompile(
		`(?i)Bearer\s+[A-Za-z0-9\-._~+/]+=*`,
	)

	// GitHub Personal Access Tokens (classic ghp_ and fine-grained ghs_).
	githubPATRe = regexp.MustCompile(
		`gh[ps]_[A-Za-z0-9]{36,}`,
	)

	// OpenAI / Anthropic API keys (including sk-proj-* format).
	openaiKeyRe = regexp.MustCompile(
		`sk-[A-Za-z0-9\-]{20,}`,
	)

	// AWS access key IDs.
	awsAccessKeyRe = regexp.MustCompile(
		`AKIA[A-Z0-9]{16}`,
	)

	// AWS secret access keys (structured key=value form).
	awsSecretRe = regexp.MustCompile(
		`(?i)(aws_secret_access_key|aws_secret)\s*([=:])\s*(\S+)`,
	)

	// Slack bot / user / app tokens.
	slackTokenRe = regexp.MustCompile(
		`xox[bpsar]-[A-Za-z0-9\-]+`,
	)

	// Private IPv4 ranges: 10.x.x.x, 172.16-31.x.x, 192.168.x.x
	privateIPRe = regexp.MustCompile(
		`\b(?:10\.\d{1,3}\.\d{1,3}\.\d{1,3}|172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3})\b`,
	)

	// Any IPv4 address.
	allIPRe = regexp.MustCompile(
		`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`,
	)
)

// builtinRules returns the 7 built-in secret-detection rules.
// Priority 10-70 so they run before IP rules (80) and custom rules (90).
func builtinRules(placeholder string) []rule {
	return []rule{
		{
			name:     "structured_secret",
			priority: 10,
			pattern:  structuredSecretRe,
			replace: func(match string) string {
				// Preserve the key name and separator, redact only the value.
				loc := structuredSecretRe.FindStringSubmatchIndex(match)
				if loc == nil {
					return placeholder
				}
				// Group 1: key name, Group 2: separator (= or :), Group 3: value
				key := match[loc[2]:loc[3]]
				sep := match[loc[4]:loc[5]]
				// Reconstruct: preserve whitespace between separator and value
				sepEnd := loc[5]
				valStart := loc[6]
				spacing := match[sepEnd:valStart]
				return key + sep + spacing + placeholder
			},
		},
		{
			name:     "bearer_token",
			priority: 20,
			pattern:  bearerTokenRe,
			replace: func(match string) string {
				return placeholder
			},
		},
		{
			name:     "github_pat",
			priority: 30,
			pattern:  githubPATRe,
			replace: func(match string) string {
				return placeholder
			},
		},
		{
			name:     "openai_anthropic_key",
			priority: 40,
			pattern:  openaiKeyRe,
			replace: func(match string) string {
				return placeholder
			},
		},
		{
			name:     "aws_access_key",
			priority: 50,
			pattern:  awsAccessKeyRe,
			replace: func(match string) string {
				return placeholder
			},
		},
		{
			name:     "aws_secret",
			priority: 60,
			pattern:  awsSecretRe,
			replace: func(match string) string {
				loc := awsSecretRe.FindStringSubmatchIndex(match)
				if loc == nil {
					return placeholder
				}
				key := match[loc[2]:loc[3]]
				sep := match[loc[4]:loc[5]]
				return key + sep + placeholder
			},
		},
		{
			name:     "slack_token",
			priority: 70,
			pattern:  slackTokenRe,
			replace: func(match string) string {
				return placeholder
			},
		},
	}
}

// ipRules returns rules for IP address redaction based on the mode.
//   - "private_only": redact RFC 1918 private addresses only
//   - "all":          redact any IPv4 address
//   - "none":         no IP redaction rules
func ipRules(mode, placeholder string) []rule {
	switch mode {
	case "private_only":
		return []rule{
			{
				name:     "private_ip",
				priority: 80,
				pattern:  privateIPRe,
				replace: func(match string) string {
					return placeholder
				},
			},
		}
	case "all":
		return []rule{
			{
				name:     "all_ip",
				priority: 80,
				pattern:  allIPRe,
				replace: func(match string) string {
					return placeholder
				},
			},
		}
	default: // "none" or unrecognized
		return nil
	}
}

// customRules compiles user-supplied regex patterns into rules.
// Invalid patterns are silently skipped.
func customRules(patterns []string, placeholder string) []rule {
	var rules []rule
	for i, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			continue // skip invalid patterns
		}
		ph := placeholder // capture for closure
		rules = append(rules, rule{
			name:     "custom_" + p,
			priority: 90 + i,
			pattern:  re,
			replace: func(match string) string {
				return ph
			},
		})
	}
	return rules
}

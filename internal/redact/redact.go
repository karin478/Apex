package redact

import "sort"

// RedactionConfig controls what the Redactor redacts.
type RedactionConfig struct {
	Enabled        bool     `yaml:"enabled"`
	RedactIPs      string   `yaml:"redact_ips"`      // "private_only" | "all" | "none"
	CustomPatterns []string `yaml:"custom_patterns"`
	Placeholder    string   `yaml:"placeholder"`
}

// DefaultConfig returns a RedactionConfig with sensible defaults.
// Redaction is disabled by default; callers opt in by setting Enabled = true.
func DefaultConfig() RedactionConfig {
	return RedactionConfig{
		Enabled:   false,
		RedactIPs: "private_only",
		Placeholder: "[REDACTED]",
	}
}

// Redactor applies a sorted set of redaction rules to strings.
type Redactor struct {
	rules       []rule
	placeholder string
}

// New compiles a Redactor from the given config. If cfg.Enabled is false,
// the returned Redactor is a passthrough (no rules, Redact returns input
// unchanged).
func New(cfg RedactionConfig) *Redactor {
	if !cfg.Enabled {
		return &Redactor{placeholder: cfg.Placeholder}
	}

	placeholder := cfg.Placeholder
	if placeholder == "" {
		placeholder = "[REDACTED]"
	}

	var rules []rule
	rules = append(rules, builtinRules(placeholder)...)
	rules = append(rules, ipRules(cfg.RedactIPs, placeholder)...)
	rules = append(rules, customRules(cfg.CustomPatterns, placeholder)...)

	sort.SliceStable(rules, func(i, j int) bool {
		return rules[i].priority < rules[j].priority
	})

	return &Redactor{
		rules:       rules,
		placeholder: placeholder,
	}
}

// Redact applies all compiled rules sequentially to the input string and
// returns the redacted result.
func (r *Redactor) Redact(input string) string {
	if len(r.rules) == 0 {
		return input
	}

	result := input
	for _, rule := range r.rules {
		if rule.replace != nil {
			result = rule.pattern.ReplaceAllStringFunc(result, rule.replace)
		} else {
			result = rule.pattern.ReplaceAllString(result, r.placeholder)
		}
	}
	return result
}

# Phase 15: Redaction Pipeline — Design Document

> Date: 2026-02-19
> Status: Approved
> Architecture Ref: v11.0 §2.6 Execution Layer — Redaction Pipeline

## 1. Goal

All persisted data (audit logs, error fields, exception traces) must pass through a redaction pipeline before disk write, preventing leakage of API keys, tokens, passwords, and sensitive IP addresses.

## 2. Package: `internal/redact`

### 2.1 Core API

```go
// RedactionConfig holds redaction settings from apex.yaml.
type RedactionConfig struct {
    Enabled        bool     `yaml:"enabled"`
    RedactIPs      string   `yaml:"redact_ips"`      // "private_only" | "all" | "none"
    CustomPatterns []string `yaml:"custom_patterns"`
    Placeholder    string   `yaml:"placeholder"`      // default "[REDACTED]"
}

// Redactor applies compiled rules to strip secrets from text.
type Redactor struct { /* compiled rules + config */ }

func New(cfg RedactionConfig) *Redactor
func (r *Redactor) Redact(input string) string
func (r *Redactor) RedactJSON(data []byte) []byte
```

### 2.2 Rule Priority Chain

Per architecture §2.6, rules execute in priority order (first match wins per token):

| Priority | Category | Examples |
|----------|----------|----------|
| 1 | Structured key=value fields | `*_TOKEN=xxx`, `*_SECRET=xxx`, `*_KEY=xxx`, `*_PASSWORD=xxx` |
| 2 | Well-known token patterns | `Bearer xxx`, `ghp_xxx`, `sk-xxx`, `AKIA xxx`, `xox[bpsar]-xxx` |
| 3 | Custom patterns | User-configured via `redaction.custom_patterns` |
| 4 | IP addresses | Configurable: `private_only` / `all` / `none` |

> Level 0 (Credential Injector placeholders) is deferred to the future Connector Spec Framework phase.

### 2.3 Built-in Regex Rules (8 patterns)

1. **Structured secrets**: `(?i)([\w]*(?:password|secret|token|api[_-]?key|auth[_-]?token))\s*[=:]\s*(\S+)` → keeps key name, redacts value
2. **Bearer tokens**: `Bearer\s+[A-Za-z0-9\-._~+/]+=*`
3. **GitHub PATs**: `gh[ps]_[A-Za-z0-9]{36,}`
4. **OpenAI/Anthropic keys**: `sk-[A-Za-z0-9]{20,}`
5. **AWS access keys**: `AKIA[A-Z0-9]{16}`
6. **AWS secret values**: `(?i)(aws_secret_access_key|aws_secret)\s*[=:]\s*\S+`
7. **Slack tokens**: `xox[bpsar]-[A-Za-z0-9\-]+`
8. **IP addresses** (mode-dependent):
   - `private_only`: `10\.\d{1,3}\.\d{1,3}\.\d{1,3}`, `172\.(1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}`, `192\.168\.\d{1,3}\.\d{1,3}`
   - `all`: `\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`

## 3. Config Extension

```yaml
# apex.yaml
redaction:
  enabled: true
  redact_ips: "private_only"
  custom_patterns:
    - "INTERNAL_.*_TOKEN"
  placeholder: "[REDACTED]"
```

`RedactionConfig` added to `config.Config` struct with sensible defaults:
- `enabled: true`
- `redact_ips: "private_only"`
- `placeholder: "[REDACTED]"`

## 4. Integration: Audit Logger

In `audit.Logger.Log()`, after building the `Record` struct but **before** computing the SHA-256 hash:

```
record.Task  = redactor.Redact(record.Task)
record.Error = redactor.Redact(record.Error)
record.Hash  = computeHash(record)  // hash covers redacted content
```

The hash chain integrity is preserved because it covers the post-redaction data.

## 5. CLI Command

```
apex redact test <input-string>
```

Applies the current redaction config and prints the result. Useful for verifying rules before deployment.

## 6. Files

| File | Purpose |
|------|---------|
| `internal/redact/redact.go` | Redactor struct, New(), Redact(), RedactJSON() |
| `internal/redact/rules.go` | Built-in rules, IP rules, custom rule compilation |
| `internal/redact/redact_test.go` | Unit tests (all patterns + edge cases) |
| `internal/config/config.go` | Add RedactionConfig to Config |
| `internal/audit/logger.go` | Integrate Redactor in Log() path |
| `cmd/apex/redact.go` | CLI `apex redact test` command |
| `cmd/apex/main.go` | Register redactCmd |
| `e2e/redact_test.go` | E2E tests |

## 7. Performance

Target: < 10ms per audit entry (per architecture §2.6).
All regexes are compiled once at `New()` time and reused.

## 8. Out of Scope

- Credential Injector (future Connector Spec Framework phase)
- AES-256-GCM at-rest encryption (can be layered on top later)
- NLI-based semantic redaction (architecture mentions < 3s timeout bypass)

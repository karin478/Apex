# Phase 15: Redaction Pipeline — Implementation Plan

> Date: 2026-02-19
> Design Doc: `2026-02-19-phase15-redaction-pipeline-design.md`
> Method: Subagent-Driven Development (TDD)

## Tasks

### Task 1: Redactor Core + Built-in Rules

**Create:** `internal/redact/redact.go`, `internal/redact/rules.go`, `internal/redact/redact_test.go`

**Data model:**
```go
type RedactionConfig struct {
    Enabled        bool     `yaml:"enabled"`
    RedactIPs      string   `yaml:"redact_ips"`
    CustomPatterns []string `yaml:"custom_patterns"`
    Placeholder    string   `yaml:"placeholder"`
}

type rule struct {
    name     string
    priority int
    pattern  *regexp.Regexp
    replace  func(match string) string
}

type Redactor struct {
    rules       []rule
    placeholder string
}
```

**Functions:**
- `DefaultConfig() RedactionConfig` — returns default redaction config
- `New(cfg RedactionConfig) *Redactor` — compile rules, sorted by priority
- `(r *Redactor) Redact(input string) string` — apply rules sequentially
- `builtinRules(placeholder string) []rule` — 7 regex patterns
- `ipRules(mode, placeholder string) []rule` — IP redaction based on mode
- `customRules(patterns []string, placeholder string) []rule` — user patterns

**Tests (TDD — write first, verify fail, then implement):**
1. `TestRedactBearerToken` — `"Authorization: Bearer sk-abc123def456"` → value redacted
2. `TestRedactGitHubPAT` — `"ghp_aBcDeFgHiJkLmNoPqRsTuVwXyZ012345678"` → redacted
3. `TestRedactOpenAIKey` — `"sk-proj-abcdef1234567890abcdef"` → redacted
4. `TestRedactAWSAccessKey` — `"AKIAIOSFODNN7EXAMPLE"` → redacted
5. `TestRedactSlackToken` — `"xoxb-1234-5678-abcdefgh"` → redacted
6. `TestRedactStructuredSecret` — `"DB_PASSWORD=hunter2"` → key preserved, value redacted
7. `TestRedactPrivateIP` — `"connecting to 192.168.1.100"` → IP redacted (private_only mode)
8. `TestRedactPublicIPSkipped` — `"connecting to 8.8.8.8"` → NOT redacted in private_only mode
9. `TestRedactAllIPs` — both private and public redacted in `all` mode
10. `TestRedactIPNone` — no IPs redacted in `none` mode
11. `TestRedactCustomPattern` — custom pattern from config works
12. `TestRedactMultipleSecrets` — string with multiple secrets all redacted
13. `TestRedactCleanString` — no secrets → string unchanged
14. `TestRedactDisabled` — `Enabled: false` → passthrough, no redaction
15. `TestRedactEmptyString` — empty input → empty output

**Commit:** `feat(redact): add Redactor with built-in rules and IP redaction`

---

### Task 2: Config Integration

**Modify:** `internal/config/config.go`, `internal/config/config_test.go`

**Changes:**
- Import `redact` package's `RedactionConfig` (or define inline to avoid circular dep)
- Add `Redaction RedactionConfig` field to `Config` struct
- Set defaults in `Default()`: enabled=true, redact_ips="private_only", placeholder="[REDACTED]"
- Add zero-value guards in `Load()`

**Tests:**
1. `TestDefaultRedactionConfig` — verify defaults
2. `TestLoadRedactionConfig` — YAML with custom redaction settings parsed correctly

**Commit:** `feat(config): add RedactionConfig to apex configuration`

---

### Task 3: Audit Logger Integration

**Modify:** `internal/audit/logger.go`, `internal/audit/logger_test.go`

**Changes:**
- Add `redactor *redact.Redactor` field to `Logger` struct
- Add `SetRedactor(r *redact.Redactor)` method
- In `Log()`, before hash computation: redact `record.Task` and `record.Error` fields
- If redactor is nil, skip redaction (backward compatible)

**Tests:**
1. `TestLogRedactsSecretInTask` — task containing `sk-xxx` → audit record has redacted value
2. `TestLogRedactsSecretInError` — error containing Bearer token → redacted in record
3. `TestLogHashCoversRedactedContent` — hash verifiable on the redacted (stored) content
4. `TestLogNoRedactorPassthrough` — nil redactor → no redaction (backward compat)

**Commit:** `feat(audit): integrate redaction pipeline before hash computation`

---

### Task 4: CLI Command

**Create:** `cmd/apex/redact.go`
**Modify:** `cmd/apex/main.go`

**Commands:**
- `apex redact test <input>` — apply redaction rules and print result

**Implementation:**
- Load config, create Redactor, call Redact(args), print result
- Register `redactCmd` in main.go init()

**Commit:** `feat(cli): add apex redact test command`

---

### Task 5: E2E Tests

**Create:** `e2e/redact_test.go`

**Tests:**
1. `TestRedactTestCommand` — `apex redact test "Bearer sk-xxx"` → output contains [REDACTED]
2. `TestRedactTestClean` — `apex redact test "hello world"` → output unchanged
3. `TestAuditEntryRedacted` — run task with secret in name → verify audit log has redacted content

**Commit:** `test(e2e): add redaction pipeline E2E tests`

---

### Task 6: Update PROGRESS.md

**Modify:** `PROGRESS.md`

- Add Phase 15 row as Done
- Update Current to Phase 16 — TBD
- Update test counts
- Add `internal/redact` to Key Packages

**Commit:** `docs: mark Phase 15 Redaction Pipeline as complete`

## Summary

| Task | Files | Tests | Description |
|------|-------|-------|-------------|
| 1 | redact.go, rules.go, redact_test.go | 15 | Core Redactor + all rules |
| 2 | config.go, config_test.go | 2 | Config integration |
| 3 | logger.go, logger_test.go | 4 | Audit integration |
| 4 | redact.go (cmd), main.go | — | CLI command |
| 5 | redact_test.go (e2e) | 3 | E2E tests |
| 6 | PROGRESS.md | — | Documentation |
| **Total** | **10 files** | **24 new tests** | |

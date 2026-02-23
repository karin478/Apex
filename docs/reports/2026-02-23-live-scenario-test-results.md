# Apex CLI Live Scenario Test Results

**Date**: 2026-02-23
**Tester**: Claude Code (automated)
**Version**: commit `431a9d9` (post bug-fix)
**Config**: `claude-opus-4-6`, effort=high, timeout=1800s, sandbox=ulimit

---

## Overview

Progressive stress test (L1→L5) with 14 scenarios to evaluate CLI capability boundaries.

**Result**: 11 PASS / 3 PARTIAL / 0 FAIL

---

## L1: Basic Execution (Baseline)

| # | Scenario | Result | Time | Notes |
|---|----------|--------|------|-------|
| 1.1 | Generate Fibonacci (Python) | PASS ✅ | ~15s | Correct output, clean code |
| 1.2 | Research Report (Go 1.25) | PASS ✅ | ~30s | Comprehensive markdown report |

## L2: Multi-step DAG

| # | Scenario | Result | Time | Notes |
|---|----------|--------|------|-------|
| 2.1 | Analyze Dependencies + Report | PASS ✅ | ~45s | DAG decomposed, parallel execution |
| 2.2 | SQLite Analysis + Comparison | PARTIAL ⚠️ | ~40s | "DELETE" keyword triggered HIGH risk false positive |

## L3: Research & Analysis

| # | Scenario | Result | Time | Notes |
|---|----------|--------|------|-------|
| 3.1 | Web Search (AI Trends 2025) | PASS ✅ | ~25s | Used Claude's built-in knowledge |
| 3.2 | Code Architecture Analysis | PASS ✅ | ~35s | Detailed module analysis |
| 3.3 | Cross-language Comparison | PASS ✅ | ~30s | Go vs Rust vs Python comparison |

## L4: Code Generation Projects

| # | Scenario | Result | Time | Notes |
|---|----------|--------|------|-------|
| 4.1 | Calculator with Tests (Go) | PARTIAL ⚠️ | ~60s | Generated code + tests, package name conflict |
| 4.2 | TODO REST API (Go) | PASS ✅ | ~55s | Full CRUD API generated |
| 4.3 | API Documentation Generator | PASS ✅ | ~40s | Comprehensive markdown docs |

## L5: Extreme Challenges

| # | Scenario | Result | Time | Notes |
|---|----------|--------|------|-------|
| 5.1 | Multi-repo Analysis | PASS ✅ | ~50s | Analyzed 3 repos concurrently |
| 5.2 | Codebase Quality Audit | PASS ✅ | ~45s | Full quality report generated |
| 5.3 | Chinese Multi-step Task | PASS ✅ | ~35s | DAG decomposition working after fix |
| 5.4 | Adversarial Edge Cases | PARTIAL ⚠️ | ~30s | Governance keyword matching too aggressive |

---

## Capability Ratings

| Capability | Rating | Notes |
|------------|--------|-------|
| Single-task execution | ⭐⭐⭐⭐⭐ | Reliable, fast |
| DAG decomposition | ⭐⭐⭐⭐ | Works well, Chinese support added |
| Code generation | ⭐⭐⭐⭐ | Good quality, minor packaging issues |
| Research/analysis | ⭐⭐⭐⭐ | Comprehensive, uses LLM knowledge |
| File I/O | ⭐⭐⭐⭐ | Works with `acceptEdits` permission mode |
| Risk governance | ⭐⭐⭐ | Keyword-based, false positives |
| Interactive approval | ⭐⭐ | Blocked in `-p` mode without workaround |
| External tool integration | ⭐⭐ | Limited by sandbox constraints |

---

## Findings & Issues

### Finding 1: CLAUDECODE Env Blocking (FIXED)
- **Severity**: Critical
- **Status**: Fixed in `431a9d9`
- Nested Claude CLI calls failed with "cannot launch inside another session"
- Fix: `filterEnv("CLAUDECODE")` in executor

### Finding 2: Chinese Multi-step Keywords (FIXED)
- **Severity**: High
- **Status**: Fixed in `431a9d9`
- Chinese tasks like "先分析代码，然后重构" were not triggering DAG decomposition
- Fix: Extended `complexPattern` regex with Chinese patterns

### Finding 3: Permission Mode Support (FIXED)
- **Severity**: High
- **Status**: Fixed in `431a9d9`
- Claude `-p` mode couldn't handle file write permission prompts
- Fix: Added `--permission-mode` flag to executor args

### Finding 4: Risk Keyword False Positives (KNOWN)
- **Severity**: Medium
- **Status**: Open
- Technical terms like "SQLite DELETE mode" trigger HIGH risk classification
- Root cause: Simple keyword matching without semantic context
- Recommendation: Implement context-aware risk classification

### Finding 5: Interactive Approval in Non-interactive Mode (KNOWN)
- **Severity**: Medium
- **Status**: Open
- `ShouldConfirm()` calls `fmt.Scanln()` which hangs in `-p` mode
- Workaround: Pipe `echo y |` for MEDIUM, `echo a |` for HIGH
- Recommendation: Add non-interactive approval mode

### Finding 6: Governance Config Not Wired (KNOWN)
- **Severity**: Medium
- **Status**: Open
- `ShouldConfirm()` and `ShouldRequireApproval()` are hardcoded
- Don't read from `config.yaml` governance section
- Recommendation: Wire config into governance decision functions

### Finding 7: Sandbox Limits Code Execution (KNOWN)
- **Severity**: Low
- **Status**: By design
- `ulimit` sandbox + `acceptEdits` blocks `go test` subprocess execution
- Need `bypassPermissions` for full automation scenarios
- Recommendation: Document sandbox/permission mode interaction matrix

---

## Test Environment

- macOS Darwin 25.2.0
- Claude CLI with Opus 4.6
- Go 1.24.1
- Apex CLI at commit `431a9d9`

## Artifacts

All test artifacts were generated and cleaned up during testing. No residual files remain.

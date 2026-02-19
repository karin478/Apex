# Phase 13: Adversarial Review — Design Document

> Approved: 2026-02-19

## Overview

New `apex review <proposal>` command that subjects a technical proposal to a structured three-party debate:

- **Advocate** (Blue Team) — argues in favor of the proposal
- **Critic** (Red Team) — challenges weaknesses and raises risks
- **Judge** — synthesizes both sides into a final verdict

This implements the Adversarial Review reasoning protocol from architecture §2.8, addressing risk R5 (echo chamber effect).

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scope | Independent CLI command | Validate protocol before DAG integration |
| Debate rounds | Fixed 1 round (4 Claude calls) | Predictable cost, sufficient for most reviews |
| Output | JSON + terminal summary | Structured storage for tooling, concise UX |

## Debate Flow

```
User input: proposal text
    │
    ▼
[Step 1] Advocate — present strengths and justification
    │
    ▼
[Step 2] Critic — receive proposal + advocate argument, raise challenges
    │
    ▼
[Step 3] Advocate — receive critic challenges, respond point-by-point
    │
    ▼
[Step 4] Judge — receive full transcript, deliver verdict
    │
    ▼
Terminal: verdict summary
File: ~/.apex/reviews/{id}.json
Audit: review event logged
```

Each step is a single `executor.Executor.Run()` call with a role-specific system prompt prepended to the task.

## New Package: `internal/reasoning`

### File: `review.go`

```go
type Step struct {
    Role   string `json:"role"`   // "advocate", "critic", "judge"
    Prompt string `json:"prompt"` // full prompt sent to Claude
    Output string `json:"output"` // Claude response
}

type Verdict struct {
    Decision string   `json:"decision"` // "approve", "reject", "revise"
    Summary  string   `json:"summary"`
    Risks    []string `json:"risks"`
    Actions  []string `json:"suggested_actions"`
}

type ReviewResult struct {
    ID        string        `json:"id"`
    Proposal  string        `json:"proposal"`
    CreatedAt string        `json:"created_at"`
    Steps     []Step        `json:"steps"`
    Verdict   Verdict       `json:"verdict"`
    DurationMs int64        `json:"duration_ms"`
}
```

### Core Function

```go
func RunReview(ctx context.Context, exec *executor.Executor, proposal string) (*ReviewResult, error)
```

Sequentially executes 4 Claude calls, building context for each step:
1. Advocate prompt = system prompt + proposal
2. Critic prompt = system prompt + proposal + advocate output
3. Advocate response prompt = system prompt + proposal + critic output
4. Judge prompt = system prompt + full transcript

### System Prompts

Each role gets a focused system prompt:

- **Advocate**: "You are a technical advocate. Present the strongest case FOR this proposal. Focus on benefits, feasibility, and alignment with goals."
- **Critic**: "You are a technical critic. Challenge the proposal and the advocate's arguments. Identify risks, blind spots, and alternatives."
- **Judge**: "You are an impartial technical judge. Review the full debate and deliver a verdict. Output JSON with: decision (approve/reject/revise), summary, risks[], suggested_actions[]."

The Judge is instructed to return structured JSON for the Verdict.

## Storage

- Directory: `~/.apex/reviews/`
- File: `{uuid}.json` containing full `ReviewResult`
- Permissions: `0600`
- Atomic write: tmp + rename pattern (consistent with audit anchors)

## CLI Command

### File: `cmd/apex/review.go`

```
apex review "Use Redis as caching layer instead of in-memory cache"
```

**Terminal output:**

```
Adversarial Review
==================

Advocate... done (3.2s)
Critic..... done (4.1s)
Response... done (3.8s)
Judge...... done (5.1s)

Verdict: APPROVE with conditions
Summary: Redis caching is viable but needs failover strategy...

Risks:
  1. Single point of failure without Sentinel/Cluster
  2. Serialization overhead for high-frequency reads

Suggested Actions:
  1. Benchmark in-memory vs Redis latency
  2. Design fallback to in-memory cache

Full report: ~/.apex/reviews/abc123.json
```

## Audit Integration

After each review, write one audit record:

```go
logger.Log(audit.Entry{
    Task:      "review: " + truncate(proposal, 80),
    RiskLevel: "LOW",
    Outcome:   verdict.Decision,
    Duration:  duration,
    Model:     cfg.Claude.Model,
})
```

## Testing Strategy

### Unit Tests (`internal/reasoning/review_test.go`)

- `TestRunReview` — mock executor returning canned responses, verify 4 steps + verdict parsing
- `TestRunReviewCriticFailure` — executor error on step 2, verify graceful error
- `TestVerdictParsing` — test JSON extraction from Judge output (clean JSON, JSON in markdown, malformed)
- `TestSaveAndLoadReview` — write/read review JSON file, verify round-trip

### E2E Tests (`e2e/review_test.go`)

- `TestReviewHappyPath` — run `apex review "test proposal"`, verify exit 0, output contains "Verdict", review file created
- `TestReviewNoArgs` — run `apex review` with no args, verify error message

## Config

No new config fields needed. Uses existing `claude.model`, `claude.effort`, `claude.timeout` from `apex.yaml`.

## Dependencies

- `internal/executor` — Claude CLI wrapper (existing)
- `internal/audit` — audit logging (existing)
- `internal/config` — config loader (existing)
- `github.com/google/uuid` — review ID generation (new dependency)

# Phase 20: Hypothesis Board — Design Document

> Date: 2026-02-20
> Status: Approved
> Architecture Ref: v11.0 §2.8 Reasoning Protocols

## 1. Goal

Provide a structured hypothesis tracking system for complex debugging and analysis tasks. Hypotheses go through a lifecycle: PROPOSED → CHALLENGED → CONFIRMED/REJECTED, with evidence scoring. Persisted per-session for later review.

## 2. Package: `internal/hypothesis`

### 2.1 Core Types

```go
type Status string

const (
    Proposed  Status = "PROPOSED"
    Challenged Status = "CHALLENGED"
    Confirmed  Status = "CONFIRMED"
    Rejected   Status = "REJECTED"
)

type Evidence struct {
    Type       string  // file_hash, log_line, user_confirmation, observation
    Content    string  // the evidence text
    Confidence float64 // 0.0-1.0
}

type Hypothesis struct {
    ID         string     `json:"id"`
    Statement  string     `json:"statement"`
    Status     Status     `json:"status"`
    Evidence   []Evidence `json:"evidence"`
    CreatedAt  string     `json:"created_at"`
    UpdatedAt  string     `json:"updated_at"`
}

type Board struct {
    SessionID   string       `json:"session_id"`
    Hypotheses  []Hypothesis `json:"hypotheses"`
}
```

### 2.2 Operations

```go
func NewBoard(sessionID string) *Board
func (b *Board) Propose(statement string) *Hypothesis
func (b *Board) Challenge(id string, evidence Evidence) error
func (b *Board) Confirm(id string, evidence Evidence) error
func (b *Board) Reject(id string, reason string) error
func (b *Board) Get(id string) (*Hypothesis, error)
func (b *Board) List() []Hypothesis
func (b *Board) Score(h *Hypothesis) float64 // avg confidence of evidence
```

### 2.3 Persistence

- Save: `{baseDir}/sessions/{sessionID}/hypothesis_board.json`
- Load: `LoadBoard(path string) (*Board, error)`
- Save: `(b *Board) Save(path string) error`

## 3. CLI: `apex hypothesis`

```
apex hypothesis list                    # list all hypotheses for latest session
apex hypothesis propose "statement"     # add new hypothesis
apex hypothesis challenge <id> "evidence"  # add challenging evidence
apex hypothesis confirm <id> "evidence"    # confirm with evidence
apex hypothesis reject <id> "reason"       # reject hypothesis
```

## 4. Non-Goals

- No LLM-driven hypothesis generation (manual only)
- No cross-session hypothesis linking
- No confidence weighting beyond simple average
- No integration with adversarial review (future enhancement)

package hypothesis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Status represents the lifecycle state of a hypothesis.
type Status string

const (
	Proposed   Status = "PROPOSED"
	Challenged Status = "CHALLENGED"
	Confirmed  Status = "CONFIRMED"
	Rejected   Status = "REJECTED"
)

// Evidence supports or challenges a hypothesis.
type Evidence struct {
	Type       string  `json:"type"`       // file_hash, log_line, user_confirmation, observation
	Content    string  `json:"content"`
	Confidence float64 `json:"confidence"` // 0.0-1.0
}

// Hypothesis represents a single hypothesis on the board.
type Hypothesis struct {
	ID        string     `json:"id"`
	Statement string     `json:"statement"`
	Status    Status     `json:"status"`
	Evidence  []Evidence `json:"evidence,omitempty"`
	CreatedAt string     `json:"created_at"`
	UpdatedAt string     `json:"updated_at"`
}

// Board tracks hypotheses for a session.
type Board struct {
	SessionID  string       `json:"session_id"`
	Hypotheses []Hypothesis `json:"hypotheses"`
}

// NewBoard creates an empty board for the given session.
func NewBoard(sessionID string) *Board {
	return &Board{SessionID: sessionID}
}

// Propose adds a new hypothesis with PROPOSED status.
func (b *Board) Propose(statement string) *Hypothesis {
	now := time.Now().UTC().Format(time.RFC3339)
	h := Hypothesis{
		ID:        uuid.New().String()[:8],
		Statement: statement,
		Status:    Proposed,
		CreatedAt: now,
		UpdatedAt: now,
	}
	b.Hypotheses = append(b.Hypotheses, h)
	return &b.Hypotheses[len(b.Hypotheses)-1]
}

// Challenge adds challenging evidence and sets status to CHALLENGED.
func (b *Board) Challenge(id string, ev Evidence) error {
	h, err := b.Get(id)
	if err != nil {
		return err
	}
	h.Evidence = append(h.Evidence, ev)
	h.Status = Challenged
	h.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// Confirm adds confirming evidence and sets status to CONFIRMED.
func (b *Board) Confirm(id string, ev Evidence) error {
	h, err := b.Get(id)
	if err != nil {
		return err
	}
	h.Evidence = append(h.Evidence, ev)
	h.Status = Confirmed
	h.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// Reject sets a hypothesis to REJECTED with a reason.
func (b *Board) Reject(id string, reason string) error {
	h, err := b.Get(id)
	if err != nil {
		return err
	}
	h.Evidence = append(h.Evidence, Evidence{
		Type:       "rejection",
		Content:    reason,
		Confidence: 1.0,
	})
	h.Status = Rejected
	h.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// Get returns a pointer to the hypothesis with the given ID.
func (b *Board) Get(id string) (*Hypothesis, error) {
	for i := range b.Hypotheses {
		if b.Hypotheses[i].ID == id {
			return &b.Hypotheses[i], nil
		}
	}
	return nil, fmt.Errorf("hypothesis %s not found", id)
}

// List returns all hypotheses.
func (b *Board) List() []Hypothesis {
	return b.Hypotheses
}

// Score returns the average confidence of all evidence for a hypothesis.
func Score(h *Hypothesis) float64 {
	if len(h.Evidence) == 0 {
		return 0
	}
	var total float64
	for _, ev := range h.Evidence {
		total += ev.Confidence
	}
	return total / float64(len(h.Evidence))
}

// Save writes the board to a JSON file.
func (b *Board) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadBoard reads a board from a JSON file.
func LoadBoard(path string) (*Board, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var b Board
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

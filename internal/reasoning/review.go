package reasoning

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Step struct {
	Role   string `json:"role"`
	Prompt string `json:"prompt"`
	Output string `json:"output"`
}

type Verdict struct {
	Decision string   `json:"decision"`
	Summary  string   `json:"summary"`
	Risks    []string `json:"risks"`
	Actions  []string `json:"suggested_actions"`
}

type ReviewResult struct {
	ID         string  `json:"id"`
	Proposal   string  `json:"proposal"`
	CreatedAt  string  `json:"created_at"`
	Steps      []Step  `json:"steps"`
	Verdict    Verdict `json:"verdict"`
	DurationMs int64   `json:"duration_ms"`
}

func SaveReview(dir string, result *ReviewResult) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, result.ID+".json")
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadReview(dir, id string) (*ReviewResult, error) {
	path := filepath.Join(dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load review %s: %w", id, err)
	}
	var result ReviewResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse review %s: %w", id, err)
	}
	return &result, nil
}

// Runner executes a single task prompt and returns the text output.
type Runner interface {
	RunTask(ctx context.Context, task string) (string, error)
}

// ProgressFunc is called after each debate step with the step index and duration.
type ProgressFunc func(step int, duration time.Duration)

const (
	advocateSystem = "You are a technical advocate. Present the strongest case FOR this proposal. Focus on benefits, feasibility, and alignment with goals. Be specific and concrete."
	criticSystem   = "You are a technical critic. Challenge the proposal and the advocate's arguments. Identify risks, blind spots, missing considerations, and better alternatives. Be thorough."
	responseSystem = "You are the original advocate. The critic has raised challenges to your proposal. Address each concern point-by-point. Acknowledge valid criticisms and explain mitigations."
	judgeSystem    = `You are an impartial technical judge. Review the full debate transcript and deliver a final verdict. You MUST respond with ONLY a JSON object in this exact format:
{"decision":"approve|reject|revise","summary":"1-2 sentence verdict","risks":["risk1","risk2"],"suggested_actions":["action1","action2"]}
Do not include any text before or after the JSON.`
)

// RunReview executes a 4-step adversarial review debate.
func RunReview(ctx context.Context, runner Runner, proposal string) (*ReviewResult, error) {
	return RunReviewWithProgress(ctx, runner, proposal, nil)
}

// RunReviewWithProgress is like RunReview but calls progress after each step.
func RunReviewWithProgress(ctx context.Context, runner Runner, proposal string, progress ProgressFunc) (*ReviewResult, error) {
	start := time.Now()
	result := &ReviewResult{
		ID:        uuid.New().String(),
		Proposal:  proposal,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	var advocateOut, criticOut, responseOut string

	type stepDef struct {
		role   string
		prompt func() string
	}

	steps := []stepDef{
		{"advocate", func() string {
			return fmt.Sprintf("%s\n\n## Proposal\n%s", advocateSystem, proposal)
		}},
		{"critic", func() string {
			return fmt.Sprintf("%s\n\n## Proposal\n%s\n\n## Advocate's Argument\n%s", criticSystem, proposal, advocateOut)
		}},
		{"advocate", func() string {
			return fmt.Sprintf("%s\n\n## Proposal\n%s\n\n## Critic's Challenges\n%s", responseSystem, proposal, criticOut)
		}},
		{"judge", func() string {
			return fmt.Sprintf("%s\n\n## Proposal\n%s\n\n## Advocate's Argument\n%s\n\n## Critic's Challenges\n%s\n\n## Advocate's Response\n%s",
				judgeSystem, proposal, advocateOut, criticOut, responseOut)
		}},
	}

	for i, s := range steps {
		stepStart := time.Now()
		prompt := s.prompt()
		out, err := runner.RunTask(ctx, prompt)
		if err != nil {
			return nil, fmt.Errorf("%s step failed: %w", s.role, err)
		}
		result.Steps = append(result.Steps, Step{Role: s.role, Prompt: prompt, Output: out})

		switch i {
		case 0:
			advocateOut = out
		case 1:
			criticOut = out
		case 2:
			responseOut = out
		}

		if progress != nil {
			progress(i, time.Since(stepStart))
		}
	}

	verdict, err := parseVerdict(result.Steps[3].Output)
	if err != nil {
		return nil, fmt.Errorf("parse verdict: %w", err)
	}
	result.Verdict = verdict
	result.DurationMs = time.Since(start).Milliseconds()

	return result, nil
}

var jsonBlockRe = regexp.MustCompile("(?s)```(?:json)?\\s*\\n(\\{.*?\\})\\s*\\n```")

func parseVerdict(output string) (Verdict, error) {
	var v Verdict

	// Try direct JSON parse
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &v); err == nil && v.Decision != "" {
		return v, nil
	}

	// Try extracting from markdown code block
	if matches := jsonBlockRe.FindStringSubmatch(output); len(matches) > 1 {
		if err := json.Unmarshal([]byte(matches[1]), &v); err == nil && v.Decision != "" {
			return v, nil
		}
	}

	// Fallback: use full output as summary
	return Verdict{
		Decision: "revise",
		Summary:  strings.TrimSpace(output),
	}, nil
}

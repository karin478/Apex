package failclose

import (
	"errors"
	"fmt"
	"sync"
)

var ErrGateBlocked = errors.New("failclose: gate blocked")

type Condition struct {
	Name  string
	Check func() (bool, string)
}

type ConditionResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Reason string `json:"reason"`
}

type GateResult struct {
	Allowed  bool              `json:"allowed"`
	Failures []ConditionResult `json:"failures"`
	Passed   []ConditionResult `json:"passed"`
}

type Gate struct {
	mu         sync.RWMutex
	conditions []Condition
}

func NewGate() *Gate {
	return &Gate{}
}

func (g *Gate) AddCondition(c Condition) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.conditions = append(g.conditions, c)
}

func (g *Gate) Evaluate() GateResult {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := GateResult{Allowed: true}
	for _, c := range g.conditions {
		pass, reason := c.Check()
		cr := ConditionResult{Name: c.Name, Passed: pass, Reason: reason}
		if pass {
			result.Passed = append(result.Passed, cr)
		} else {
			result.Failures = append(result.Failures, cr)
			result.Allowed = false
		}
	}
	return result
}

func (g *Gate) MustPass() error {
	result := g.Evaluate()
	if result.Allowed {
		return nil
	}
	names := make([]string, len(result.Failures))
	for i, f := range result.Failures {
		names[i] = fmt.Sprintf("%s: %s", f.Name, f.Reason)
	}
	return fmt.Errorf("%w: %v", ErrGateBlocked, names)
}

func (g *Gate) Conditions() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	names := make([]string, len(g.conditions))
	for i, c := range g.conditions {
		names[i] = c.Name
	}
	return names
}

func HealthCondition(status string) Condition {
	return Condition{
		Name: "health",
		Check: func() (bool, string) {
			if status == "RED" || status == "CRITICAL" {
				return false, fmt.Sprintf("system health is %s", status)
			}
			return true, "system health OK"
		},
	}
}

func KillSwitchCondition(active bool) Condition {
	return Condition{
		Name: "killswitch",
		Check: func() (bool, string) {
			if active {
				return false, "kill switch is active"
			}
			return true, "kill switch inactive"
		},
	}
}

func DefaultGate() *Gate {
	g := NewGate()
	g.AddCondition(HealthCondition("GREEN"))
	g.AddCondition(KillSwitchCondition(false))
	return g
}

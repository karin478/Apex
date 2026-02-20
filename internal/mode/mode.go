// Package mode provides execution mode selection based on task complexity.
package mode

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Mode represents an execution mode.
type Mode string

const (
	ModeNormal      Mode = "NORMAL"
	ModeUrgent      Mode = "URGENT"
	ModeExploratory Mode = "EXPLORATORY"
	ModeBatch       Mode = "BATCH"
	ModeLongRunning Mode = "LONG_RUNNING"
)

// ModeConfig holds the configuration for an execution mode.
type ModeConfig struct {
	Name           Mode          `json:"name"            yaml:"name"`
	TokenReserve   int           `json:"token_reserve"   yaml:"token_reserve"`
	Concurrency    int           `json:"concurrency"     yaml:"concurrency"`
	SkipValidation bool          `json:"skip_validation" yaml:"skip_validation"`
	Timeout        time.Duration `json:"timeout"         yaml:"timeout"`
}

// DefaultModes returns the 5 built-in mode configurations.
func DefaultModes() map[Mode]ModeConfig {
	return map[Mode]ModeConfig{
		ModeNormal: {
			Name: ModeNormal, TokenReserve: 4000, Concurrency: 2,
			SkipValidation: false, Timeout: 5 * time.Minute,
		},
		ModeUrgent: {
			Name: ModeUrgent, TokenReserve: 2000, Concurrency: 4,
			SkipValidation: true, Timeout: 2 * time.Minute,
		},
		ModeExploratory: {
			Name: ModeExploratory, TokenReserve: 8000, Concurrency: 1,
			SkipValidation: false, Timeout: 10 * time.Minute,
		},
		ModeBatch: {
			Name: ModeBatch, TokenReserve: 4000, Concurrency: 8,
			SkipValidation: false, Timeout: 30 * time.Minute,
		},
		ModeLongRunning: {
			Name: ModeLongRunning, TokenReserve: 6000, Concurrency: 2,
			SkipValidation: false, Timeout: 60 * time.Minute,
		},
	}
}

// Selector manages mode registration and selection.
type Selector struct {
	mu      sync.RWMutex
	modes   map[Mode]ModeConfig
	current Mode
}

// NewSelector creates a Selector with the given modes, defaulting to NORMAL.
// If ModeNormal is not in the provided modes map, it defaults to the first
// mode found alphabetically.
func NewSelector(modes map[Mode]ModeConfig) *Selector {
	current := ModeNormal
	if _, ok := modes[ModeNormal]; !ok {
		// Fall back to the first mode alphabetically.
		names := make([]string, 0, len(modes))
		for m := range modes {
			names = append(names, string(m))
		}
		sort.Strings(names)
		if len(names) > 0 {
			current = Mode(names[0])
		}
	}
	return &Selector{modes: modes, current: current}
}

// Select manually selects a mode. Returns an error if the mode is unknown.
func (s *Selector) Select(mode Mode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.modes[mode]; !ok {
		return fmt.Errorf("mode: unknown mode %q", mode)
	}
	s.current = mode
	return nil
}

// Current returns the current mode and its configuration.
func (s *Selector) Current() (Mode, ModeConfig) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current, s.modes[s.current]
}

// List returns all registered modes sorted alphabetically by name.
func (s *Selector) List() []ModeConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	configs := make([]ModeConfig, 0, len(s.modes))
	for _, cfg := range s.modes {
		configs = append(configs, cfg)
	}
	sort.Slice(configs, func(i, j int) bool {
		return string(configs[i].Name) < string(configs[j].Name)
	})
	return configs
}

// SelectByComplexity selects a mode based on the complexity score.
// Score < 30 -> NORMAL, 30-60 -> EXPLORATORY, > 60 -> LONG_RUNNING.
// Also updates the current mode. If the auto-selected mode is not registered,
// the current mode is left unchanged and returned instead.
func (s *Selector) SelectByComplexity(score int) Mode {
	s.mu.Lock()
	defer s.mu.Unlock()
	var mode Mode
	switch {
	case score > 60:
		mode = ModeLongRunning
	case score >= 30:
		mode = ModeExploratory
	default:
		mode = ModeNormal
	}
	if _, ok := s.modes[mode]; !ok {
		return s.current
	}
	s.current = mode
	return mode
}

// Config returns the configuration for a specific mode.
func (s *Selector) Config(mode Mode) (ModeConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.modes[mode]
	if !ok {
		return ModeConfig{}, fmt.Errorf("mode: unknown mode %q", mode)
	}
	return cfg, nil
}

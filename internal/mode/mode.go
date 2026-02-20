// Package mode provides execution mode selection based on task complexity.
package mode

import (
	"fmt"
	"sort"
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
	modes   map[Mode]ModeConfig
	current Mode
}

// NewSelector creates a Selector with the given modes, defaulting to NORMAL.
func NewSelector(modes map[Mode]ModeConfig) *Selector {
	return &Selector{modes: modes, current: ModeNormal}
}

// Select manually selects a mode. Returns an error if the mode is unknown.
func (s *Selector) Select(mode Mode) error {
	if _, ok := s.modes[mode]; !ok {
		return fmt.Errorf("mode: unknown mode %q", mode)
	}
	s.current = mode
	return nil
}

// Current returns the current mode and its configuration.
func (s *Selector) Current() (Mode, ModeConfig) {
	return s.current, s.modes[s.current]
}

// List returns all registered modes sorted alphabetically by name.
func (s *Selector) List() []ModeConfig {
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
// Also updates the current mode.
func (s *Selector) SelectByComplexity(score int) Mode {
	var mode Mode
	switch {
	case score > 60:
		mode = ModeLongRunning
	case score >= 30:
		mode = ModeExploratory
	default:
		mode = ModeNormal
	}
	s.current = mode
	return mode
}

// Config returns the configuration for a specific mode.
func (s *Selector) Config(mode Mode) (ModeConfig, error) {
	cfg, ok := s.modes[mode]
	if !ok {
		return ModeConfig{}, fmt.Errorf("mode: unknown mode %q", mode)
	}
	return cfg, nil
}

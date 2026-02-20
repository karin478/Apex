// Package ratelimit implements token-bucket rate limiting with named groups.
package ratelimit

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// LimiterStatus is a snapshot of a limiter's current state.
type LimiterStatus struct {
	Name      string  `json:"name"`
	Rate      float64 `json:"rate"`
	Burst     int     `json:"burst"`
	Available float64 `json:"available"`
}

// Limiter implements a token bucket rate limiter.
type Limiter struct {
	name       string
	rate       float64 // tokens per second
	burst      int     // max bucket size
	tokens     float64 // current available tokens
	lastRefill time.Time
	mu         sync.Mutex
}

// NewLimiter creates a new token bucket Limiter. The bucket starts full
// (tokens = burst). Rate is specified in tokens per second.
func NewLimiter(name string, rate float64, burst int) *Limiter {
	return &Limiter{
		name:       name,
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		lastRefill: time.Now(),
	}
}

// Allow consumes one token and returns true, or returns false if no tokens
// are available. It is safe for concurrent use.
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill()
	if l.tokens >= 1 {
		l.tokens--
		return true
	}
	return false
}

// Wait blocks until a token is available or the context is cancelled.
// It returns nil on success or the context error on cancellation.
func (l *Limiter) Wait(ctx context.Context) error {
	for {
		if l.Allow() {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		sleepDur := time.Duration(float64(time.Second) / l.rate)
		if sleepDur > 100*time.Millisecond {
			sleepDur = 100 * time.Millisecond
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleepDur):
		}
	}
}

// Status returns a snapshot of the limiter's current state.
func (l *Limiter) Status() LimiterStatus {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill()
	return LimiterStatus{
		Name:      l.name,
		Rate:      l.rate,
		Burst:     l.burst,
		Available: l.tokens,
	}
}

// refill adds tokens based on elapsed time since the last refill.
// MUST be called with l.mu held.
func (l *Limiter) refill() {
	now := time.Now()
	elapsed := now.Sub(l.lastRefill)
	newTokens := elapsed.Seconds() * l.rate
	l.tokens += newTokens
	if l.tokens > float64(l.burst) {
		l.tokens = float64(l.burst)
	}
	l.lastRefill = now
}

// ---------------------------------------------------------------------------
// Group
// ---------------------------------------------------------------------------

// Group manages named rate limiters.
type Group struct {
	limiters map[string]*Limiter
	mu       sync.RWMutex
}

// NewGroup creates an empty Group.
func NewGroup() *Group {
	return &Group{
		limiters: make(map[string]*Limiter),
	}
}

// Add creates a new Limiter and registers it under the given name.
func (g *Group) Add(name string, rate float64, burst int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.limiters[name] = NewLimiter(name, rate, burst)
}

// Allow routes an Allow() call to the named limiter.
// Returns an error if the name is not found.
func (g *Group) Allow(name string) (bool, error) {
	g.mu.RLock()
	lim, ok := g.limiters[name]
	g.mu.RUnlock()
	if !ok {
		return false, fmt.Errorf("ratelimit: group not found: %s", name)
	}
	return lim.Allow(), nil
}

// Wait routes a Wait() call to the named limiter.
// Returns an error if the name is not found.
func (g *Group) Wait(name string, ctx context.Context) error {
	g.mu.RLock()
	lim, ok := g.limiters[name]
	g.mu.RUnlock()
	if !ok {
		return fmt.Errorf("ratelimit: group not found: %s", name)
	}
	return lim.Wait(ctx)
}

// Status returns a snapshot of all limiters, sorted by name.
func (g *Group) Status() []LimiterStatus {
	g.mu.RLock()
	defer g.mu.RUnlock()

	statuses := make([]LimiterStatus, 0, len(g.limiters))
	for _, lim := range g.limiters {
		statuses = append(statuses, lim.Status())
	}
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Name < statuses[j].Name
	})
	return statuses
}

// Remove deletes a limiter from the group.
func (g *Group) Remove(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.limiters, name)
}

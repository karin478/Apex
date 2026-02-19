package reasoning

import (
	"context"
	"sort"
	"sync"
)

// Protocol defines a registered reasoning protocol.
type Protocol struct {
	Name        string
	Description string
	Run         func(ctx context.Context, runner Runner, proposal string, progress ProgressFunc) (*ReviewResult, error)
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Protocol{}
)

// Register adds a reasoning protocol to the global registry.
func Register(p Protocol) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[p.Name] = p
}

// GetProtocol retrieves a protocol by name from the registry.
func GetProtocol(name string) (Protocol, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	p, ok := registry[name]
	return p, ok
}

// ListProtocols returns all registered protocols sorted by name.
func ListProtocols() []Protocol {
	registryMu.RLock()
	defer registryMu.RUnlock()
	var list []Protocol
	for _, p := range registry {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	return list
}

func clearRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = map[string]Protocol{}
}

func registerBuiltins() {
	Register(Protocol{
		Name:        "adversarial-review",
		Description: "4-step Advocate/Critic/Judge debate",
		Run: func(ctx context.Context, runner Runner, proposal string, progress ProgressFunc) (*ReviewResult, error) {
			return RunReviewWithProgress(ctx, runner, proposal, progress)
		},
	})
}

func init() {
	registerBuiltins()
}

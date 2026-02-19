package sandbox

import "context"

// NoneSandbox is a no-op passthrough — returns the command unchanged.
type NoneSandbox struct{}

func (n *NoneSandbox) Level() Level { return None }

func (n *NoneSandbox) Wrap(_ context.Context, binary string, args []string) (string, []string, error) {
	return binary, args, nil
}

package approval

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/lyndonlyu/apex/internal/dag"
	"github.com/lyndonlyu/apex/internal/governance"
)

type Decision int

const (
	Approved Decision = iota
	Skipped
	Rejected
)

func (d Decision) String() string {
	switch d {
	case Approved:
		return "approved"
	case Skipped:
		return "skipped"
	case Rejected:
		return "rejected"
	default:
		return "unknown"
	}
}

type NodeDecision struct {
	NodeID   string
	Decision Decision
}

type Result struct {
	Approved bool
	Nodes    []NodeDecision
}

type Reviewer struct {
	in  *bufio.Scanner
	out io.Writer
}

func NewReviewer(in io.Reader, out io.Writer) *Reviewer {
	return &Reviewer{
		in:  bufio.NewScanner(in),
		out: out,
	}
}

func (r *Reviewer) Review(nodes []*dag.Node, classify func(string) governance.RiskLevel) (*Result, error) {
	if len(nodes) == 0 {
		return &Result{Approved: true}, nil
	}

	// Display plan
	fmt.Fprintf(r.out, "\nApproval Required — %d steps planned\n", len(nodes))
	fmt.Fprintln(r.out, strings.Repeat("-", 44))
	for i, n := range nodes {
		risk := classify(n.Task)
		fmt.Fprintf(r.out, "  [%d] %-30s %s\n", i+1, n.Task, risk)
	}
	fmt.Fprintln(r.out, strings.Repeat("-", 44))
	fmt.Fprintf(r.out, "(a)pprove all / (r)eview one-by-one / (q)uit: ")

	choice := r.readLine()

	switch strings.ToLower(strings.TrimSpace(choice)) {
	case "a":
		return r.approveAll(nodes), nil
	case "r":
		return r.reviewOneByOne(nodes, classify), nil
	default:
		return r.rejectAll(nodes), nil
	}
}

func (r *Reviewer) readLine() string {
	if r.in.Scan() {
		return r.in.Text()
	}
	return ""
}

func (r *Reviewer) approveAll(nodes []*dag.Node) *Result {
	decisions := make([]NodeDecision, len(nodes))
	for i, n := range nodes {
		decisions[i] = NodeDecision{NodeID: n.ID, Decision: Approved}
	}
	return &Result{Approved: true, Nodes: decisions}
}

func (r *Reviewer) rejectAll(nodes []*dag.Node) *Result {
	decisions := make([]NodeDecision, len(nodes))
	for i, n := range nodes {
		decisions[i] = NodeDecision{NodeID: n.ID, Decision: Rejected}
	}
	return &Result{Approved: false, Nodes: decisions}
}

func (r *Reviewer) reviewOneByOne(nodes []*dag.Node, classify func(string) governance.RiskLevel) *Result {
	decisions := make([]NodeDecision, len(nodes))
	anyApproved := false

	for i, n := range nodes {
		risk := classify(n.Task)
		fmt.Fprintf(r.out, "\n[%d/%d] %s (%s)\n", i+1, len(nodes), n.Task, risk)
		fmt.Fprintf(r.out, "  (a)pprove / (s)kip / (r)eject all: ")

		choice := strings.ToLower(strings.TrimSpace(r.readLine()))

		switch choice {
		case "a":
			decisions[i] = NodeDecision{NodeID: n.ID, Decision: Approved}
			anyApproved = true
		case "s":
			decisions[i] = NodeDecision{NodeID: n.ID, Decision: Skipped}
		default: // "r" or anything else = reject all remaining
			decisions[i] = NodeDecision{NodeID: n.ID, Decision: Rejected}
			for j := i + 1; j < len(nodes); j++ {
				decisions[j] = NodeDecision{NodeID: nodes[j].ID, Decision: Rejected}
			}
			return &Result{Approved: false, Nodes: decisions}
		}
	}

	return &Result{Approved: anyApproved, Nodes: decisions}
}

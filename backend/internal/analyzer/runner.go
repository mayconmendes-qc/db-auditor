package analyzer

import (
	"context"
	"fmt"
)

// Runner executes all registered analyzers against snapshot facts.
type Runner struct {
	Registry *Registry
}

// NewRunner builds a runner with the given registry (defaults if nil).
func NewRunner(reg *Registry) *Runner {
	if reg == nil {
		reg = DefaultRegistry()
	}
	return &Runner{Registry: reg}
}

// Run executes every analyzer and concatenates findings.
func (r *Runner) Run(ctx context.Context, facts SnapshotFacts) ([]Finding, error) {
	all := make([]Finding, 0)
	for _, a := range r.Registry.All() {
		if err := ctx.Err(); err != nil {
			return all, err
		}
		items, err := a.Analyze(ctx, facts)
		if err != nil {
			return all, fmt.Errorf("analyzer %s: %w", a.Name(), err)
		}
		all = append(all, items...)
	}
	return EnrichFindings(facts, all), nil
}

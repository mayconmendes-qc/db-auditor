package audit

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Profile identifiers for collector inclusion.
const (
	ProfileFast    = "fast"
	ProfileDaily   = "daily"
	ProfileWeekly  = "weekly"
	ProfileMonthly = "monthly"
	ProfileManual  = "manual"
)

// CollectorFunc executes one collector step and returns rows collected.
type CollectorFunc func(ctx context.Context) (rows int64, err error)

// CollectorSpec describes a registered collector.
type CollectorSpec struct {
	Name     string
	Version  string
	Profiles map[string]struct{}
	Run      CollectorFunc
}

// Registry holds named collectors available to the AuditRunner.
type Registry struct {
	mu     sync.RWMutex
	byName map[string]CollectorSpec
}

// NewRegistry creates an empty collector registry.
func NewRegistry() *Registry {
	return &Registry{byName: make(map[string]CollectorSpec)}
}

// Register adds or replaces a collector. Name must be non-empty.
func (r *Registry) Register(spec CollectorSpec) error {
	if spec.Name == "" {
		return fmt.Errorf("collector name is required")
	}
	if spec.Version == "" {
		spec.Version = "1.0.0"
	}
	if spec.Profiles == nil {
		spec.Profiles = map[string]struct{}{ProfileManual: {}}
	}
	if spec.Run == nil {
		return fmt.Errorf("collector %s: Run func is required", spec.Name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byName[spec.Name] = spec
	return nil
}

// List returns collectors for a profile in stable name order.
func (r *Registry) List(profile string) []CollectorSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]CollectorSpec, 0)
	for _, spec := range r.byName {
		if profile == ProfileManual {
			out = append(out, spec)
			continue
		}
		if _, ok := spec.Profiles[profile]; ok {
			out = append(out, spec)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Names returns all registered collector names sorted.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.byName))
	for name := range r.byName {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// DefaultProfiles is the common profile set for full inventory collectors.
func DefaultProfiles() map[string]struct{} {
	return map[string]struct{}{
		ProfileFast:    {},
		ProfileDaily:   {},
		ProfileWeekly:  {},
		ProfileMonthly: {},
		ProfileManual:  {},
	}
}

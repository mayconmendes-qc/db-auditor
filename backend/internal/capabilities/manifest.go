// Package capabilities describes engine-neutral facts and the collectors that
// can provide them. A connector advertises capabilities before rules run.
package capabilities

import "strings"

type Fact struct {
	Kind       string  `json:"kind"`
	Scope      string  `json:"scope"`
	Numeric    float64 `json:"numeric"`
	Confidence float64 `json:"confidence"`
}

type Adapter interface {
	Engine() string
	Capabilities() []string
}

type Capability struct {
	Name       string `json:"name"`
	Applicable bool   `json:"applicable"`
	Reason     string `json:"reason,omitempty"`
}

var known = []string{"catalog", "constraints", "indexes", "workload", "security", "timescale", "data_quality"}

func Matrix(engine string, timescaleObserved bool, qualityEnabled bool) []Capability {
	if engine == "" {
		engine = "postgresql"
	}
	out := make([]Capability, 0, len(known))
	postgres := engine == "postgresql" || engine == "timescaledb"
	for _, name := range known {
		available := postgres || engine == "mongodb" && (name == "catalog" || name == "indexes")
		switch name {
		case "timescale":
			available = postgres && timescaleObserved
		case "data_quality":
			available = postgres && qualityEnabled
		}
		item := Capability{Name: name, Applicable: available}
		if !available {
			item.Reason = "Não aplicável ou desativado para este mecanismo e coleta."
		}
		out = append(out, item)
	}
	return out
}

func RuleApplicable(engine, rule string, timescaleObserved bool) bool {
	if engine == "" {
		engine = "postgresql"
	}
	if engine != "postgresql" && engine != "timescaledb" {
		return false
	}
	if strings.HasPrefix(rule, "chunk.") || strings.HasPrefix(rule, "cagg.") || strings.HasPrefix(rule, "policy.") || strings.HasPrefix(rule, "job.") || strings.HasPrefix(rule, "timescale.") {
		return timescaleObserved
	}
	return true
}

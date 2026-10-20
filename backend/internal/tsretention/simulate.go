// Package tsretention simulates TimescaleDB retention and compression savings
// from chunk snapshot metadata (spec 09). Pure calculation — no I/O.
package tsretention

import (
	"math"
	"sort"
	"time"
)

const (
	DefaultCompressionRatio = 5.0 // when no compressed chunks observed
	bytesPerGB              = 1024 * 1024 * 1024
)

// Chunk is one chunk observation used by the simulator.
type Chunk struct {
	Name                   string
	RangeStart             *time.Time
	RangeEnd               *time.Time
	RangeEndInteger        *int64 // non-nil ⇒ integer dimension (retention by age N/A)
	IsCompressed           bool
	TotalSizeBytes         int64
	BeforeCompressionBytes int64
	AfterCompressionBytes  int64
	DeadTuples             int64 // optional; >0 on recent chunks ⇒ write risk
}

// CAGGDep describes a continuous aggregate dependency for conflict checks.
type CAGGDep struct {
	ViewName          string
	RefreshWindowDays int // 0 = unknown / no refresh policy
	HasRefreshPolicy  bool
}

// HypertableState is the input for one hypertable simulation.
type HypertableState struct {
	Database, Schema, Name string
	Chunks                 []Chunk
	CAGGDeps               []CAGGDep
	InventoryTruncated     bool
	IntegerDimensionOnly   bool // no time/"now" function
	GrowthPerDayBytes      int64
	Now                    time.Time
}

// Scenario is one retention/compression what-if.
type Scenario struct {
	Label             string
	RetentionDays     int // 0 = no retention change
	CompressAfterDays int // 0 = no compression change
}

// AgeBucket aggregates chunk sizes by age band.
type AgeBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
	Bytes int64  `json:"bytes"`
}

// SimulationResult is the output of Simulate.
type SimulationResult struct {
	Partial             bool             `json:"partial"`
	RetentionApplicable bool             `json:"retention_applicable"`
	ObservedRatio       float64          `json:"observed_ratio"`
	RatioConfidenceLow  bool             `json:"ratio_confidence_low"`
	CurrentBytes        int64            `json:"current_bytes"`
	CompressedPct       float64          `json:"compressed_pct"`
	AgeBuckets          []AgeBucket      `json:"age_buckets"`
	Scenarios           []ScenarioResult `json:"scenarios"`
	Risks               []string         `json:"risks"`
}

// ScenarioResult is one scenario's projected savings.
type ScenarioResult struct {
	Label             string   `json:"label"`
	RetentionDays     int      `json:"retention_days"`
	CompressAfterDays int      `json:"compress_after_days"`
	BytesAfter        int64    `json:"bytes_after"`
	BytesSaved        int64    `json:"bytes_saved"`
	GBSaved           float64  `json:"gb_saved"`
	Risks             []string `json:"risks"`
	Projection30GB    float64  `json:"projection_30_gb"`
	Projection90GB    float64  `json:"projection_90_gb"`
	Projection180GB   float64  `json:"projection_180_gb"`
}

// ObservedCompressionRatio returns Σbefore/Σafter for compressed chunks with both sizes > 0.
func ObservedCompressionRatio(chunks []Chunk) (ratio float64, ok bool) {
	var before, after int64
	for _, c := range chunks {
		if !c.IsCompressed {
			continue
		}
		if c.BeforeCompressionBytes > 0 && c.AfterCompressionBytes > 0 {
			before += c.BeforeCompressionBytes
			after += c.AfterCompressionBytes
		}
	}
	if after <= 0 || before <= 0 {
		return DefaultCompressionRatio, false
	}
	return float64(before) / float64(after), true
}

// Simulate evaluates up to five scenarios against hypertable state.
func Simulate(ht HypertableState, scenarios []Scenario) SimulationResult {
	now := ht.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	out := SimulationResult{
		Partial:             ht.InventoryTruncated,
		RetentionApplicable: !ht.IntegerDimensionOnly,
	}
	ratio, observed := ObservedCompressionRatio(ht.Chunks)
	out.ObservedRatio = ratio
	out.RatioConfidenceLow = !observed

	var current, compressedBytes int64
	for _, c := range ht.Chunks {
		current += c.TotalSizeBytes
		if c.IsCompressed {
			compressedBytes += c.TotalSizeBytes
		}
	}
	out.CurrentBytes = current
	if current > 0 {
		out.CompressedPct = float64(compressedBytes) / float64(current) * 100
	}
	out.AgeBuckets = ageBuckets(ht.Chunks, now)

	if len(scenarios) > 5 {
		scenarios = scenarios[:5]
	}
	for _, sc := range scenarios {
		sr := evaluateScenario(ht, sc, ratio, observed, now, current)
		out.Scenarios = append(out.Scenarios, sr)
		out.Risks = mergeUnique(out.Risks, sr.Risks)
	}
	return out
}

func evaluateScenario(ht HypertableState, sc Scenario, ratio float64, observed bool, now time.Time, current int64) ScenarioResult {
	sr := ScenarioResult{
		Label:             sc.Label,
		RetentionDays:     sc.RetentionDays,
		CompressAfterDays: sc.CompressAfterDays,
	}
	if sc.RetentionDays < 0 || sc.CompressAfterDays < 0 {
		sr.Risks = append(sr.Risks, "invalid_scenario")
		sr.BytesAfter = current
		return sr
	}

	kept := make([]Chunk, 0, len(ht.Chunks))
	for _, c := range ht.Chunks {
		if sc.RetentionDays > 0 && ht.RetentionApplicable() {
			if chunkOlderThan(c, now, sc.RetentionDays) {
				continue // dropped by retention
			}
		}
		kept = append(kept, c)
	}

	var after int64
	for _, c := range kept {
		size := c.TotalSizeBytes
		if sc.CompressAfterDays > 0 && !c.IsCompressed && chunkOlderThan(c, now, sc.CompressAfterDays) {
			size = int64(math.Round(float64(size) / ratio))
		}
		after += size
	}
	sr.BytesAfter = after
	if current > after {
		sr.BytesSaved = current - after
	}
	sr.GBSaved = float64(sr.BytesSaved) / bytesPerGB

	if sc.RetentionDays > 0 {
		for _, dep := range ht.CAGGDeps {
			if dep.HasRefreshPolicy && dep.RefreshWindowDays > 0 && sc.RetentionDays < dep.RefreshWindowDays {
				sr.Risks = append(sr.Risks, "retention_cagg_conflict:"+dep.ViewName)
			}
			if !dep.HasRefreshPolicy {
				sr.Risks = append(sr.Risks, "retention_cagg_no_refresh:"+dep.ViewName)
			}
		}
	}
	if sc.CompressAfterDays > 0 {
		for _, c := range kept {
			if !c.IsCompressed && chunkOlderThan(c, now, sc.CompressAfterDays) && c.DeadTuples > 0 {
				sr.Risks = append(sr.Risks, "compress_active_write:"+c.Name)
			}
		}
		if !observed {
			sr.Risks = append(sr.Risks, "low_confidence_ratio")
		}
	}

	growth := ht.GrowthPerDayBytes
	baseGB := float64(after) / bytesPerGB
	sr.Projection30GB = baseGB + float64(growth*30)/bytesPerGB
	sr.Projection90GB = baseGB + float64(growth*90)/bytesPerGB
	sr.Projection180GB = baseGB + float64(growth*180)/bytesPerGB
	return sr
}

func (ht HypertableState) RetentionApplicable() bool {
	return !ht.IntegerDimensionOnly
}

func chunkOlderThan(c Chunk, now time.Time, days int) bool {
	if c.RangeEndInteger != nil {
		return false // age N/A for integer dimensions
	}
	if c.RangeEnd == nil {
		return false
	}
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	return c.RangeEnd.Before(cutoff) || c.RangeEnd.Equal(cutoff)
}

func ageBuckets(chunks []Chunk, now time.Time) []AgeBucket {
	bands := []struct {
		label string
		maxD  int
	}{
		{"0-7d", 7},
		{"7-30d", 30},
		{"30-90d", 90},
		{"90-365d", 365},
		{">365d", 0},
	}
	out := make([]AgeBucket, len(bands))
	for i, b := range bands {
		out[i].Label = b.label
	}
	for _, c := range chunks {
		ageDays := 0
		if c.RangeEnd != nil {
			ageDays = int(now.Sub(*c.RangeEnd).Hours() / 24)
			if ageDays < 0 {
				ageDays = 0
			}
		}
		idx := len(bands) - 1
		for i, b := range bands {
			if b.maxD > 0 && ageDays <= b.maxD {
				idx = i
				break
			}
		}
		out[idx].Count++
		out[idx].Bytes += c.TotalSizeBytes
	}
	return out
}

func mergeUnique(a, b []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, xs := range [][]string{a, b} {
		for _, x := range xs {
			if _, ok := seen[x]; ok {
				continue
			}
			seen[x] = struct{}{}
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

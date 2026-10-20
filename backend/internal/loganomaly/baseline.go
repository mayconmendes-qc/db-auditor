package loganomaly

import (
	"math"
	"time"
)

// TemplateStats is mean/stddev of counts across baseline windows.
type TemplateStats struct {
	Mean         float64
	StdDev       float64
	WindowsWith  int
	TotalWindows int
	Present      bool
}

// LevelStats is mean of a level bucket across baseline windows.
type LevelStats struct {
	Mean   float64
	StdDev float64
}

// Baseline holds per-template and per-level stats over K windows.
type Baseline struct {
	Templates    map[string]TemplateStats
	Levels       map[string]LevelStats
	WindowCount  int
	Insufficient bool
	Patterns     map[string]string
	From         time.Time
	To           time.Time
}

// SplitWindows selects baseline and analysis windows.
// Default: baseline = up to K windows immediately before the last window;
// analysis = last window. With from/to, baseline is windows in [from,to),
// analysis is windows at/after to (or after the baseline block).
func SplitWindows(windows []*WindowCounts, k int, from, to time.Time) (base, analysis []*WindowCounts) {
	if k <= 0 {
		k = 12
	}
	if len(windows) == 0 {
		return nil, nil
	}
	if !from.IsZero() || !to.IsZero() {
		for _, w := range windows {
			inBase := true
			if !from.IsZero() && w.Start.Before(from) {
				inBase = false
			}
			if !to.IsZero() && !w.Start.Before(to) {
				inBase = false
			}
			if inBase {
				base = append(base, w)
			} else if !to.IsZero() && !w.Start.Before(to) {
				analysis = append(analysis, w)
			}
		}
		if len(analysis) == 0 && len(base) > 0 {
			last := base[len(base)-1].Start
			for _, w := range windows {
				if w.Start.After(last) {
					analysis = append(analysis, w)
				}
			}
		}
		return base, analysis
	}
	if len(windows) == 1 {
		return nil, windows
	}
	// Baseline: up to K windows before the last; analysis: last window.
	prior := windows[:len(windows)-1]
	if len(prior) > k {
		prior = prior[len(prior)-k:]
	}
	return prior, windows[len(windows)-1:]
}

// ComputeBaseline builds stats from base windows.
func ComputeBaseline(base []*WindowCounts, minWins int) Baseline {
	if minWins <= 0 {
		minWins = 6
	}
	b := Baseline{
		Templates:    make(map[string]TemplateStats),
		Levels:       make(map[string]LevelStats),
		WindowCount:  len(base),
		Insufficient: len(base) < minWins,
		Patterns:     make(map[string]string),
	}
	if len(base) == 0 {
		return b
	}
	b.From = base[0].Start
	b.To = base[len(base)-1].Start

	ids := map[string]bool{}
	for _, w := range base {
		for id, pat := range w.Templates {
			ids[id] = true
			b.Patterns[id] = pat
		}
		for id := range w.ByTemplate {
			ids[id] = true
		}
	}
	n := float64(len(base))
	for id := range ids {
		var sum, sumSq float64
		var with int
		for _, w := range base {
			c := float64(w.ByTemplate[id])
			sum += c
			sumSq += c * c
			if c > 0 {
				with++
			}
		}
		mean := sum / n
		vari := sumSq/n - mean*mean
		if vari < 0 {
			vari = 0
		}
		b.Templates[id] = TemplateStats{
			Mean:         mean,
			StdDev:       math.Sqrt(vari),
			WindowsWith:  with,
			TotalWindows: len(base),
			Present:      with > 0,
		}
	}
	for _, lvl := range []string{LevelError, LevelWarning, LevelInfo} {
		var sum, sumSq float64
		for _, w := range base {
			c := float64(w.ByLevel[lvl])
			sum += c
			sumSq += c * c
		}
		mean := sum / n
		vari := sumSq/n - mean*mean
		if vari < 0 {
			vari = 0
		}
		b.Levels[lvl] = LevelStats{Mean: mean, StdDev: math.Sqrt(vari)}
	}
	return b
}

// ZScore returns (count-mean)/stddev.
func ZScore(count int, st TemplateStats) float64 {
	if st.StdDev > 1e-9 {
		return (float64(count) - st.Mean) / st.StdDev
	}
	if float64(count) == st.Mean {
		return 0
	}
	if st.Mean == 0 {
		return float64(count)
	}
	return (float64(count) - st.Mean) / math.Max(st.Mean*0.1, 1)
}

// Factor returns count/mean (or count if mean≈0).
func Factor(count int, mean float64) float64 {
	if mean < 1e-9 {
		if count == 0 {
			return 0
		}
		return float64(count)
	}
	return float64(count) / mean
}

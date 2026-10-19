package loganomaly

import (
	"math"
	"sort"
	"strings"
	"time"
)

// Anomaly is one detected finding (masked template only).
type Anomaly struct {
	Kind        string    `json:"kind"`
	Severity    string    `json:"severity"`
	Confidence  string    `json:"confidence"`
	TemplateID  string    `json:"template_id,omitempty"`
	Template    string    `json:"template,omitempty"`
	Level       string    `json:"level,omitempty"`
	WindowStart time.Time `json:"window_start"`
	Observed    int       `json:"observed"`
	Expected    float64   `json:"expected"`
	Factor      float64   `json:"factor,omitempty"`
	ZScore      float64   `json:"z_score,omitempty"`
	Detail      string    `json:"detail,omitempty"`
}

// DetectAnomalies applies spec §3 rules.
func DetectAnomalies(all []*WindowCounts, base Baseline, analysis []*WindowCounts, th Thresholds, levelByTemplate map[string]string) []Anomaly {
	var out []Anomaly
	if th.StrongFactor == 0 {
		th = DefaultThresholds()
	}

	for _, w := range analysis {
		// Per-template spikes / new templates.
		for id, count := range w.ByTemplate {
			if id == UnclassifiableID {
				continue
			}
			pat := w.Templates[id]
			if pat == "" {
				pat = base.Patterns[id]
			}
			lvl := levelByTemplate[id]
			st, known := base.Templates[id]

			if base.Insufficient {
				// Only new templates, low confidence.
				if !known || !st.Present {
					if count >= th.NewMinCount {
						sev := SeverityLow
						if isErrorLevel(lvl) {
							sev = SeverityMedium
						}
						out = append(out, Anomaly{
							Kind: AnomalyNewTemplate, Severity: sev, Confidence: ConfidenceLow,
							TemplateID: id, Template: pat, Level: lvl,
							WindowStart: w.Start, Observed: count, Expected: 0,
							Detail: "base insuficiente; confiança baixa",
						})
					}
				}
				continue
			}

			if !known || !st.Present {
				if count >= th.NewMinCount {
					sev := SeverityLow
					if isErrorLevel(lvl) {
						sev = SeverityMedium
					}
					out = append(out, Anomaly{
						Kind: AnomalyNewTemplate, Severity: sev, Confidence: ConfidenceMedium,
						TemplateID: id, Template: pat, Level: lvl,
						WindowStart: w.Start, Observed: count, Expected: 0,
					})
				}
				continue
			}

			z := ZScore(count, st)
			fac := Factor(count, st.Mean)

			// Strong spike first (more specific).
			if fac >= th.StrongFactor && count >= th.StrongMinCount {
				sev := SeverityMedium
				if isErrorLevel(lvl) || isFatalRaw(lvl) {
					sev = SeverityHigh
				}
				out = append(out, Anomaly{
					Kind: AnomalyStrongSpike, Severity: sev, Confidence: ConfidenceHigh,
					TemplateID: id, Template: pat, Level: lvl,
					WindowStart: w.Start, Observed: count, Expected: round2(st.Mean),
					Factor: round2(fac), ZScore: round2(z),
				})
				continue
			}
			if z >= th.SpikeZ && count >= th.SpikeMinCount {
				sev := SeverityLow
				if isErrorLevel(lvl) {
					sev = SeverityMedium
				}
				out = append(out, Anomaly{
					Kind: AnomalySpike, Severity: sev, Confidence: ConfidenceHigh,
					TemplateID: id, Template: pat, Level: lvl,
					WindowStart: w.Start, Observed: count, Expected: round2(st.Mean),
					Factor: round2(fac), ZScore: round2(z),
				})
			}
		}

		// Error volume spike for this window.
		if !base.Insufficient {
			errCount := w.ByLevel[LevelError]
			ls := base.Levels[LevelError]
			fac := Factor(errCount, ls.Mean)
			if fac >= th.ErrorSpikeFactor && errCount > 0 {
				out = append(out, Anomaly{
					Kind: AnomalyErrorSpike, Severity: SeverityHigh, Confidence: ConfidenceHigh,
					Level: LevelError, WindowStart: w.Start,
					Observed: errCount, Expected: round2(ls.Mean), Factor: round2(fac),
				})
			}
		}
	}

	// Missing templates: present in ≥50% baseline windows, absent in last 3 overall.
	if !base.Insufficient && len(all) >= th.MissingAbsentN {
		lastN := all[len(all)-th.MissingAbsentN:]
		for id, st := range base.Templates {
			if !st.Present || st.TotalWindows == 0 {
				continue
			}
			frac := float64(st.WindowsWith) / float64(st.TotalWindows)
			if frac < th.MissingBaseFrac {
				continue
			}
			absent := true
			for _, w := range lastN {
				if w.ByTemplate[id] > 0 {
					absent = false
					break
				}
			}
			if !absent {
				continue
			}
			pat := base.Patterns[id]
			out = append(out, Anomaly{
				Kind: AnomalyMissing, Severity: SeverityInfo, Confidence: ConfidenceMedium,
				TemplateID: id, Template: pat,
				WindowStart: lastN[0].Start, Observed: 0, Expected: round2(st.Mean),
				Detail: "ausente nas últimas 3 janelas",
			})
		}
	}

	sortAnomalies(out)
	return out
}

func isErrorLevel(lvl string) bool {
	return strings.EqualFold(lvl, LevelError)
}

func isFatalRaw(lvl string) bool {
	u := strings.ToUpper(lvl)
	return u == "FATAL" || u == "PANIC" || strings.Contains(u, "FATAL") || strings.Contains(u, "PANIC")
}

func round2(f float64) float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return math.Round(f*100) / 100
}

func sortAnomalies(a []Anomaly) {
	rank := map[string]int{
		SeverityCritical: 5, SeverityHigh: 4, SeverityMedium: 3, SeverityLow: 2, SeverityInfo: 1,
	}
	sort.Slice(a, func(i, j int) bool {
		ri, rj := rank[a[i].Severity], rank[a[j].Severity]
		if ri != rj {
			return ri > rj
		}
		if !a[i].WindowStart.Equal(a[j].WindowStart) {
			return a[i].WindowStart.Before(a[j].WindowStart)
		}
		if a[i].Kind != a[j].Kind {
			return a[i].Kind < a[j].Kind
		}
		return a[i].TemplateID < a[j].TemplateID
	})
}

// GateExit maps anomalies + stats to CLI exit codes.
func GateExit(anomalies []Anomaly, failOn string, stats ReadStats, baselineInsufficient bool, wantPeaks bool) int {
	if stats.LinesRead > 0 && stats.InvalidRatio() > InvalidLineRatio {
		return 3
	}
	if baselineInsufficient && wantPeaks {
		return 3
	}
	threshold := severityRank(failOn)
	if threshold == 0 {
		threshold = severityRank(SeverityHigh)
	}
	for _, a := range anomalies {
		if severityRank(a.Severity) >= threshold {
			return 1
		}
	}
	return 0
}

func severityRank(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case SeverityInfo:
		return 1
	case SeverityLow:
		return 2
	case SeverityMedium:
		return 3
	case SeverityHigh:
		return 4
	case SeverityCritical:
		return 5
	default:
		return 0
	}
}

package digest

import "sort"

// Classification labels for a fingerprint across two windows.
const (
	ClassRegression  = "regressao"
	ClassNew         = "nova"
	ClassGone        = "sumiu"
	ClassImprovement = "melhora"
	ClassStable      = "estavel"
	SeverityMedium   = "media"
	SeverityHigh     = "alta"
	SeverityInfo     = "informativa"
)

// Thresholds are the initial calibratable cutoffs (spec §3). Zero values use defaults.
type Thresholds struct {
	RegressionMeanPct  float64 // +50%
	RegressionAbsMS    float64 // +10 ms
	MinCalls           int64   // ≥100 each window
	HighTotalPct       float64 // +100% total time
	HighShareOfDB      float64 // ≥5% of DB total
	NewShareOfCurrent  float64 // ≥5% of current window total
	GoneShareOfPrior   float64 // ≥1% of prior total
	ImprovementMeanPct float64 // −30%
	TopRegressions     int
	TopNew             int
	TopGone            int
	TopImprovements    int
}

// DefaultThresholds returns the documented initial values.
func DefaultThresholds() Thresholds {
	return Thresholds{
		RegressionMeanPct:  0.50,
		RegressionAbsMS:    10,
		MinCalls:           100,
		HighTotalPct:       1.00,
		HighShareOfDB:      0.05,
		NewShareOfCurrent:  0.05,
		GoneShareOfPrior:   0.01,
		ImprovementMeanPct: 0.30,
		TopRegressions:     10,
		TopNew:             10,
		TopGone:            5,
		TopImprovements:    5,
	}
}

func (t Thresholds) withDefaults() Thresholds {
	d := DefaultThresholds()
	if t.RegressionMeanPct <= 0 {
		t.RegressionMeanPct = d.RegressionMeanPct
	}
	if t.RegressionAbsMS <= 0 {
		t.RegressionAbsMS = d.RegressionAbsMS
	}
	if t.MinCalls <= 0 {
		t.MinCalls = d.MinCalls
	}
	if t.HighTotalPct <= 0 {
		t.HighTotalPct = d.HighTotalPct
	}
	if t.HighShareOfDB <= 0 {
		t.HighShareOfDB = d.HighShareOfDB
	}
	if t.NewShareOfCurrent <= 0 {
		t.NewShareOfCurrent = d.NewShareOfCurrent
	}
	if t.GoneShareOfPrior <= 0 {
		t.GoneShareOfPrior = d.GoneShareOfPrior
	}
	if t.ImprovementMeanPct <= 0 {
		t.ImprovementMeanPct = d.ImprovementMeanPct
	}
	if t.TopRegressions <= 0 {
		t.TopRegressions = d.TopRegressions
	}
	if t.TopNew <= 0 {
		t.TopNew = d.TopNew
	}
	if t.TopGone <= 0 {
		t.TopGone = d.TopGone
	}
	if t.TopImprovements <= 0 {
		t.TopImprovements = d.TopImprovements
	}
	return t
}

// ClassifiedItem is one fingerprint after comparing prior and current windows.
type ClassifiedItem struct {
	Classification    string
	Severity          string
	Fingerprint       string
	QueryID           string
	DatabaseName      string
	ReferencedObjects []string
	PriorCalls        int64
	CurrentCalls      int64
	PriorMeanMS       float64
	CurrentMeanMS     float64
	PriorTotalMS      float64
	CurrentTotalMS    float64
	MeanChangePct     float64 // (current-prior)/prior; 0 when prior mean is 0
	TotalChangePct    float64
	PriorShare        float64
	CurrentShare      float64
}

// CompareWindows classifies fingerprints present in either window.
type fpKey struct{ db, fp string }

func CompareWindows(prior, current WindowResult, th Thresholds) []ClassifiedItem {
	th = th.withDefaults()
	priorBy := indexActiveDeltas(prior.Deltas)
	currBy := indexActiveDeltas(current.Deltas)

	keys := map[fpKey]struct{}{}
	for k := range priorBy {
		keys[k] = struct{}{}
	}
	for k := range currBy {
		keys[k] = struct{}{}
	}
	out := make([]ClassifiedItem, 0, len(keys))
	for k := range keys {
		p, hasP := priorBy[k]
		c, hasC := currBy[k]
		item := ClassifiedItem{
			Fingerprint:  k.fp,
			DatabaseName: k.db,
		}
		if hasC {
			item.QueryID = c.QueryID
			item.ReferencedObjects = append([]string(nil), c.ReferencedObjects...)
			item.CurrentCalls = windowCalls(c)
			item.CurrentMeanMS = windowMean(c)
			item.CurrentTotalMS = windowTotal(c)
		}
		if hasP {
			if item.QueryID == "" {
				item.QueryID = p.QueryID
			}
			if len(item.ReferencedObjects) == 0 {
				item.ReferencedObjects = append([]string(nil), p.ReferencedObjects...)
			}
			item.PriorCalls = windowCalls(p)
			item.PriorMeanMS = windowMean(p)
			item.PriorTotalMS = windowTotal(p)
		}
		if prior.TotalTimeMS > 0 {
			item.PriorShare = item.PriorTotalMS / prior.TotalTimeMS
		}
		if current.TotalTimeMS > 0 {
			item.CurrentShare = item.CurrentTotalMS / current.TotalTimeMS
		}
		if item.PriorMeanMS > 0 {
			item.MeanChangePct = (item.CurrentMeanMS - item.PriorMeanMS) / item.PriorMeanMS
		}
		if item.PriorTotalMS > 0 {
			item.TotalChangePct = (item.CurrentTotalMS - item.PriorTotalMS) / item.PriorTotalMS
		}
		item.Classification, item.Severity = classifyOne(item, hasP, hasC, th, current.TotalTimeMS)
		out = append(out, item)
	}
	return out
}

func indexActiveDeltas(deltas []WindowDelta) map[fpKey]WindowDelta {
	out := map[fpKey]WindowDelta{}
	for _, d := range deltas {
		k := fpKey{d.DatabaseName, d.Fingerprint}
		// Gone candidates: only on the first edge of the window.
		if d.FirstRunID != "" && d.LastCalls == 0 && d.FirstCalls > 0 && d.DeltaCalls == 0 && d.DeltaTotalMS == 0 {
			out[k] = d
			continue
		}
		if windowCalls(d) > 0 || windowTotal(d) > 0 || d.MeanMS > 0 {
			out[k] = d
		}
	}
	return out
}

func windowCalls(d WindowDelta) int64 {
	if d.FirstRunID != "" && d.LastRunID != "" && d.DeltaCalls > 0 {
		return d.DeltaCalls
	}
	if d.FirstRunID != "" && d.LastCalls == 0 && d.FirstCalls > 0 {
		return d.FirstCalls
	}
	if d.DeltaCalls > 0 {
		return d.DeltaCalls
	}
	if d.LastCalls > 0 && d.FirstRunID == "" {
		return d.LastCalls
	}
	if d.DeltaCalls < 0 {
		// Counter went backwards without detected reset — treat as unstable, zero.
		return 0
	}
	return d.DeltaCalls
}

func windowTotal(d WindowDelta) float64 {
	if d.FirstRunID != "" && d.LastRunID != "" && d.DeltaTotalMS > 0 {
		return d.DeltaTotalMS
	}
	if d.FirstRunID != "" && d.LastTotalMS == 0 && d.FirstTotalMS > 0 {
		return d.FirstTotalMS
	}
	if d.DeltaTotalMS > 0 {
		return d.DeltaTotalMS
	}
	if d.LastTotalMS > 0 && d.FirstRunID == "" {
		return d.LastTotalMS
	}
	return d.DeltaTotalMS
}

func windowMean(d WindowDelta) float64 {
	calls := windowCalls(d)
	total := windowTotal(d)
	if calls > 0 && total > 0 {
		return total / float64(calls)
	}
	if d.MeanMS > 0 {
		return d.MeanMS
	}
	if d.LastMeanMS > 0 && d.FirstRunID == "" {
		return d.LastMeanMS
	}
	if d.FirstMeanMS > 0 && d.LastCalls == 0 {
		return d.FirstMeanMS
	}
	return 0
}

func classifyOne(item ClassifiedItem, hasP, hasC bool, th Thresholds, currentTotal float64) (class, severity string) {
	switch {
	case hasC && !hasP:
		if currentTotal > 0 && item.CurrentTotalMS/currentTotal >= th.NewShareOfCurrent {
			return ClassNew, SeverityMedium
		}
		return ClassStable, SeverityInfo
	case hasP && !hasC:
		if item.PriorShare >= th.GoneShareOfPrior {
			return ClassGone, SeverityInfo
		}
		return ClassStable, SeverityInfo
	case hasP && hasC:
		enoughCalls := item.PriorCalls >= th.MinCalls && item.CurrentCalls >= th.MinCalls
		if enoughCalls && item.PriorMeanMS > 0 {
			abs := item.CurrentMeanMS - item.PriorMeanMS
			if item.MeanChangePct >= th.RegressionMeanPct && abs >= th.RegressionAbsMS {
				sev := SeverityMedium
				if item.TotalChangePct >= th.HighTotalPct && item.CurrentShare >= th.HighShareOfDB {
					sev = SeverityHigh
				}
				return ClassRegression, sev
			}
			if item.MeanChangePct <= -th.ImprovementMeanPct {
				return ClassImprovement, SeverityInfo
			}
		}
		return ClassStable, SeverityInfo
	default:
		return ClassStable, SeverityInfo
	}
}

// DigestSections holds the capped tops for RF-04.
type DigestSections struct {
	Regressions  []ClassifiedItem
	NewQueries   []ClassifiedItem
	GoneQueries  []ClassifiedItem
	Improvements []ClassifiedItem
}

// SelectTops keeps the top N items per classification (RF-04).
func SelectTops(items []ClassifiedItem, th Thresholds) DigestSections {
	th = th.withDefaults()
	var reg, neu, gone, imp []ClassifiedItem
	for _, it := range items {
		switch it.Classification {
		case ClassRegression:
			reg = append(reg, it)
		case ClassNew:
			neu = append(neu, it)
		case ClassGone:
			gone = append(gone, it)
		case ClassImprovement:
			imp = append(imp, it)
		}
	}
	sort.Slice(reg, func(i, j int) bool {
		if reg[i].MeanChangePct != reg[j].MeanChangePct {
			return reg[i].MeanChangePct > reg[j].MeanChangePct
		}
		return reg[i].CurrentTotalMS > reg[j].CurrentTotalMS
	})
	sort.Slice(neu, func(i, j int) bool { return neu[i].CurrentTotalMS > neu[j].CurrentTotalMS })
	sort.Slice(gone, func(i, j int) bool { return gone[i].PriorTotalMS > gone[j].PriorTotalMS })
	sort.Slice(imp, func(i, j int) bool { return imp[i].MeanChangePct < imp[j].MeanChangePct })
	return DigestSections{
		Regressions:  clip(reg, th.TopRegressions),
		NewQueries:   clip(neu, th.TopNew),
		GoneQueries:  clip(gone, th.TopGone),
		Improvements: clip(imp, th.TopImprovements),
	}
}

func clip(items []ClassifiedItem, n int) []ClassifiedItem {
	if n <= 0 || len(items) <= n {
		return items
	}
	return items[:n]
}

// WindowFromMeans builds a synthetic WindowResult for tests (CA-01 style).
func WindowFromMeans(items []ClassifiedItem, asCurrent bool) WindowResult {
	var deltas []WindowDelta
	var total float64
	for _, it := range items {
		d := WindowDelta{
			Fingerprint:       it.Fingerprint,
			QueryID:           it.QueryID,
			DatabaseName:      it.DatabaseName,
			ReferencedObjects: it.ReferencedObjects,
		}
		if asCurrent {
			d.DeltaCalls = it.CurrentCalls
			d.MeanMS = it.CurrentMeanMS
			d.DeltaTotalMS = it.CurrentMeanMS * float64(it.CurrentCalls)
			d.LastCalls = it.CurrentCalls
			d.LastTotalMS = d.DeltaTotalMS
			d.LastMeanMS = it.CurrentMeanMS
			d.LastRunID = "current"
			d.FirstRunID = "current-first"
		} else {
			d.DeltaCalls = it.PriorCalls
			d.MeanMS = it.PriorMeanMS
			d.DeltaTotalMS = it.PriorMeanMS * float64(it.PriorCalls)
			d.LastCalls = it.PriorCalls
			d.LastTotalMS = d.DeltaTotalMS
			d.LastMeanMS = it.PriorMeanMS
			d.LastRunID = "prior"
			d.FirstRunID = "prior-first"
		}
		total += d.DeltaTotalMS
		deltas = append(deltas, d)
	}
	return WindowResult{OK: true, Deltas: deltas, TotalTimeMS: total, FingerprintCount: len(deltas)}
}

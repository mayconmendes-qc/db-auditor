package tsretention

import "time"

// ApproximateUnscannedLabel is the mandatory ADR-0001 D9 disclaimer.
const ApproximateUnscannedLabel = "aproximado — contadores zeram com reset"

// ChunkScanObservation is one chunk's scan/write counters at a collection.
type ChunkScanObservation struct {
	Name       string
	SeqScan    *int64 // nil = not collected (legacy snapshot)
	IdxScan    *int64
	NTupIns    *int64
	NTupUpd    *int64
	NTupDel    *int64
	StatsReset *time.Time
}

// UnscannedResult is the RF-07 proxy outcome between two complete collections.
type UnscannedResult struct {
	Comparable   bool
	Reason       string // empty when comparable; else e.g. "stats_reset", "not_collected", "no_previous"
	Approximate  bool   // always true when listing candidates
	Label        string
	Unscanned    []string // chunk names with zero scan delta and no writes
	NotCollected []string
}

// FindUnscannedChunks compares previous→current counters (ADR-0001 D9).
// A stats_reset between the two points invalidates that chunk (not "unscanned").
// Missing counters on either side are "não coletado", never "sem consulta".
func FindUnscannedChunks(previous, current []ChunkScanObservation) UnscannedResult {
	out := UnscannedResult{
		Comparable:  true,
		Approximate: true,
		Label:       ApproximateUnscannedLabel,
	}
	if len(current) == 0 {
		out.Comparable = false
		out.Reason = "no_current"
		return out
	}
	if len(previous) == 0 {
		out.Comparable = false
		out.Reason = "no_previous"
		return out
	}
	prev := map[string]ChunkScanObservation{}
	for _, p := range previous {
		prev[p.Name] = p
	}
	for _, c := range current {
		p, ok := prev[c.Name]
		if !ok {
			continue
		}
		if c.SeqScan == nil || c.IdxScan == nil || p.SeqScan == nil || p.IdxScan == nil {
			out.NotCollected = append(out.NotCollected, c.Name)
			continue
		}
		if statsResetBetween(p.StatsReset, c.StatsReset) {
			out.Comparable = false
			if out.Reason == "" {
				out.Reason = "stats_reset"
			}
			continue
		}
		seqDelta := *c.SeqScan - *p.SeqScan
		idxDelta := *c.IdxScan - *p.IdxScan
		if seqDelta < 0 || idxDelta < 0 {
			// Counter went backwards without an observed reset → not comparable.
			out.Comparable = false
			if out.Reason == "" {
				out.Reason = "counter_regression"
			}
			continue
		}
		wrote := false
		if c.NTupIns != nil && p.NTupIns != nil && *c.NTupIns > *p.NTupIns {
			wrote = true
		}
		if c.NTupUpd != nil && p.NTupUpd != nil && *c.NTupUpd > *p.NTupUpd {
			wrote = true
		}
		if c.NTupDel != nil && p.NTupDel != nil && *c.NTupDel > *p.NTupDel {
			wrote = true
		}
		if seqDelta == 0 && idxDelta == 0 && !wrote {
			out.Unscanned = append(out.Unscanned, c.Name)
		}
	}
	return out
}

func statsResetBetween(prev, cur *time.Time) bool {
	if cur == nil {
		return false
	}
	if prev == nil {
		return true
	}
	return cur.After(*prev)
}

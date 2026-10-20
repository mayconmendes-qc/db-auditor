// Package digest computes workload window deltas and classifications for the
// slow-query digest (spec 06). Pure functions over snapshot points; no I/O.
package digest

import (
	"time"
)

// StatsResetChanged mirrors repository.StatsResetChanged (AN-11 / #128):
// NULL→value and value→NULL both count as a reset. Duplicated here to keep
// digest free of an import cycle with repository.
func StatsResetChanged(prior, current *time.Time) bool {
	if prior == nil || current == nil {
		return prior != current
	}
	return !prior.Equal(*current)
}

// CollectionPoint is one complete collection edge used as a window bound.
type CollectionPoint struct {
	AuditRunID     string
	StartedAt      time.Time
	Completeness   string // must be "complete" for window edges
	RunStatus      string // audit_run.status; partial_success never edges
	AnalysisStatus string
	StatsReset     *time.Time // pg_stat_database / table counters
	WorkloadReset  *time.Time // pg_stat_statements
}

// WorkloadPoint is one fingerprint observation in a collection.
type WorkloadPoint struct {
	AuditRunID        string
	DatabaseName      string
	Fingerprint       string
	QueryID           string
	Calls             int64
	TotalExecTimeMS   float64
	MeanExecTimeMS    float64
	RowsTotal         int64
	SharedBlocksRead  int64
	ReferencedObjects []string
	StatsReset        *time.Time
	CollectedAt       time.Time
}

// WindowDelta is the counter delta for one fingerprint between two edges.
type WindowDelta struct {
	Fingerprint       string
	QueryID           string
	DatabaseName      string
	ReferencedObjects []string
	DeltaCalls        int64
	DeltaTotalMS      float64
	MeanMS            float64 // DeltaTotalMS / DeltaCalls when DeltaCalls > 0
	DeltaRows         int64
	DeltaBlocksRead   int64
	FirstRunID        string
	LastRunID         string
	FirstCalls        int64
	LastCalls         int64
	FirstTotalMS      float64
	LastTotalMS       float64
	FirstMeanMS       float64
	LastMeanMS        float64
}

// WindowResult describes a resolved comparable window (or why none exists).
type WindowResult struct {
	OK               bool
	Reason           string // empty when OK; else human-readable cause
	First            CollectionPoint
	Last             CollectionPoint
	UsedSubwindow    bool
	SubwindowNote    string
	Points           []CollectionPoint // ordered ascending by StartedAt
	Deltas           []WindowDelta
	TotalTimeMS      float64 // sum of LastTotalMS across fingerprints in this window
	FingerprintCount int
}

// IsCompleteEdge reports whether a collection may bound a digest window.
// partial_success never serves as an edge (CA-03).
func IsCompleteEdge(p CollectionPoint) bool {
	return p.Completeness == "complete" &&
		p.RunStatus == "success" &&
		p.AnalysisStatus == "success"
}

// SelectWindowEdges picks the first and last complete edges in points (already
// filtered and sorted ascending). Returns false when fewer than two edges exist.
func SelectWindowEdges(points []CollectionPoint) (first, last CollectionPoint, ok bool) {
	var edges []CollectionPoint
	for _, p := range points {
		if IsCompleteEdge(p) {
			edges = append(edges, p)
		}
	}
	if len(edges) < 2 {
		return CollectionPoint{}, CollectionPoint{}, false
	}
	return edges[0], edges[len(edges)-1], true
}

// LargestContiguousSubwindowWithoutReset finds the longest contiguous stretch
// of complete edges with no stats_reset / workload_reset change between
// consecutive edges (including NULL→value). Prefer the latest such stretch on ties.
func LargestContiguousSubwindowWithoutReset(points []CollectionPoint) (first, last CollectionPoint, used []CollectionPoint, ok bool) {
	var edges []CollectionPoint
	for _, p := range points {
		if IsCompleteEdge(p) {
			edges = append(edges, p)
		}
	}
	if len(edges) < 2 {
		return CollectionPoint{}, CollectionPoint{}, nil, false
	}
	bestStart, bestEnd := 0, 0
	segStart := 0
	for i := 1; i < len(edges); i++ {
		reset := StatsResetChanged(edges[i-1].StatsReset, edges[i].StatsReset) ||
			StatsResetChanged(edges[i-1].WorkloadReset, edges[i].WorkloadReset)
		if reset {
			if i-1-segStart > bestEnd-bestStart || (i-1-segStart == bestEnd-bestStart && i-1 > bestEnd) {
				bestStart, bestEnd = segStart, i-1
			}
			segStart = i
		}
	}
	if len(edges)-1-segStart > bestEnd-bestStart || (len(edges)-1-segStart == bestEnd-bestStart && len(edges)-1 > bestEnd) {
		bestStart, bestEnd = segStart, len(edges)-1
	}
	if bestEnd-bestStart < 1 {
		return CollectionPoint{}, CollectionPoint{}, nil, false
	}
	used = append([]CollectionPoint(nil), edges[bestStart:bestEnd+1]...)
	return edges[bestStart], edges[bestEnd], used, true
}

// WindowHasReset reports whether any consecutive complete edges differ in reset epoch.
func WindowHasReset(points []CollectionPoint) bool {
	var edges []CollectionPoint
	for _, p := range points {
		if IsCompleteEdge(p) {
			edges = append(edges, p)
		}
	}
	for i := 1; i < len(edges); i++ {
		if StatsResetChanged(edges[i-1].StatsReset, edges[i].StatsReset) ||
			StatsResetChanged(edges[i-1].WorkloadReset, edges[i].WorkloadReset) {
			return true
		}
	}
	return false
}

// GapExceedsMax reports whether first→last exceeds the regression max gap.
func GapExceedsMax(first, last CollectionPoint, maxGap time.Duration) bool {
	if first.StartedAt.IsZero() || last.StartedAt.IsZero() || !last.StartedAt.After(first.StartedAt) {
		return true
	}
	return last.StartedAt.Sub(first.StartedAt) > maxGap
}

// ComputeDeltas builds per-fingerprint deltas between first and last run IDs.
// Points must include observations for those runs; missing fingerprints are skipped.
func ComputeDeltas(points []WorkloadPoint, firstRunID, lastRunID string) []WindowDelta {
	type key struct{ db, fp string }
	first := map[key]WorkloadPoint{}
	last := map[key]WorkloadPoint{}
	for _, p := range points {
		k := key{p.DatabaseName, p.Fingerprint}
		switch p.AuditRunID {
		case firstRunID:
			first[k] = p
		case lastRunID:
			last[k] = p
		}
	}
	out := make([]WindowDelta, 0, len(last)+len(first))
	seen := map[key]struct{}{}
	for k, l := range last {
		seen[k] = struct{}{}
		d := WindowDelta{
			Fingerprint:       k.fp,
			QueryID:           l.QueryID,
			DatabaseName:      k.db,
			ReferencedObjects: append([]string(nil), l.ReferencedObjects...),
			LastRunID:         lastRunID,
			LastCalls:         l.Calls,
			LastTotalMS:       l.TotalExecTimeMS,
			LastMeanMS:        l.MeanExecTimeMS,
		}
		if f, ok := first[k]; ok {
			d.FirstRunID = firstRunID
			d.FirstCalls = f.Calls
			d.FirstTotalMS = f.TotalExecTimeMS
			d.FirstMeanMS = f.MeanExecTimeMS
			d.DeltaCalls = l.Calls - f.Calls
			d.DeltaTotalMS = l.TotalExecTimeMS - f.TotalExecTimeMS
			d.DeltaRows = l.RowsTotal - f.RowsTotal
			d.DeltaBlocksRead = l.SharedBlocksRead - f.SharedBlocksRead
			if d.QueryID == "" {
				d.QueryID = f.QueryID
			}
			if len(d.ReferencedObjects) == 0 {
				d.ReferencedObjects = append([]string(nil), f.ReferencedObjects...)
			}
			if d.DeltaCalls > 0 {
				d.MeanMS = d.DeltaTotalMS / float64(d.DeltaCalls)
			}
		} else {
			// New in last window edge: treat cumulative as the window contribution.
			d.DeltaCalls = l.Calls
			d.DeltaTotalMS = l.TotalExecTimeMS
			d.DeltaRows = l.RowsTotal
			d.DeltaBlocksRead = l.SharedBlocksRead
			d.MeanMS = l.MeanExecTimeMS
		}
		out = append(out, d)
	}
	for k, f := range first {
		if _, ok := seen[k]; ok {
			continue
		}
		// Present only in first edge → candidate for "sumiu".
		out = append(out, WindowDelta{
			Fingerprint:       k.fp,
			QueryID:           f.QueryID,
			DatabaseName:      k.db,
			ReferencedObjects: append([]string(nil), f.ReferencedObjects...),
			FirstRunID:        firstRunID,
			FirstCalls:        f.Calls,
			FirstTotalMS:      f.TotalExecTimeMS,
			FirstMeanMS:       f.MeanExecTimeMS,
			LastRunID:         lastRunID,
		})
	}
	return out
}

// ResolveWindow selects comparable edges for a calendar window of collections.
// On reset mid-window it uses the largest contiguous subwindow (CA-02).
func ResolveWindow(points []CollectionPoint, workload []WorkloadPoint, maxGap time.Duration) WindowResult {
	edges := make([]CollectionPoint, 0, len(points))
	for _, p := range points {
		if IsCompleteEdge(p) {
			edges = append(edges, p)
		}
	}
	if len(edges) < 2 {
		return WindowResult{Reason: "sem janela comparável: menos de duas coletas completas"}
	}
	first, last := edges[0], edges[len(edges)-1]
	usedSub := false
	note := ""
	if WindowHasReset(edges) {
		f, l, used, ok := LargestContiguousSubwindowWithoutReset(edges)
		if !ok {
			return WindowResult{Reason: "sem janela comparável: stats_reset sem subjanela contígua"}
		}
		first, last, usedSub = f, l, true
		note = "subjanela sem reset de " + first.StartedAt.UTC().Format(time.RFC3339) +
			" a " + last.StartedAt.UTC().Format(time.RFC3339) +
			" (" + first.AuditRunID + " → " + last.AuditRunID + ")"
		edges = used
	}
	if GapExceedsMax(first, last, maxGap) {
		return WindowResult{Reason: "sem janela comparável: lacuna maior que AUDITOR_REGRESSION_MAX_GAP"}
	}
	deltas := ComputeDeltas(workload, first.AuditRunID, last.AuditRunID)
	var total float64
	fps := map[string]struct{}{}
	for _, d := range deltas {
		if d.LastCalls > 0 || d.LastTotalMS > 0 {
			total += d.LastTotalMS
		}
		fps[d.Fingerprint] = struct{}{}
	}
	return WindowResult{
		OK:               true,
		First:            first,
		Last:             last,
		UsedSubwindow:    usedSub,
		SubwindowNote:    note,
		Points:           edges,
		Deltas:           deltas,
		TotalTimeMS:      total,
		FingerprintCount: len(fps),
	}
}

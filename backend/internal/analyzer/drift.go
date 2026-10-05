package analyzer

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// ClosedGUCs is the only setting set compared between environments.
var ClosedGUCs = []string{
	"shared_buffers",
	"work_mem",
	"max_connections",
	"timescaledb.max_background_workers",
}

// ServerSide is one environment's latest successful snapshot, with no live session.
type ServerSide struct {
	EnvironmentID    string
	Name             string
	AuditRunID       string
	ServerVersion    string
	TimescaleVersion string
	Extensions       []ExtensionFact
	Settings         map[string]string
	Hypertables      []HypertableFact
	Policies         []PolicyFact
	CAGGs            []CAGGFact
	Critical         []CriticalFact
}

// ExtensionFact is one installed extension version.
type ExtensionFact struct {
	Database string
	Name     string
	Version  string
}

// CriticalFact is a high or critical finding observed on a run.
type CriticalFact struct {
	DedupKey string
	Title    string
	Severity string
}

// CompareRow is one side-by-side difference. Informational rows are not defects.
type CompareRow struct {
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	Left          string `json:"left"`
	Right         string `json:"right"`
	Status        string `json:"status"`
	Informational bool   `json:"informational"`
	Detail        string `json:"detail,omitempty"`
}

// CompareReport is the snapshot-only view of two environments.
type CompareReport struct {
	LeftRunID  string       `json:"left_run_id"`
	RightRunID string       `json:"right_run_id"`
	Rows       []CompareRow `json:"rows"`
}

// DriftAnalyzer emits version, extension and closed-GUC drift when another
// environment already has a successful snapshot.
type DriftAnalyzer struct{}

func (DriftAnalyzer) Name() string { return "drift" }

func (DriftAnalyzer) Analyze(_ context.Context, facts SnapshotFacts) ([]Finding, error) {
	if len(facts.Peers) == 0 {
		return nil, nil
	}
	tolerance := gucTolerance(facts)
	var out []Finding
	for _, peer := range facts.Peers {
		out = append(out, versionDrift(facts, peer)...)
		out = append(out, extensionDrift(facts, peer)...)
		out = append(out, gucDrift(facts, peer, tolerance)...)
	}
	return out, nil
}

func versionDrift(facts SnapshotFacts, peer ServerSide) []Finding {
	var out []Finding
	if facts.ServerVersion != "" && peer.ServerVersion != "" && facts.ServerVersion != peer.ServerVersion {
		out = append(out, driftFinding(facts, peer, "config.version_drift", "postgres", facts.ServerVersion, peer.ServerVersion, SeverityMedium, false))
	}
	if facts.TimescaleVersion != "" && peer.TimescaleVersion != "" && facts.TimescaleVersion != peer.TimescaleVersion {
		out = append(out, driftFinding(facts, peer, "config.version_drift", "timescaledb", facts.TimescaleVersion, peer.TimescaleVersion, SeverityMedium, false))
	}
	return out
}

func extensionDrift(facts SnapshotFacts, peer ServerSide) []Finding {
	left := extensionMap(facts.Extensions)
	right := extensionMap(peer.Extensions)
	var out []Finding
	seen := map[string]bool{}
	for name, version := range left {
		seen[name] = true
		other, ok := right[name]
		if !ok {
			out = append(out, driftFinding(facts, peer, "config.extension_drift", name, version, "", SeverityLow, false))
			continue
		}
		if other != version {
			out = append(out, driftFinding(facts, peer, "config.extension_drift", name, version, other, SeverityMedium, false))
		}
	}
	for name, version := range right {
		if seen[name] {
			continue
		}
		out = append(out, driftFinding(facts, peer, "config.extension_drift", name, "", version, SeverityLow, false))
	}
	return out
}

func gucDrift(facts SnapshotFacts, peer ServerSide, tolerance float64) []Finding {
	var out []Finding
	for _, name := range ClosedGUCs {
		left := facts.Settings[name]
		right := peer.Settings[name]
		if left == "" || right == "" || left == right {
			continue
		}
		info := name == "shared_buffers"
		if !info && withinTolerance(left, right, tolerance) {
			continue
		}
		sev := SeverityMedium
		if info {
			sev = SeverityInfo
		}
		out = append(out, driftFinding(facts, peer, "config.guc_drift", name, left, right, sev, info))
	}
	return out
}

func driftFinding(facts SnapshotFacts, peer ServerSide, kind, name, left, right string, sev Severity, informational bool) Finding {
	key := fmt.Sprintf("%s|%s|%s", peer.EnvironmentID, kind, name)
	title := fmt.Sprintf("Drift %s %s vs %s", name, facts.EnvironmentID, peer.Name)
	if peer.Name == "" {
		title = fmt.Sprintf("Drift %s vs %s", name, peer.EnvironmentID)
	}
	summary := fmt.Sprintf("%s is %q here and %q on %s.", name, left, right, peerLabel(peer))
	if informational {
		summary += " A shared_buffers difference between cloud and self-hosted is informational, not an automatic defect."
	}
	return Finding{
		EnvironmentID: facts.EnvironmentID,
		AuditRunID:    facts.AuditRunID,
		FindingType:   kind,
		Severity:      sev,
		Status:        StatusOpen,
		Title:         title,
		Summary:       summary,
		ObjectType:    "environment",
		ObjectKey:     key,
		Evidence: map[string]any{
			"name": name, "left": left, "right": right,
			"peer_environment_id": peer.EnvironmentID, "peer_run_id": peer.AuditRunID,
			"informational": informational,
		},
		DedupKey: DedupKey(kind, key, name),
	}
}

func peerLabel(peer ServerSide) string {
	if peer.Name != "" {
		return peer.Name
	}
	return peer.EnvironmentID
}

func extensionMap(items []ExtensionFact) map[string]string {
	grouped := map[string]map[string]struct{}{}
	for _, item := range items {
		if item.Name == "" {
			continue
		}
		if grouped[item.Name] == nil {
			grouped[item.Name] = map[string]struct{}{}
		}
		grouped[item.Name][item.Version] = struct{}{}
	}
	out := map[string]string{}
	for name, versions := range grouped {
		parts := make([]string, 0, len(versions))
		for version := range versions {
			parts = append(parts, version)
		}
		sort.Strings(parts)
		out[name] = strings.Join(parts, ",")
	}
	return out
}

func gucTolerance(facts SnapshotFacts) float64 {
	for _, rule := range EffectiveCatalog(facts, "") {
		if rule.ID != "config.guc_drift" {
			continue
		}
		switch value := rule.EffectiveParameters["numeric_tolerance"].(type) {
		case float64:
			return value
		case int:
			return float64(value)
		case string:
			parsed, err := strconv.ParseFloat(value, 64)
			if err == nil {
				return parsed
			}
		}
	}
	return 0
}

func withinTolerance(left, right string, tolerance float64) bool {
	if tolerance <= 0 {
		return false
	}
	a, aOK := parseSettingNumber(left)
	b, bOK := parseSettingNumber(right)
	if !aOK || !bOK || a == 0 || b == 0 {
		return false
	}
	return math.Abs(a-b)/math.Max(a, b) <= tolerance
}

func parseSettingNumber(value string) (float64, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return 0, false
	}
	mult := 1.0
	switch {
	case strings.HasSuffix(value, "tb"):
		mult, value = float64(1<<40), strings.TrimSuffix(value, "tb")
	case strings.HasSuffix(value, "gb"):
		mult, value = float64(1<<30), strings.TrimSuffix(value, "gb")
	case strings.HasSuffix(value, "mb"):
		mult, value = float64(1<<20), strings.TrimSuffix(value, "mb")
	case strings.HasSuffix(value, "kb"):
		mult, value = float64(1<<10), strings.TrimSuffix(value, "kb")
	case strings.HasSuffix(value, "b"):
		value = strings.TrimSuffix(value, "b")
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, false
	}
	return n * mult, true
}

// CompareServers builds a side-by-side report from snapshots only.
func CompareServers(left, right ServerSide) CompareReport {
	report := CompareReport{LeftRunID: left.AuditRunID, RightRunID: right.AuditRunID}
	add := func(row CompareRow) {
		if row.Left == row.Right && row.Status == "match" {
			return
		}
		report.Rows = append(report.Rows, row)
	}
	add(versionRow("postgres", left.ServerVersion, right.ServerVersion))
	add(versionRow("timescaledb", left.TimescaleVersion, right.TimescaleVersion))
	leftExt := extensionMap(left.Extensions)
	rightExt := extensionMap(right.Extensions)
	for name, version := range leftExt {
		other, ok := rightExt[name]
		status := "match"
		if !ok {
			status = "only_left"
		} else if other != version {
			status = "drift"
		}
		add(CompareRow{Kind: "extension", Name: name, Left: version, Right: other, Status: status})
	}
	for name, version := range rightExt {
		if _, ok := leftExt[name]; ok {
			continue
		}
		add(CompareRow{Kind: "extension", Name: name, Right: version, Status: "only_right"})
	}
	for _, name := range ClosedGUCs {
		lv, rv := left.Settings[name], right.Settings[name]
		if lv == "" && rv == "" {
			continue
		}
		status := "match"
		switch {
		case lv == "":
			status = "only_right"
		case rv == "":
			status = "only_left"
		case lv != rv:
			status = "drift"
		}
		row := CompareRow{Kind: "guc", Name: name, Left: lv, Right: rv, Status: status}
		if name == "shared_buffers" && status == "drift" {
			row.Informational = true
			row.Detail = "Diferença de shared_buffers entre cloud e self-hosted é informativa, não um defeito automático."
		}
		add(row)
	}
	leftHT := hypertableSet(left.Hypertables)
	rightHT := hypertableSet(right.Hypertables)
	for name, interval := range leftHT {
		other, ok := rightHT[name]
		if !ok {
			add(CompareRow{Kind: "hypertable", Name: name, Left: "present", Status: "only_left"})
			continue
		}
		status := "match"
		if interval != other {
			status = "drift"
		}
		add(CompareRow{Kind: "chunk_interval", Name: name, Left: interval, Right: other, Status: status})
	}
	for name := range rightHT {
		if _, ok := leftHT[name]; !ok {
			add(CompareRow{Kind: "hypertable", Name: name, Right: "present", Status: "only_right"})
		}
	}
	leftPol := policySet(left.Policies)
	rightPol := policySet(right.Policies)
	for name, cfg := range leftPol {
		other, ok := rightPol[name]
		status := "match"
		if !ok {
			status = "only_left"
		} else if other != cfg {
			status = "drift"
		}
		add(CompareRow{Kind: "policy", Name: name, Left: cfg, Right: other, Status: status})
	}
	for name, cfg := range rightPol {
		if _, ok := leftPol[name]; ok {
			continue
		}
		add(CompareRow{Kind: "policy", Name: name, Right: cfg, Status: "only_right"})
	}
	leftC := caggSet(left.CAGGs)
	rightC := caggSet(right.CAGGs)
	for name, detail := range leftC {
		other, ok := rightC[name]
		status := "match"
		if !ok {
			status = "only_left"
		} else if other != detail {
			status = "drift"
		}
		add(CompareRow{Kind: "cagg", Name: name, Left: detail, Right: other, Status: status})
	}
	for name, detail := range rightC {
		if _, ok := leftC[name]; ok {
			continue
		}
		add(CompareRow{Kind: "cagg", Name: name, Right: detail, Status: "only_right"})
	}
	leftCrit := criticalSet(left.Critical)
	rightCrit := criticalSet(right.Critical)
	for key, title := range leftCrit {
		if _, ok := rightCrit[key]; !ok {
			add(CompareRow{Kind: "critical", Name: title, Left: key, Status: "only_left", Detail: "Novo desde o run anterior deste lado."})
		}
	}
	for key, title := range rightCrit {
		if _, ok := leftCrit[key]; !ok {
			add(CompareRow{Kind: "critical", Name: title, Right: key, Status: "only_right", Detail: "Novo desde o run anterior deste lado."})
		}
	}
	return report
}

func versionRow(name, left, right string) CompareRow {
	status := "match"
	switch {
	case left == "" && right == "":
		status = "match"
	case left == "":
		status = "only_right"
	case right == "":
		status = "only_left"
	case left != right:
		status = "drift"
	}
	return CompareRow{Kind: "version", Name: name, Left: left, Right: right, Status: status}
}

func hypertableSet(items []HypertableFact) map[string]string {
	out := map[string]string{}
	for _, item := range items {
		out[fmt.Sprintf("%s.%s.%s", item.Database, item.Schema, item.Name)] = item.ChunkInterval
	}
	return out
}

func policySet(items []PolicyFact) map[string]string {
	out := map[string]string{}
	for _, item := range items {
		name := fmt.Sprintf("%s %s.%s", item.PolicyType, item.HypertableSchema, item.HypertableName)
		if item.Database != "" {
			name = item.Database + " " + name
		}
		out[name] = item.Config
	}
	return out
}

func caggSet(items []CAGGFact) map[string]string {
	out := map[string]string{}
	for _, item := range items {
		name := fmt.Sprintf("%s.%s", item.Schema, item.ViewName)
		if item.Database != "" {
			name = item.Database + "." + name
		}
		flags := fmt.Sprintf("lag=%s hierarchical=%t realtime=%t refresh=%t", item.Lag, item.Hierarchical, item.Realtime, item.HasRefreshPolicy)
		out[name] = flags
	}
	return out
}

func criticalSet(items []CriticalFact) map[string]string {
	out := map[string]string{}
	for _, item := range items {
		out[item.DedupKey] = item.Title
	}
	return out
}

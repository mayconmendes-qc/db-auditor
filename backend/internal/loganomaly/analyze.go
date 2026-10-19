package loganomaly

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// AnalyzeResult is the full local run outcome.
type AnalyzeResult struct {
	Report   Report
	ExitCode int
	Stats    ReadStats
}

// Analyze runs the local pipeline: read → mask → template → windows → baseline → anomalies.
// It never opens a database or network connection (RNF-01).
func Analyze(opts Options, stdin io.Reader) (AnalyzeResult, error) {
	th := opts.Thresholds
	if th.Window == 0 {
		th = DefaultThresholds()
	}
	if opts.Window > 0 {
		th.Window = opts.Window
	}
	if opts.BaselineK > 0 {
		th.BaselineWindows = opts.BaselineK
	}
	if opts.FailOn != "" {
		th.FailOn = opts.FailOn
	}

	prof, err := NewProfile(opts.Profile, opts.TimeField, opts.LevelField, opts.MessageField)
	if err != nil {
		return AnalyzeResult{ExitCode: 2}, err
	}
	if opts.LastLoginOut != "" && prof.Name() != ProfilePostgresJSONLog {
		return AnalyzeResult{ExitCode: 2}, fmt.Errorf("--last-login-out só é suportado com perfil postgres-jsonlog")
	}

	store := NewTemplateStore()
	counter := NewCounter(th.Window)
	login := NewLastLoginAgg()
	levelCount := map[string]map[string]int{}
	var softInvalid int64

	stats, err := ReadSources(opts.Paths, stdin, func(raw map[string]json.RawMessage) error {
		ts, level, msg, perr := prof.Parse(raw)
		if perr != nil {
			softInvalid++
			return nil
		}
		id, pattern, unclass := store.Observe(msg)
		ev := Event{
			Time:           ts,
			Level:          level,
			TemplateID:     id,
			Template:       pattern,
			Unclassifiable: unclass,
		}
		if !unclass && id != OthersTemplateID && id != UnclassifiableID {
			if levelCount[id] == nil {
				levelCount[id] = map[string]int{}
			}
			levelCount[id][level]++
		}
		counter.Add(ev)

		if opts.LastLoginOut != "" && IsConnectionAuthorized(msg) {
			role := prof.RoleFromJSON(raw)
			if role == "" {
				role = RoleFromConnMessage(msg)
			}
			login.Observe(role, ts)
		}
		return nil
	})
	if err != nil {
		code := 3
		if isPathError(err) {
			code = 2
		}
		return AnalyzeResult{ExitCode: code, Stats: stats}, err
	}
	if softInvalid > 0 {
		stats.LinesInvalid += softInvalid
		stats.LinesParsed -= softInvalid
		if stats.LinesParsed < 0 {
			stats.LinesParsed = 0
		}
	}

	levelByTemplate := map[string]string{}
	for id, m := range levelCount {
		best, bestN := LevelInfo, -1
		for lvl, n := range m {
			if n > bestN {
				best, bestN = lvl, n
			}
		}
		levelByTemplate[id] = best
	}

	windows := counter.Windows()
	baseWins, analysisWins := SplitWindows(windows, th.BaselineWindows, opts.BaselineFrom, opts.BaselineTo)
	base := ComputeBaseline(baseWins, th.MinBaselineWins)
	anoms := DetectAnomalies(windows, base, analysisWins, th, levelByTemplate)

	var caveats []string
	if stats.LinesOversized > 0 {
		caveats = append(caveats, fmt.Sprintf("%d linha(s) acima de 64KB descartadas", stats.LinesOversized))
	}
	if stats.LinesInvalid > 0 {
		caveats = append(caveats, fmt.Sprintf("%d linha(s) JSON/campos inválidos", stats.LinesInvalid))
	}
	if store.Overflow() > 0 {
		caveats = append(caveats, fmt.Sprintf("limite de %d templates: %d evento(s) em \"outros\"", MaxTemplates, store.Overflow()))
	}
	if base.Insufficient {
		caveats = append(caveats, fmt.Sprintf("base insuficiente (%d<%d janelas): só templates novos com confiança baixa", base.WindowCount, th.MinBaselineWins))
	}
	if stats.LinesRead > 0 && stats.InvalidRatio() > InvalidLineRatio {
		caveats = append(caveats, fmt.Sprintf("mais de %.0f%% das linhas inválidas", InvalidLineRatio*100))
	}

	rep := Report{
		SchemaVersion:    SchemaVersion,
		Profile:          prof.Name(),
		WindowSeconds:    int(th.Window.Seconds()),
		Stats:            stats,
		Templates:        store.Len(),
		TemplateOverflow: store.Overflow(),
		Baseline: BaselineSummary{
			Windows:      base.WindowCount,
			Insufficient: base.Insufficient,
		},
		Anomalies: anoms,
		Summary:   BuildSummary(anoms),
		Caveats:   caveats,
	}
	if !base.From.IsZero() {
		rep.Baseline.From = base.From.UTC().Format(time.RFC3339)
		rep.Baseline.To = base.To.UTC().Format(time.RFC3339)
	}
	if min, max, ok := counter.Range(); ok {
		rep.PeriodStart = min.UTC().Format(time.RFC3339)
		rep.PeriodEnd = max.Add(th.Window).UTC().Format(time.RFC3339)
	}

	if opts.LastLoginOut != "" {
		f := login.File()
		rep.LastLogin = &f
		if opts.LastLoginOut != "-" {
			if werr := WriteLastLoginJSON(opts.LastLoginOut, f); werr != nil {
				return AnalyzeResult{ExitCode: 2, Stats: stats, Report: rep}, werr
			}
		}
	}

	exit := GateExit(anoms, th.FailOn, stats, base.Insufficient, true)
	return AnalyzeResult{Report: rep, ExitCode: exit, Stats: stats}, nil
}

func isPathError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "arquivo inacessível") ||
		strings.Contains(msg, "diretório inacessível") ||
		strings.Contains(msg, "nenhum arquivo")
}

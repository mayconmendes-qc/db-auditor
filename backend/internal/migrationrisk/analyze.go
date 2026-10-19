package migrationrisk

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AnalyzePaths reads SQL files/directories and returns a Report.
func AnalyzePaths(paths []string, ctx Context, th Thresholds) (Report, error) {
	if ctx.PGVersion == 0 {
		ctx.PGVersion = 16
	}
	if ctx.Mode == "" {
		ctx.Mode = "none"
	}
	files, err := collectSQLFiles(paths)
	if err != nil {
		return Report{}, err
	}
	var allOps []Operation
	var allSuppress []Suppression
	for _, file := range files {
		data, err := os.ReadFile(file) //nolint:gosec // G703: path comes from operator CLI args.
		if err != nil {
			return Report{}, fmt.Errorf("arquivo ilegível %s: %w", file, err)
		}
		content := string(data)
		suppress, err := ParseSuppressions(content)
		if err != nil {
			return Report{}, err
		}
		allSuppress = append(allSuppress, suppress...)
		ops := ParseFile(file, content)
		allOps = append(allOps, ops...)
	}
	truncated := false
	if len(allOps) > MaxOperations {
		allOps = allOps[:MaxOperations]
		truncated = true
	}
	findings := analyzeOperations(allOps, ctx, th, allSuppress)
	rep := Report{
		SchemaVersion: SchemaVersion,
		RulesVersion:  RulesVersion,
		PGVersion:     ctx.PGVersion,
		ContextMode:   ctx.Mode,
		Partial:       ctx.Partial || ctx.Stale || ctx.Missing,
		Truncated:     truncated,
		AuditRunID:    ctx.AuditRunID,
		AuditRunAt:    ctx.AuditRunAt,
		Findings:      findings,
		Summary:       summarize(findings),
	}
	return rep, nil
}

func collectSQLFiles(paths []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			err := filepath.Walk(p, func(path string, fi os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if fi.IsDir() {
					return nil
				}
				if strings.EqualFold(filepath.Ext(path), ".sql") {
					if !seen[path] {
						seen[path] = true
						out = append(out, path)
					}
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
			continue
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, nil
}

func summarize(findings []Finding) Summary {
	var s Summary
	rank := map[string]int{
		SeverityInfo: 1, SeverityLow: 2, SeverityMedium: 3,
		SeverityHigh: 4, SeverityCritical: 5,
	}
	highest := 0
	for _, f := range findings {
		if f.Suppressed || f.Unanalyzed {
			continue
		}
		switch f.Severity {
		case SeverityInfo:
			s.Info++
		case SeverityLow:
			s.Low++
		case SeverityMedium:
			s.Medium++
		case SeverityHigh:
			s.High++
		case SeverityCritical:
			s.Critical++
		}
		if r := rank[f.Severity]; r > highest {
			highest = r
			s.Highest = f.Severity
		}
	}
	if s.Highest == "" {
		s.Highest = SeverityInfo
	}
	return s
}

// GateExit returns the CLI exit code for a report given fail-on threshold.
func GateExit(rep Report, failOn string, suppressErr error) int {
	if suppressErr != nil {
		return 2 // ExitUsage — bad suppression syntax
	}
	if rep.Partial {
		return 3
	}
	for _, f := range rep.Findings {
		if f.Unanalyzed {
			return 3
		}
	}
	threshold := severityRank(failOn)
	if threshold == 0 {
		threshold = severityRank(SeverityHigh)
	}
	for _, f := range rep.Findings {
		if f.Suppressed {
			continue
		}
		if severityRank(f.Severity) >= threshold {
			return 1
		}
	}
	return 0
}

func severityRank(s string) int {
	switch strings.ToLower(s) {
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

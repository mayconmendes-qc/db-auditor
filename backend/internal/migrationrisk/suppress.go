package migrationrisk

import (
	"fmt"
	"regexp"
	"strings"
)

// -- db-auditor:ignore MR-01 motivo: <texto>
var suppressPattern = regexp.MustCompile(`(?i)--\s*db-auditor:ignore\s+(MR-\d{2})\s+motivo:\s*(\S.*)`)

// Suppression is an inline ignore directive tied to the next statement.
type Suppression struct {
	Code          string
	Justification string
	Line          int
}

// ParseSuppressions extracts ignore directives from raw file content.
func ParseSuppressions(content string) ([]Suppression, error) {
	var out []Suppression
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		m := suppressPattern.FindStringSubmatch(line)
		if m == nil {
			if strings.Contains(strings.ToLower(line), "db-auditor:ignore") {
				return nil, fmt.Errorf("linha %d: supressão sem justificativa (use -- db-auditor:ignore MR-XX motivo: <texto>)", i+1)
			}
			continue
		}
		just := strings.TrimSpace(m[2])
		if just == "" {
			return nil, fmt.Errorf("linha %d: justificativa de supressão obrigatória", i+1)
		}
		out = append(out, Suppression{Code: strings.ToUpper(m[1]), Justification: just, Line: i + 1})
	}
	return out, nil
}

// MatchSuppression finds a directive on the line immediately before the operation.
func MatchSuppression(opsLine int, code string, all []Suppression) *Suppression {
	for i := range all {
		s := &all[i]
		if !strings.EqualFold(s.Code, code) {
			continue
		}
		// Directive must sit on the previous non-empty comment line (opsLine-1 typically).
		if s.Line == opsLine-1 || s.Line == opsLine {
			return s
		}
	}
	return nil
}

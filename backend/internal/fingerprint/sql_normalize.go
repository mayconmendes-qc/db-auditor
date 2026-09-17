package fingerprint

import (
	"regexp"
	"strings"
)

var (
	multiSpace = regexp.MustCompile(`\s+`)
	lineComments = regexp.MustCompile(`(?m)--.*?$`)
	blockComments = regexp.MustCompile(`(?s)/\*.*?\*/`)
)

// NormalizeSQL collapses whitespace, lowercases keywords-ish content and strips comments
// for structural comparison. It is deliberately conservative (not a full SQL parser).
func NormalizeSQL(def string) string {
	s := strings.TrimSpace(def)
	if s == "" {
		return ""
	}
	s = blockComments.ReplaceAllString(s, " ")
	s = lineComments.ReplaceAllString(s, " ")
	s = multiSpace.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	// strip trailing semicolon for stable equality
	s = strings.TrimSuffix(s, ";")
	s = strings.TrimSpace(s)
	return strings.ToLower(s)
}

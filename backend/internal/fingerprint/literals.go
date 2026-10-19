package fingerprint

import (
	"regexp"
	"strings"
)

// Patterns used to strip literals from SQL for privacy-safe fingerprints.
// Extracted from collectors/postgres.NormalizeQuery (ADR-0001 D16).
var (
	literalBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	literalLineComment  = regexp.MustCompile(`(?m)--[^\n]*`)
	literalQuoted       = regexp.MustCompile(`'(?:''|[^'])*'`)
	literalDollar       = regexp.MustCompile(`(?s)\$[A-Za-z_0-9]*\$.*?\$[A-Za-z_0-9]*\$`)
	literalNumber       = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	literalSpace        = regexp.MustCompile(`\s+`)
)

// StripLiterals removes comments and literal values from SQL.
// It is intentionally conservative and is never used to reconstruct or execute SQL.
func StripLiterals(query string) string {
	q := literalBlockComment.ReplaceAllString(query, " ")
	q = literalLineComment.ReplaceAllString(q, " ")
	q = literalDollar.ReplaceAllString(q, "?")
	q = literalQuoted.ReplaceAllString(q, "?")
	q = literalNumber.ReplaceAllString(q, "?")
	return strings.TrimSpace(literalSpace.ReplaceAllString(strings.ToLower(q), " "))
}

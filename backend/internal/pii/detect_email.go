package pii

import (
	"strings"
	"unicode"
)

// ValidEmail accepts a simple practical email shape (local@domain.tld).
func ValidEmail(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 5 || len(s) > 254 {
		return false
	}
	at := strings.LastIndexByte(s, '@')
	if at < 1 || at >= len(s)-3 {
		return false
	}
	local, domain := s[:at], s[at+1:]
	if local == "" || domain == "" || strings.ContainsAny(local+domain, " 	") {
		return false
	}
	dot := strings.LastIndexByte(domain, '.')
	if dot < 1 || dot >= len(domain)-1 {
		return false
	}
	for _, part := range []string{local, domain} {
		for _, r := range part {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '+' || r == '-' {
				continue
			}
			return false
		}
	}
	return true
}

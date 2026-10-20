package pii

import (
	"strings"
	"unicode"
)

// ValidCNPJ accepts numeric CNPJ (14 digits) and alphanumeric CNPJ
// (Receita Federal format; ADR-0001 D14) with matching check digits.
func ValidCNPJ(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	cleaned := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '.' || r == '/' || r == '-' || unicode.IsSpace(r) {
			continue
		}
		cleaned = append(cleaned, unicode.ToUpper(r))
	}
	if len(cleaned) != 14 {
		return false
	}
	vals := make([]int, 14)
	numeric := true
	for i, r := range cleaned {
		if unicode.IsDigit(r) {
			vals[i] = int(r - '0')
			continue
		}
		if r >= 'A' && r <= 'Z' {
			numeric = false
			// Receita alphanumeric: value = ASCII code - 48 (0→0 … A→17 …).
			vals[i] = int(r) - 48
			continue
		}
		return false
	}
	if numeric {
		same := true
		for i := 1; i < 14; i++ {
			if vals[i] != vals[0] {
				same = false
				break
			}
		}
		if same {
			return false
		}
	} else if !unicode.IsDigit(cleaned[12]) || !unicode.IsDigit(cleaned[13]) {
		return false
	}
	d1 := cnpjDV(vals[:12], []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2})
	d2 := cnpjDV(vals[:13], []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2})
	return vals[12] == d1 && vals[13] == d2
}

func cnpjDV(vals []int, weights []int) int {
	sum := 0
	for i := range vals {
		sum += vals[i] * weights[i]
	}
	r := sum % 11
	if r < 2 {
		return 0
	}
	return 11 - r
}

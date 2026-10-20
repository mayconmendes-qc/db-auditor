package pii

import "strings"

// ValidPIXKey accepts CPF, CNPJ, email, phone, or random UUID-shaped keys.
func ValidPIXKey(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if ValidCPF(s) || ValidCNPJ(s) || ValidEmail(s) || ValidBRPhone(s) {
		return true
	}
	return validPIXUUID(s)
}

func validPIXUUID(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) != 36 {
		return false
	}
	// 8-4-4-4-12
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
				return false
			}
		}
	}
	return true
}

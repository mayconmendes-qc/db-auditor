package pii

// ValidBRPhone accepts Brazilian landline/mobile with optional +55 and separators.
func ValidBRPhone(s string) bool {
	d := digitsOnly(s)
	switch len(d) {
	case 10, 11:
		// DDD + number
	case 12, 13:
		// 55 + DDD + number
		if d[0] != 5 || d[1] != 5 {
			return false
		}
		d = d[2:]
	default:
		return false
	}
	if d[0] == 0 || d[1] == 0 {
		return false
	}
	if len(d) == 11 && d[2] != 9 {
		return false
	}
	return true
}

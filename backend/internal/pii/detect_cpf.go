package pii

import "unicode"

// ValidCPF reports whether s is a CPF with valid check digits.
// Rejects empty input and repeated-digit sequences. Never logs s.
func ValidCPF(s string) bool {
	digits := digitsOnly(s)
	if len(digits) != 11 {
		return false
	}
	if allSame(digits) {
		return false
	}
	d1 := cpfCheckDigit(digits[:9], 10)
	d2 := cpfCheckDigit(digits[:10], 11)
	return digits[9] == d1 && digits[10] == d2
}

func cpfCheckDigit(nums []byte, weightStart int) byte {
	sum := 0
	w := weightStart
	for _, n := range nums {
		sum += int(n) * w
		w--
	}
	r := sum % 11
	if r < 2 {
		return 0
	}
	return byte(11 - r)
}

func digitsOnly(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if unicode.IsDigit(r) {
			out = append(out, byte(r-'0'))
		}
	}
	return out
}

func allSame(d []byte) bool {
	if len(d) == 0 {
		return true
	}
	for i := 1; i < len(d); i++ {
		if d[i] != d[0] {
			return false
		}
	}
	return true
}

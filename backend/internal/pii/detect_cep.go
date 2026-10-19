package pii

// ValidCEP accepts Brazilian postal codes as 8 digits, optionally NNNNN-NNN.
func ValidCEP(s string) bool {
	d := digitsOnly(s)
	return len(d) == 8
}

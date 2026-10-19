package fingerprint

import (
	"strings"
	"testing"
)

func TestStripLiteralsMatchesWorkloadContract(t *testing.T) {
	raw := "SELECT * FROM public.customers -- customer email\n WHERE email = 'person@example.com' AND id = 4242 /* secret */"
	got := StripLiterals(raw)
	for _, sensitive := range []string{"person@example.com", "4242", "customer email", "secret"} {
		if strings.Contains(got, sensitive) {
			t.Fatalf("leaked %q: %s", sensitive, got)
		}
	}
	if !strings.Contains(got, "public.customers") {
		t.Fatalf("lost object reference: %s", got)
	}
}

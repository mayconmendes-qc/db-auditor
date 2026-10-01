package postgres

import (
	"strings"
	"testing"
)

func TestNormalizeQueryRemovesSensitiveLiteralsAndComments(t *testing.T) {
	raw := "SELECT * FROM public.customers -- customer email\n WHERE email = 'person@example.com' AND id = 4242 /* secret */"
	normalized := NormalizeQuery(raw)
	for _, sensitive := range []string{"person@example.com", "4242", "customer email", "secret"} {
		if strings.Contains(normalized, sensitive) {
			t.Fatalf("normalized query leaked %q: %s", sensitive, normalized)
		}
	}
	if got := referencedObjects(normalized); len(got) != 1 || got[0] != "public.customers" {
		t.Fatalf("unexpected references: %#v", got)
	}
	fp := queryFingerprint(normalized)
	if !strings.HasPrefix(fp, "sha256:") || strings.Contains(fp, "customers") {
		t.Fatalf("unsafe fingerprint: %s", fp)
	}
}

func TestColumnStatsSQLDoesNotCollectRawValues(t *testing.T) {
	lower := strings.ToLower(columnStatsSQL)
	for _, forbidden := range []string{"most_common_vals", "histogram_bounds", "most_common_elems"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("column stats SQL includes private field %s", forbidden)
		}
	}
}

func TestReferencedObjectsRejectsAmbiguousUnqualifiedNames(t *testing.T) {
	got := referencedObjects(NormalizeQuery("select * from orders join public.customers on true"))
	if len(got) != 1 || got[0] != "public.customers" {
		t.Fatalf("unexpected qualified references: %#v", got)
	}
}

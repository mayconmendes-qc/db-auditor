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

func TestQueryKindAndCollectionLimits(t *testing.T) {
	for raw, want := range map[string]string{"SELECT * FROM public.orders WHERE id=42": "select", "INSERT INTO public.orders VALUES ('private')": "insert", "UPDATE public.orders SET x=2": "update", "DELETE FROM public.orders": "delete", "WITH x AS (SELECT 1) SELECT * FROM x": "other"} {
		if got := QueryKind(NormalizeQuery(raw)); got != want {
			t.Errorf("%q: got %s want %s", raw, got, want)
		}
	}
	if !strings.Contains(columnStatsSQL, "LIMIT $3") || !strings.Contains(columnStatsSQL, "schemaname = ANY") {
		t.Fatal("column-stat collection must enforce SQL-side limits and schema policy")
	}
	if !strings.Contains(workloadAvailableSQL, "extversion") {
		t.Fatal("workload extension version not detected")
	}
	if strings.Contains(workloadSQL, "pg_stat_database") {
		t.Fatal("workload window must not use the database statistics reset clock")
	}
}

func TestQueryFingerprintNeverContainsLiteralOrComment(t *testing.T) {
	queries := []string{
		"SELECT * FROM public.customer WHERE email='secret@example.org' /* private */",
		"UPDATE public.customer SET token=$tag$do-not-store$tag$ WHERE id=987654",
		"SELECT E'private\\nvalue' FROM public.customer -- confidential",
	}
	for _, raw := range queries {
		normalized := NormalizeQuery(raw)
		fp := queryFingerprint(normalized)
		for _, secret := range []string{"secret@example.org", "private", "do-not-store", "987654", "confidential"} {
			if strings.Contains(normalized, secret) || strings.Contains(fp, secret) {
				t.Fatalf("literal leaked from %q", raw)
			}
		}
	}
}

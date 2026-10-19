package postgres

import (
	"testing"
)

func TestOriginLabelPT(t *testing.T) {
	t.Parallel()
	cases := []struct {
		kind string
		via  []string
		want string
	}{
		{"direct", nil, "direto"},
		{"public", nil, "PUBLIC"},
		{"predefined", []string{"pg_read_all_data"}, "predefinida (pg_read_all_data)"},
		{"role", []string{"g_leitura"}, "via g_leitura"},
		{"role", []string{"g_a", "g_b"}, "via g_a → g_b"},
		{"role", nil, "via role"},
	}
	for _, tc := range cases {
		if got := OriginLabelPT(tc.kind, tc.via); got != tc.want {
			t.Fatalf("OriginLabelPT(%q,%v)=%q want %q", tc.kind, tc.via, got, tc.want)
		}
	}
}

func TestResolveOriginViaGroupRole(t *testing.T) {
	t.Parallel()
	// CA-01: app_report reads vendas.pedido only as member of g_leitura.
	acls := map[aclKey][]aclGrant{
		{objectType: "table", objectName: "vendas.pedido", privilege: "SELECT"}: {
			{grantee: "g_leitura", owner: "app_owner"},
		},
	}
	reach := map[string][]reachEdge{
		"app_report": {{granted: "g_leitura", setOK: true, inherit: true}},
	}
	kind, via, _, _ := resolveOrigin("app_report", "table", "vendas.pedido", "SELECT", acls, reach, true)
	if kind != "role" || len(via) != 1 || via[0] != "g_leitura" {
		t.Fatalf("got kind=%s via=%v, want role via g_leitura", kind, via)
	}
	if label := OriginLabelPT(kind, via); label != "via g_leitura" {
		t.Fatalf("label=%q", label)
	}
}

func TestResolveOriginDirectPublicPredefined(t *testing.T) {
	t.Parallel()
	acls := map[aclKey][]aclGrant{
		{objectType: "table", objectName: "public.t", privilege: "SELECT"}: {
			{grantee: "reader", owner: "owner"},
		},
		{objectType: "table", objectName: "vendas.t", privilege: "SELECT"}: {
			{grantee: "", owner: "owner"},
		},
	}
	reach := map[string][]reachEdge{
		"reader": {{granted: "pg_read_all_data", setOK: true, inherit: true}},
		"etl":    {{granted: "pg_read_all_data", setOK: true, inherit: false}},
	}
	kind, _, _, _ := resolveOrigin("reader", "table", "public.t", "SELECT", acls, reach, true)
	if kind != "direct" {
		t.Fatalf("direct grant: got %s", kind)
	}
	kind, _, _, _ = resolveOrigin("reader", "table", "vendas.t", "SELECT", acls, reach, true)
	if kind != "public" {
		t.Fatalf("public grant: got %s", kind)
	}
	kind, via, inherit, setOK := resolveOrigin("etl", "table", "other.t", "SELECT", map[aclKey][]aclGrant{}, reach, true)
	if kind != "predefined" || len(via) != 1 || via[0] != "pg_read_all_data" || !setOK || inherit {
		t.Fatalf("predefined: kind=%s via=%v inherit=%v set=%v", kind, via, inherit, setOK)
	}
}

func TestResolveOriginOwnerIsDirect(t *testing.T) {
	t.Parallel()
	acls := map[aclKey][]aclGrant{
		{objectType: "table", objectName: "s.t", privilege: "DELETE"}: {
			{grantee: "", owner: "app_owner"},
		},
	}
	kind, via, _, _ := resolveOrigin("app_owner", "table", "s.t", "DELETE", acls, nil, false)
	if kind != "direct" || len(via) != 0 {
		t.Fatalf("owner should be direct, got %s %v", kind, via)
	}
}

func TestSchemaOf(t *testing.T) {
	t.Parallel()
	if got := schemaOf("schema", "vendas"); got != "vendas" {
		t.Fatalf("schema object: %s", got)
	}
	if got := schemaOf("table", "vendas.pedido"); got != "vendas" {
		t.Fatalf("table: %s", got)
	}
}

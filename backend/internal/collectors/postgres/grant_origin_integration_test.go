package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/osmendes/db-auditor/internal/config"
)

// CA-01 + CA-08: origin via group role, with and without inheritance, on the
// version under test. CI Role E2E must run this (not skip) on PG 14–18.
func TestGrantOriginViaGroupRoleIntegration(t *testing.T) {
	dsn := os.Getenv("AUDITOR_TEST_DATABASE_URL")
	if dsn == "" || os.Getenv("AUDITOR_TEST_DISPOSABLE") != "1" {
		t.Skip("requires disposable database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()

	var version int
	if err = conn.QueryRow(ctx, `SELECT current_setting('server_version_num')::int`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	pg16 := version >= 160000
	t.Logf("server_version_num=%d (PostgreSQL 16+ semantics: %v)", version, pg16)

	suffix := strings.ReplaceAll(time.Now().UTC().Format("150405.000000000"), ".", "")
	name := func(base string) string { return "g07_" + base + "_" + suffix }
	q := func(base string) string { return pgx.Identifier{name(base)}.Sanitize() }
	schema := name("vendas")
	table := name("pedido")
	created := []string{}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = conn.Exec(c, `DROP TABLE IF EXISTS `+pgx.Identifier{schema, table}.Sanitize())
		_, _ = conn.Exec(c, `DROP SCHEMA IF EXISTS `+pgx.Identifier{schema}.Sanitize()+` CASCADE`)
		for i := len(created) - 1; i >= 0; i-- {
			_, _ = conn.Exec(c, "DROP ROLE IF EXISTS "+pgx.Identifier{created[i]}.Sanitize())
		}
	})
	exec := func(sql string) {
		t.Helper()
		if _, err := conn.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	role := func(base, attrs string) {
		created = append(created, name(base))
		exec("CREATE ROLE " + q(base) + " " + attrs)
	}

	role("leitura", "NOLOGIN")
	role("report", "LOGIN PASSWORD 'x'")
	role("noinherit", "LOGIN PASSWORD 'x' NOINHERIT")
	exec("CREATE SCHEMA " + pgx.Identifier{schema}.Sanitize())
	exec("CREATE TABLE " + pgx.Identifier{schema, table}.Sanitize() + " (id int)")
	exec("GRANT USAGE ON SCHEMA " + pgx.Identifier{schema}.Sanitize() + " TO " + q("leitura"))
	exec("GRANT SELECT ON " + pgx.Identifier{schema, table}.Sanitize() + " TO " + q("leitura"))
	exec("GRANT " + q("leitura") + " TO " + q("report"))
	if pg16 {
		exec("GRANT " + q("leitura") + " TO " + q("noinherit") + " WITH INHERIT FALSE, SET TRUE")
	} else {
		exec("GRANT " + q("leitura") + " TO " + q("noinherit"))
	}

	origins, _, err := CollectGrantOrigins(ctx, conn, config.Scope{})
	if err != nil {
		t.Fatal(err)
	}
	obj := schema + "." + table
	find := func(account string) *GrantOriginFacts {
		for i := range origins {
			o := &origins[i]
			if o.AccountName == account && o.ObjectName == obj && o.Privilege == "SELECT" && o.ObjectType == "table" {
				return o
			}
		}
		return nil
	}

	got := find(name("report"))
	if got == nil {
		t.Fatalf("missing SELECT origin for app_report-equivalent; sample=%d", len(origins))
	}
	if got.OriginKind != "role" || len(got.ViaChain) == 0 || got.ViaChain[0] != name("leitura") {
		t.Fatalf("CA-01: want via %s, got kind=%s via=%v", name("leitura"), got.OriginKind, got.ViaChain)
	}
	if label := OriginLabelPT(got.OriginKind, got.ViaChain); !strings.Contains(label, name("leitura")) {
		t.Fatalf("label %q should mention group role", label)
	}

	noinh := find(name("noinherit"))
	if pg16 {
		// WITH INHERIT FALSE, SET TRUE: effective privilege still exists via SET ROLE path.
		if noinh == nil {
			t.Fatal("PG16+: NOINHERIT+SET member should still show effective privilege origin")
		}
		if noinh.OriginKind != "role" {
			t.Fatalf("PG16+ noinherit origin kind=%s", noinh.OriginKind)
		}
		if noinh.SetPath == nil || !*noinh.SetPath {
			t.Fatalf("PG16+ expected set_path=true, got %+v", noinh.SetPath)
		}
		if noinh.InheritPath != nil && *noinh.InheritPath {
			t.Fatalf("PG16+ expected inherit_path=false, got %+v", noinh.InheritPath)
		}
	} else if noinh == nil || noinh.OriginKind != "role" {
		t.Fatalf("PG14/15: MEMBER covers NOINHERIT; got %+v", noinh)
	}
}

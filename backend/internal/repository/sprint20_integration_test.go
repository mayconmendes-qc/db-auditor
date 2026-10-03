package repository

import (
	"context"
	"crypto/sha256"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSprint20IdentityInventoryAndPagesIntegration(t *testing.T) {
	dsn := os.Getenv("AUDITOR_TEST_DATABASE_URL")
	if dsn == "" || os.Getenv("AUDITOR_TEST_DISPOSABLE") != "1" {
		t.Skip("disposable integration database not explicitly enabled")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := NewStore(pool)
	var env, run string
	if err := pool.QueryRow(ctx, `INSERT INTO audit_environment(name,type,discovery_mode) VALUES(gen_random_uuid()::text,'self_hosted','single_database') RETURNING id::text`).Scan(&env); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO audit_run(environment_id,profile,status,service_version,collector_version) VALUES($1::uuid,'manual','success','test','test') RETURNING id::text`, env).Scan(&run); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO database_snapshot(audit_run_id,environment_id,database_name) VALUES($1::uuid,$2::uuid,'db')`,
		`INSERT INTO schema_snapshot(audit_run_id,environment_id,database_name,schema_name) VALUES($1::uuid,$2::uuid,'db','public')`,
		`INSERT INTO table_snapshot(audit_run_id,environment_id,database_name,schema_name,table_name) VALUES($1::uuid,$2::uuid,'db','public','orders')`,
	} {
		if _, err := pool.Exec(ctx, statement, run, env); err != nil {
			t.Fatal(err)
		}
	}
	counts, err := store.CountLatestInventory(ctx, env)
	if err != nil || counts == nil || counts.Databases != 1 || counts.Schemas != 1 || counts.Tables != 1 || counts.Status != "complete" {
		t.Fatalf("counts %#v, %v", counts, err)
	}
	username := "user-" + run[:8]
	hash, err := HashAuditorPassword("safe-integration-password")
	if err != nil {
		t.Fatal(err)
	}
	user, err := store.CreateAuditorUser(ctx, username, hash, "auditor", []string{env})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := store.FindAuditorUser(ctx, username)
	if err != nil || loaded == nil || len(loaded.Environments) != 1 || loaded.Environments[0] != env {
		t.Fatalf("user %#v, %v", loaded, err)
	}
	digest := sha256.Sum256([]byte("test-session-" + run))
	if err := store.CreateAuditorSession(ctx, user.ID, digest[:], time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	active, err := store.GetAuditorSession(ctx, digest[:])
	if err != nil || active == nil || active.ID != user.ID {
		t.Fatalf("session %#v, %v", active, err)
	}
	if err := store.DeleteAuditorSession(ctx, digest[:]); err != nil {
		t.Fatal(err)
	}
	active, err = store.GetAuditorSession(ctx, digest[:])
	if err != nil || active != nil {
		t.Fatalf("revoked session %#v, %v", active, err)
	}
	if err := store.LogAuditorOperation(ctx, user, "test", env, run, "success"); err != nil {
		t.Fatal(err)
	}
	items, total, err := store.ListAuditRunsPage(ctx, env, "", "", 20, 0)
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("runs %d/%d %v", len(items), total, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO finding(environment_id,audit_run_id,finding_type,severity,status,title,dedup_key)
		VALUES($1::uuid,$2::uuid,'security.test','high','open','test',gen_random_uuid()::text)`, env, run); err != nil {
		t.Fatal(err)
	}
	findings, findingTotal, err := store.ListFindingsCategoryPage(ctx, env, "security", "open", 20, 0)
	if err != nil || findingTotal != 1 || len(findings) != 1 {
		t.Fatalf("security findings %d/%d %v", len(findings), findingTotal, err)
	}
	allFindings, allTotal, err := store.ListFindingsPage(ctx, env, "", "", "", 20, 0)
	if err != nil || allTotal != 1 || len(allFindings) != 1 {
		t.Fatalf("findings %d/%d %v", len(allFindings), allTotal, err)
	}
}

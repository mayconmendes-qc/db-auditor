package repository

import (
	"context"
	"crypto/sha256"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFirstStageAggregatesAndPagesIntegration(t *testing.T) {
	dsn := os.Getenv("AUDITOR_TEST_DATABASE_URL")
	if dsn == "" || os.Getenv("AUDITOR_TEST_DISPOSABLE") != "1" {
		t.Skip("requires disposable snapshot store")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := NewStore(pool)
	if err := store.EnsureIdentitySchema(ctx); err != nil {
		t.Fatal(err)
	}
	var env, older, newer string
	if err := pool.QueryRow(ctx, `INSERT INTO audit_environment(name,type,discovery_mode)
VALUES(gen_random_uuid()::text,'self_hosted','multi_database') RETURNING id::text`).Scan(&env); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		dst *string
		at  time.Time
	}{
		{&older, time.Now().UTC().Add(-48 * time.Hour)},
		{&newer, time.Now().UTC().Add(-24 * time.Hour)},
	} {
		if err := pool.QueryRow(ctx, `INSERT INTO audit_run(environment_id,profile,status,service_version,collector_version,started_at)
VALUES($1::uuid,'manual','success','test','test',$2) RETURNING id::text`, env, item.at).Scan(item.dst); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []struct {
		run  string
		size int
	}{{older, 100}, {newer, 150}} {
		if _, err := pool.Exec(ctx, `INSERT INTO database_snapshot(audit_run_id,environment_id,database_name,size_bytes)
VALUES($1::uuid,$2::uuid,'db',$3)`, item.run, env, item.size); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO analysis_run(audit_run_id,environment_id,status,analyzer_version)
VALUES($1::uuid,$2::uuid,'success','test')`, newer, env); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO finding(environment_id,audit_run_id,finding_type,severity,status,title,dedup_key)
SELECT $1::uuid,$2::uuid,'security.test','high','open','test','finding-'||n
FROM generate_series(1,600) AS n`, env, newer); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO finding_event(finding_id,audit_run_id,event_type,severity)
SELECT id,$1::uuid,'observed','high' FROM finding WHERE environment_id=$2::uuid ORDER BY dedup_key LIMIT 2`, newer, env); err != nil {
		t.Fatal(err)
	}
	aggregate, err := store.CountFindings(ctx, env)
	if err != nil || aggregate.Total != 600 || aggregate.Open != 600 || aggregate.High != 600 {
		t.Fatalf("finding aggregate %+v: %v", aggregate, err)
	}
	from, to := time.Now().UTC().Add(-72*time.Hour), time.Now().UTC()
	storage, err := store.ListStorageTrend(ctx, env, from, to, "day")
	if err != nil || len(storage) != 2 || storage[0].SizeBytes == nil || *storage[0].SizeBytes != 100 ||
		storage[1].SizeBytes == nil || *storage[1].SizeBytes != 150 {
		t.Fatalf("storage trend %+v: %v", storage, err)
	}
	findings, err := store.ListFindingTrend(ctx, env, from, to, "day")
	if err != nil || len(findings) != 1 || findings[0].Findings == nil || *findings[0].Findings != 2 {
		t.Fatalf("finding trend %+v: %v", findings, err)
	}
	for _, table := range []string{"hypertable_snapshot", "continuous_aggregate_snapshot"} {
		name := "hypertable_name"
		if table == "continuous_aggregate_snapshot" {
			name = "view_name"
		}
		// Table and column names are fixed test constants, never user input.
		query := "INSERT INTO " + table + "(audit_run_id,environment_id,database_name,schema_name," + name + ") SELECT $1::uuid,$2::uuid,'db','public','obj'||n FROM generate_series(1,3)n"
		if _, err := pool.Exec(ctx, query, newer, env); err != nil {
			t.Fatal(err)
		}
	}
	hypertables, total, err := store.ListHypertableSnapshotsPage(ctx, InventoryFilter{EnvironmentID: env, Q: "obj", Limit: 2, Offset: 1})
	if err != nil || total != 3 || len(hypertables) != 2 {
		t.Fatalf("hypertables %d/%d: %v", len(hypertables), total, err)
	}
	caggs, total, err := store.ListCAGGSnapshotsPage(ctx, InventoryFilter{EnvironmentID: env, Q: "obj", Limit: 2, Offset: 1})
	if err != nil || total != 3 || len(caggs) != 2 {
		t.Fatalf("caggs %d/%d: %v", len(caggs), total, err)
	}
	capabilities, err := store.CountLatestCapabilities(ctx, env)
	if err != nil || capabilities.TotalStorageBytes != 150 || capabilities.Hypertables != 3 {
		t.Fatalf("capabilities %+v: %v", capabilities, err)
	}
	successful, failed, err := store.CountRecentRuns(ctx, env)
	if err != nil || successful != 2 || failed != 0 {
		t.Fatalf("recent runs %d/%d: %v", successful, failed, err)
	}
	key := sha256.Sum256([]byte("integration-login-" + env))
	if count, err := store.RecordLoginAttempt(ctx, key[:], time.Minute); err != nil || count != 1 {
		t.Fatalf("first login count %d: %v", count, err)
	}
	if count, err := store.RecordLoginAttempt(ctx, key[:], time.Minute); err != nil || count != 2 {
		t.Fatalf("second login count %d: %v", count, err)
	}
	if err := store.ClearLoginAttempt(ctx, key[:]); err != nil {
		t.Fatal(err)
	}
}

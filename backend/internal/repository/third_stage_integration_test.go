package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osmendes/db-auditor/internal/quality"
)

func TestThirdStageHistoryAndMeasurementIntegration(t *testing.T) {
	if os.Getenv("AUDITOR_TEST_DATABASE_URL") == "" || os.Getenv("AUDITOR_TEST_DISPOSABLE") != "1" {
		t.Skip("requires disposable database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, os.Getenv("AUDITOR_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := NewStore(pool)
	var env string
	if err = pool.QueryRow(ctx, `INSERT INTO audit_environment(name,type,discovery_mode) VALUES(gen_random_uuid()::text,'self_hosted','single_database') RETURNING id::text`).Scan(&env); err != nil {
		t.Fatal(err)
	}
	capabilities, err := store.GetEnvironmentCapabilities(ctx, env, false)
	if err != nil || capabilities == nil || capabilities.Engine != "postgresql" {
		t.Fatalf("capabilities: %+v %v", capabilities, err)
	}
	request := quality.Request{Database: "db", Schema: "public", Table: "orders", Limit: 10, ExpectedNonNull: []string{"account_id"}}
	scan, err := store.SaveQualityScan(ctx, env, "tester", request, quality.Result{SampledRows: 3, Issues: []quality.Issue{{Kind: "null", Column: "account_id", AffectedRows: 1, SampledRows: 3}}})
	if err != nil || len(scan.Issues) != 1 {
		t.Fatalf("quality scan: %+v %v", scan, err)
	}
	scan, err = store.UpdateQualityIssue(ctx, env, scan.Issues[0].ID, "tester", "planned", "dba", "confirmar regra", "aguardando janela")
	if err != nil || scan.Issues[0].Status != "planned" {
		t.Fatalf("quality action: %+v %v", scan, err)
	}
	var events int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM quality_issue_event WHERE issue_id=$1::uuid`, scan.Issues[0].ID).Scan(&events); err != nil || events != 1 {
		t.Fatalf("quality event count %d: %v", events, err)
	}
	if footprint, err := store.GetStorageFootprint(ctx); err != nil || footprint.QualityHistoryBytes <= 0 {
		t.Fatalf("storage: %+v %v", footprint, err)
	}

	runs := make([]string, 2)
	for i := range runs {
		started := time.Now().UTC().Add(time.Duration(i-2) * time.Hour)
		if err = pool.QueryRow(ctx, `INSERT INTO audit_run(environment_id,profile,status,service_version,collector_version,started_at) VALUES($1::uuid,'manual','success','test','test',$2) RETURNING id::text`, env, started).Scan(&runs[i]); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO database_snapshot(audit_run_id,environment_id,database_name) VALUES($1::uuid,$2::uuid,'db')`, runs[i], env); err != nil {
			t.Fatal(err)
		}
		for _, collector := range scoreCollectors {
			if _, err = pool.Exec(ctx, `INSERT INTO audit_run_coverage(audit_run_id,environment_id,collector_name,database_name,status) VALUES($1::uuid,$2::uuid,$3,'db','success')`, runs[i], env, collector); err != nil {
				t.Fatal(err)
			}
		}
		if _, err = pool.Exec(ctx, `INSERT INTO analysis_run(audit_run_id,environment_id,status,analyzer_version) VALUES($1::uuid,$2::uuid,'success','test')`, runs[i], env); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO table_snapshot(audit_run_id,environment_id,database_name,schema_name,table_name,total_size_bytes) VALUES($1::uuid,$2::uuid,'db','public','orders',$3)`, runs[i], env, 1000+i*100); err != nil {
			t.Fatal(err)
		}
	}
	var finding string
	if err = pool.QueryRow(ctx, `INSERT INTO finding(environment_id,audit_run_id,finding_type,severity,title,dedup_key,database_name,schema_name,object_name) VALUES($1::uuid,$2::uuid,'model.no_primary_key','high','test',gen_random_uuid()::text,'db','public','orders') RETURNING id::text`, env, runs[0]).Scan(&finding); err != nil {
		t.Fatal(err)
	}
	for _, run := range runs {
		if _, err = pool.Exec(ctx, `INSERT INTO finding_event(finding_id,audit_run_id,event_type,severity,category) VALUES($1::uuid,$2::uuid,'observed','high','model')`, finding, run); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = store.UpdateFindingAction(ctx, finding, "tester", ActionProgress{Status: "validated", Owner: "dba", Justification: "conferido", Result: "feito externamente"}); err != nil {
		t.Fatal(err)
	}
	measurement, err := store.RecordActionMeasurement(ctx, finding, runs[0], runs[1], "finding_observed", "achado deve desaparecer", "7 dias sem alteração de carga", "tester")
	if err != nil || measurement == nil || !measurement.Comparable {
		t.Fatalf("measurement: %+v %v", measurement, err)
	}
	tracked, err := store.ListTrackedActions(ctx, env)
	if err != nil || len(tracked) != 1 || tracked[0].LatestMeasurement == nil {
		t.Fatalf("tracked action: %+v %v", tracked, err)
	}
	action, err := store.GetFindingAction(ctx, finding)
	if err != nil || action.Status != "in_review" {
		t.Fatalf("regression must reopen review: %+v %v", action, err)
	}
	job, err := store.CreateReportJob(ctx, ReportRequest{EnvironmentID: env, AuditRunID: runs[1], Type: "executive"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO report_artifact(report_job_id,content,sha256,size_bytes,filename,content_type) VALUES($1::uuid,$2,'test',1,'test.pdf','application/pdf')`, job.ID, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE report_job SET status='success',expires_at=now()-interval '1 second' WHERE id=$1::uuid`, job.ID); err != nil {
		t.Fatal(err)
	}
	fresh, err := store.CreateReportJob(ctx, ReportRequest{EnvironmentID: env, AuditRunID: runs[1], Type: "executive"})
	if err != nil || fresh.ID == job.ID {
		t.Fatalf("expired job was overwritten: %+v %v", fresh, err)
	}
	deleted, err := store.CleanupExpiredReports(ctx)
	if err != nil || deleted < 1 {
		t.Fatalf("artifact cleanup: %d %v", deleted, err)
	}
	old, err := store.GetReportJob(ctx, env, job.ID)
	if err != nil || old == nil || old.Status != "success" {
		t.Fatalf("provenance lost: %+v %v", old, err)
	}
}

package repository

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type digestG6Fixture struct {
	t     *testing.T
	ctx   context.Context
	pool  *pgxpool.Pool
	store *Store
	env   string
}

func newDigestG6Fixture(t *testing.T) *digestG6Fixture {
	t.Helper()
	dsn := os.Getenv("AUDITOR_TEST_DATABASE_URL")
	if dsn == "" || os.Getenv("AUDITOR_TEST_DISPOSABLE") != "1" {
		t.Skip("requires disposable database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	f := &digestG6Fixture{t: t, ctx: ctx, pool: pool, store: NewStore(pool)}
	if err = pool.QueryRow(ctx, `INSERT INTO audit_environment(name,type,discovery_mode,engine)
VALUES(gen_random_uuid()::text,'self_hosted','single_database','postgresql') RETURNING id::text`).Scan(&f.env); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *digestG6Fixture) completeRun(at time.Time, fp string, calls int64, totalMS float64, reset time.Time) string {
	f.t.Helper()
	var id string
	if err := f.pool.QueryRow(f.ctx, `INSERT INTO audit_run(environment_id,profile,status,service_version,collector_version,started_at,finished_at)
VALUES($1::uuid,'manual','success','test','test',$2,$2) RETURNING id::text`, f.env, at).Scan(&id); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO analysis_run(audit_run_id,environment_id,status,analyzer_version)
VALUES($1::uuid,$2::uuid,'success','test')`, id, f.env); err != nil {
		f.t.Fatal(err)
	}
	mean := 0.0
	if calls > 0 {
		mean = totalMS / float64(calls)
	}
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO workload_snapshot(
audit_run_id,environment_id,database_name,query_fingerprint,calls,total_exec_time_ms,mean_exec_time_ms,referenced_objects,stats_reset,collected_at)
VALUES($1::uuid,$2::uuid,'app',$3,$4,$5,$6,ARRAY['public.orders'],$7,$8)`,
		id, f.env, fp, calls, totalMS, mean, reset, at); err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f *digestG6Fixture) countRegressionAlerts() int {
	f.t.Helper()
	var n int
	if err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM regression_alert WHERE environment_id=$1::uuid`, f.env).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

// Integration counterpart of CA-04 / RF-08 against the real store.
func TestDigestLinksOpenWorkloadAlertIntegration(t *testing.T) {
	f := newDigestG6Fixture(t)
	now := time.Now().UTC().Truncate(time.Second)
	reset := now.Add(-30 * 24 * time.Hour)
	fp := "fp-reg-g6"

	priorFirst := f.completeRun(now.Add(-13*24*time.Hour), fp, 100, 2000, reset)
	_ = f.completeRun(now.Add(-8*24*time.Hour), fp, 200, 4000, reset)
	currFirst := f.completeRun(now.Add(-6*24*time.Hour), fp, 200, 4000, reset)
	_ = f.completeRun(now.Add(-1*24*time.Hour), fp, 400, 14000, reset)

	var alertID string
	if err := f.pool.QueryRow(f.ctx, `INSERT INTO regression_alert(environment_id,baseline_run_id,audit_run_id,category,evidence,status)
VALUES($1::uuid,$2::uuid,$3::uuid,'workload',$4::jsonb,'open') RETURNING id::text`,
		f.env, priorFirst, currFirst, `{"keys":["`+fp+`"],"current":1}`).Scan(&alertID); err != nil {
		t.Fatal(err)
	}
	before := f.countRegressionAlerts()

	d, err := f.store.GenerateQueryDigest(f.ctx, f.env, now)
	if err != nil {
		t.Fatal(err)
	}
	if d == nil || d.State != "ok" {
		t.Fatalf("digest=%+v", d)
	}
	after := f.countRegressionAlerts()
	if after != before {
		t.Fatalf("regression_alert rows changed: before=%d after=%d", before, after)
	}
	linked := false
	for _, it := range d.Items {
		if it.Fingerprint == fp && it.RegressionAlertID != nil && *it.RegressionAlertID == alertID {
			linked = true
		}
	}
	if !linked {
		t.Fatalf("expected item to link alert %s; items=%+v", alertID, d.Items)
	}
	if strings.Contains(d.Markdown, "SELECT ") || strings.Contains(d.Markdown, "INSERT INTO") {
		t.Fatal("markdown must not contain SQL literals")
	}
}

func TestDigestDeliveryFailureRecordedAndRetriedIntegration(t *testing.T) {
	f := newDigestG6Fixture(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "SELECT ") || strings.Contains(string(body), "INSERT INTO") {
			t.Errorf("webhook leaked SQL: %s", body)
		}
		if n == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Bypass ValidateNotificationDestination (blocks loopback) — delivery reads secrets directly.
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO notification_destination(environment_id,channel,target,format,active,created_by)
VALUES($1::uuid,'webhook',$2,'generic',true,'test')`, f.env, srv.URL); err != nil {
		t.Fatal(err)
	}

	d := &QueryDigest{
		ID: "", EnvironmentID: f.env, State: "ok",
		RegressionCount: 1, Markdown: "# Digest\n",
	}
	if err := f.pool.QueryRow(f.ctx, `INSERT INTO query_digest(environment_id,state,markdown,regression_count)
VALUES($1::uuid,'ok','# Digest',1) RETURNING id::text`, f.env).Scan(&d.ID); err != nil {
		t.Fatal(err)
	}

	if err := f.store.DeliverQueryDigest(f.ctx, d); err != nil {
		t.Fatal(err)
	}
	var failed int
	if err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM query_digest_delivery WHERE digest_id=$1::uuid AND status='failed'`, d.ID).Scan(&failed); err != nil {
		t.Fatal(err)
	}
	if failed == 0 {
		t.Fatal("expected failed delivery row")
	}
	var claimed bool
	if err := f.pool.QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM alert_delivery WHERE environment_id=$1::uuid AND dedup_key=$2)`, f.env, d.ID).Scan(&claimed); err != nil {
		t.Fatal(err)
	}
	if claimed {
		t.Fatal("failed delivery must not claim alert_delivery")
	}

	if err := f.store.RetryFailedQueryDigestDeliveries(f.ctx, f.env); err != nil {
		t.Fatal(err)
	}
	if calls.Load() < 2 {
		t.Fatalf("retry did not re-attempt webhook: calls=%d", calls.Load())
	}
	var sent int
	if err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM query_digest_delivery WHERE digest_id=$1::uuid AND status='sent'`, d.ID).Scan(&sent); err != nil {
		t.Fatal(err)
	}
	if sent == 0 {
		t.Fatal("expected sent delivery after retry")
	}
	if err := f.pool.QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM alert_delivery WHERE environment_id=$1::uuid AND dedup_key=$2)`, f.env, d.ID).Scan(&claimed); err != nil {
		t.Fatal(err)
	}
	if !claimed {
		t.Fatal("successful retry must ClaimAlert")
	}
}

func TestDigestRecipientsPerEnvironmentWithGlobalFallbackIntegration(t *testing.T) {
	dsn := os.Getenv("AUDITOR_TEST_DATABASE_URL")
	if dsn == "" || os.Getenv("AUDITOR_TEST_DISPOSABLE") != "1" {
		t.Skip("requires disposable database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	store := NewStore(pool)

	var envA, envB string
	if err = pool.QueryRow(ctx, `INSERT INTO audit_environment(name,type,discovery_mode) VALUES(gen_random_uuid()::text,'self_hosted','single_database') RETURNING id::text`).Scan(&envA); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO audit_environment(name,type,discovery_mode) VALUES(gen_random_uuid()::text,'self_hosted','single_database') RETURNING id::text`).Scan(&envB); err != nil {
		t.Fatal(err)
	}

	var hitsA, hitsGlobal atomic.Int32
	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitsA.Add(1)
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		if env, _ := payload["environment_id"].(string); env != "" && env != envA {
			t.Errorf("A webhook got env %s", env)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srvA.Close()
	srvGlobal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitsGlobal.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srvGlobal.Close()
	t.Setenv("AUDITOR_ALERT_WEBHOOK_URL", srvGlobal.URL)
	t.Setenv("AUDITOR_ALERT_EMAIL_TO", "")
	t.Setenv("AUDITOR_ALERT_SMTP_HOST", "")

	if _, err = pool.Exec(ctx, `INSERT INTO notification_destination(environment_id,channel,target,format,active,created_by)
VALUES($1::uuid,'webhook',$2,'generic',true,'test')`, envA, srvA.URL); err != nil {
		t.Fatal(err)
	}

	insertDigest := func(env string) *QueryDigest {
		t.Helper()
		d := &QueryDigest{EnvironmentID: env, State: "ok"}
		if err := pool.QueryRow(ctx, `INSERT INTO query_digest(environment_id,state,markdown) VALUES($1::uuid,'ok','# d') RETURNING id::text`, env).Scan(&d.ID); err != nil {
			t.Fatal(err)
		}
		return d
	}
	if err := store.DeliverQueryDigest(ctx, insertDigest(envA)); err != nil {
		t.Fatal(err)
	}
	if err := store.DeliverQueryDigest(ctx, insertDigest(envB)); err != nil {
		t.Fatal(err)
	}
	if hitsA.Load() < 1 {
		t.Fatal("env A destination not hit")
	}
	if hitsGlobal.Load() < 1 {
		t.Fatal("env B must fall back to global webhook")
	}
}

func TestDigestNeverMixesEnvironmentsIntegration(t *testing.T) {
	dsn := os.Getenv("AUDITOR_TEST_DATABASE_URL")
	if dsn == "" || os.Getenv("AUDITOR_TEST_DISPOSABLE") != "1" {
		t.Skip("requires disposable database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	store := NewStore(pool)

	var envA, envB string
	if err = pool.QueryRow(ctx, `INSERT INTO audit_environment(name,type,discovery_mode) VALUES(gen_random_uuid()::text,'self_hosted','single_database') RETURNING id::text`).Scan(&envA); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO audit_environment(name,type,discovery_mode) VALUES(gen_random_uuid()::text,'self_hosted','single_database') RETURNING id::text`).Scan(&envB); err != nil {
		t.Fatal(err)
	}

	var hitB atomic.Int32
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hitB.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srvB.Close()
	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), envB) {
			t.Errorf("A webhook received B id: %s", body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srvA.Close()
	t.Setenv("AUDITOR_ALERT_WEBHOOK_URL", "")

	if _, err = pool.Exec(ctx, `INSERT INTO notification_destination(environment_id,channel,target,format,active,created_by)
VALUES($1::uuid,'webhook',$2,'generic',true,'test'),($3::uuid,'webhook',$4,'generic',true,'test')`,
		envA, srvA.URL, envB, srvB.URL); err != nil {
		t.Fatal(err)
	}

	var idA string
	if err = pool.QueryRow(ctx, `INSERT INTO query_digest(environment_id,state,markdown) VALUES($1::uuid,'ok','# a') RETURNING id::text`, envA).Scan(&idA); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO query_digest_item(digest_id,classification,severity,fingerprint,sort_rank)
VALUES($1::uuid,'regressao','media','fp-only-a',0)`, idA); err != nil {
		t.Fatal(err)
	}
	var idB string
	if err = pool.QueryRow(ctx, `INSERT INTO query_digest(environment_id,state,markdown) VALUES($1::uuid,'ok','# b') RETURNING id::text`, envB).Scan(&idB); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO query_digest_item(digest_id,classification,severity,fingerprint,sort_rank)
VALUES($1::uuid,'regressao','media','fp-only-b',0)`, idB); err != nil {
		t.Fatal(err)
	}

	listA, _, err := store.ListQueryDigests(ctx, envA, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range listA {
		if d.EnvironmentID != envA {
			t.Fatalf("list A returned env %s", d.EnvironmentID)
		}
		full, err := store.GetQueryDigest(ctx, envA, d.ID)
		if err != nil || full == nil {
			t.Fatalf("get: %v", err)
		}
		for _, it := range full.Items {
			if it.Fingerprint == "fp-only-b" {
				t.Fatal("digest A contained fingerprint from B")
			}
		}
	}
	if err := store.DeliverQueryDigest(ctx, &QueryDigest{ID: idA, EnvironmentID: envA, State: "ok"}); err != nil {
		t.Fatal(err)
	}
	if hitB.Load() != 0 {
		t.Fatal("delivery for A must not hit B destination")
	}
}

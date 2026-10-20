package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics is a minimal Prometheus-compatible registry (stdlib only).
type Metrics struct {
	startedAt time.Time

	httpRequests   sync.Map // key method|path|code -> *atomic.Uint64
	httpDurationNs sync.Map // key method|path -> *atomic.Uint64 (sum)
	httpDurationN  sync.Map // key method|path -> *atomic.Uint64 (count)

	auditRunsTotal  atomic.Uint64
	auditRunsFailed atomic.Uint64
	backupAge       atomic.Uint64 // seconds; 0 means not yet observed

	// Plan 06 — query digest
	queryDigestsOK              atomic.Uint64
	queryDigestsNoWindow        atomic.Uint64
	queryDigestsNotApplicable   atomic.Uint64
	queryDigestDeliveriesFailed atomic.Uint64

	// Plan 01 — migration-risk
	migrationAnalyses    sync.Map // result -> *atomic.Uint64
	migrationParseErrors atomic.Uint64
	envTokenRequests     sync.Map // scope|result -> *atomic.Uint64

	// Plan 04 — schema drift
	schemaChangeEvents    sync.Map // kind -> *atomic.Uint64
	alertDeliveriesFailed sync.Map // kind -> *atomic.Uint64

	// Plan 02 — cost-lens
	costCalculationsPriced   atomic.Uint64
	costCalculationsUnpriced atomic.Uint64

	// Plan 09 — timescale retention advisor
	retentionSimulations         atomic.Uint64
	retentionSimulationsLowRatio atomic.Uint64

	// Plan 07 — access review
	accessReviewCampaigns sync.Map // status -> *atomic.Uint64
	accessReviewPending   atomic.Int64
	lastLoginIngest       sync.Map // result -> *atomic.Uint64 (accepted|rejected)

	// Plan 05 — PII scanner
	piiScans           sync.Map
	piiColumnsByMethod sync.Map
	piiScanCeilings    atomic.Uint64
	piiExportRows      atomic.Uint64

	// Plan 08 — target restore drills
	targetRestoreDrillAge sync.Map // environment -> *atomic.Uint64 (seconds)
	targetRestoreRTO      sync.Map // environment -> *atomic.Uint64 (seconds)

	// Plan 03 — quality contracts
	qualityContractChecks     sync.Map // result -> *atomic.Uint64
	qualityContractDurationNs sync.Map // result -> *atomic.Uint64 (sum)
	qualityContractDurationN  sync.Map // result -> *atomic.Uint64 (count)
}

// DefaultMetrics is the process-wide registry.
var DefaultMetrics = NewMetrics()

// NewMetrics creates a metrics registry.
func NewMetrics() *Metrics {
	return &Metrics{startedAt: time.Now().UTC()}
}

func (m *Metrics) incMap(store *sync.Map, key string) {
	v, _ := store.LoadOrStore(key, &atomic.Uint64{})
	v.(*atomic.Uint64).Add(1)
}

func (m *Metrics) addMap(store *sync.Map, key string, n uint64) {
	v, _ := store.LoadOrStore(key, &atomic.Uint64{})
	v.(*atomic.Uint64).Add(n)
}

// ObserveHTTP records one completed HTTP request.
func (m *Metrics) ObserveHTTP(method, path string, status int, d time.Duration) {
	path = normalizePath(path)
	codeKey := fmt.Sprintf("%s|%s|%d", method, path, status)
	m.incMap(&m.httpRequests, codeKey)
	durKey := fmt.Sprintf("%s|%s", method, path)
	if d < 0 {
		d = 0
	}
	m.addMap(&m.httpDurationNs, durKey, uint64(d.Nanoseconds())) //nolint:gosec // G115: negative durations are clamped above.
	m.incMap(&m.httpDurationN, durKey)
}

// SetSnapshotBackupAge records seconds since the last successful snapshot dump.
func (m *Metrics) SetSnapshotBackupAge(seconds uint64) {
	m.backupAge.Store(seconds)
}
func (m *Metrics) IncAuditRun(failed bool) {
	m.auditRunsTotal.Add(1)
	if failed {
		m.auditRunsFailed.Add(1)
	}
}

// IncMigrationAnalysis increments auditor_migration_analyses_total{result}.
func (m *Metrics) IncMigrationAnalysis(result string) {
	if result == "" {
		result = "unknown"
	}
	m.incMap(&m.migrationAnalyses, result)
}

// IncMigrationParseError increments auditor_migration_parse_errors_total.
func (m *Metrics) IncMigrationParseError() {
	m.migrationParseErrors.Add(1)
}

// IncEnvTokenRequest increments auditor_env_token_requests_total{scope,result}.
func (m *Metrics) IncEnvTokenRequest(scope, result string) {
	m.incMap(&m.envTokenRequests, scope+"|"+result)
}

// IncQueryDigest records one generated digest by result state.
func (m *Metrics) IncQueryDigest(result string) {
	switch result {
	case "ok":
		m.queryDigestsOK.Add(1)
	case "no_comparable_window":
		m.queryDigestsNoWindow.Add(1)
	case "not_applicable":
		m.queryDigestsNotApplicable.Add(1)
	default:
		m.queryDigestsOK.Add(1)
	}
}

// IncQueryDigestDeliveryFailed records one failed digest delivery attempt.
func (m *Metrics) IncQueryDigestDeliveryFailed() {
	m.queryDigestDeliveriesFailed.Add(1)
}

// IncSchemaChangeEvent records one timeline event by change kind.
func (m *Metrics) IncSchemaChangeEvent(kind string) {
	if kind == "" {
		kind = "unknown"
	}
	m.incMap(&m.schemaChangeEvents, kind)
}

// IncAlertDeliveryFailed records one failed alert delivery by kind (RNF-05).
func (m *Metrics) IncAlertDeliveryFailed(kind string) {
	if kind == "" {
		kind = "unknown"
	}
	m.incMap(&m.alertDeliveriesFailed, kind)
}

// IncCostCalculation increments auditor_cost_calculations_total{priced}.
func (m *Metrics) IncCostCalculation(priced bool) {
	if priced {
		m.costCalculationsPriced.Add(1)
	} else {
		m.costCalculationsUnpriced.Add(1)
	}
}

// IncRetentionSimulation increments auditor_retention_simulations_total (RNF-05).
func (m *Metrics) IncRetentionSimulation(lowConfidenceRatio bool) {
	m.retentionSimulations.Add(1)
	if lowConfidenceRatio {
		m.retentionSimulationsLowRatio.Add(1)
	}
}

// IncAccessReviewCampaign increments auditor_access_review_campaigns_total{status}.
func (m *Metrics) IncAccessReviewCampaign(status string) {
	if status == "" {
		status = "unknown"
	}
	m.incMap(&m.accessReviewCampaigns, status)
}

// AddAccessReviewPendingDelta adjusts the pending-lines gauge (can be negative).
func (m *Metrics) AddAccessReviewPendingDelta(delta int) {
	m.accessReviewPending.Add(int64(delta))
}

// SetAccessReviewPending sets the pending-lines gauge from a store recount.
func (m *Metrics) SetAccessReviewPending(n int) {
	m.accessReviewPending.Store(int64(n))
}

// IncLastLoginIngest increments auditor_last_login_ingest_total{result}.
func (m *Metrics) IncLastLoginIngest(result string) {
	if result == "" {
		result = "unknown"
	}
	m.incMap(&m.lastLoginIngest, result)
}

// IncPIIScan increments auditor_pii_scans_total{status,content}.
func (m *Metrics) IncPIIScan(status string, content bool) {
	if status == "" {
		status = "unknown"
	}
	contentLabel := "false"
	if content {
		contentLabel = "true"
	}
	m.incMap(&m.piiScans, status+"|"+contentLabel)
	if status == "partial_success" {
		m.piiScanCeilings.Add(1)
	}
}

func (m *Metrics) IncPIIColumnMethod(method string, n int) {
	if method == "" || n <= 0 {
		return
	}
	m.addMap(&m.piiColumnsByMethod, method, uint64(n))
}

func (m *Metrics) AddPIIExportRows(n int) {
	if n > 0 {
		m.piiExportRows.Add(uint64(n))
	}
}

// SetTargetRestoreDrillAge records seconds since the last successful target restore drill.
func (m *Metrics) SetTargetRestoreDrillAge(environment string, seconds uint64) {
	if environment == "" {
		environment = "unknown"
	}
	v, _ := m.targetRestoreDrillAge.LoadOrStore(environment, &atomic.Uint64{})
	v.(*atomic.Uint64).Store(seconds)
}

// SetTargetRestoreRTO records the RTO seconds of the last successful target restore drill.
func (m *Metrics) SetTargetRestoreRTO(environment string, seconds uint64) {
	if environment == "" {
		environment = "unknown"
	}
	v, _ := m.targetRestoreRTO.LoadOrStore(environment, &atomic.Uint64{})
	v.(*atomic.Uint64).Store(seconds)
}

// ObserveQualityContractCheck records one contract check by result and duration (RNF-05).
func (m *Metrics) ObserveQualityContractCheck(result string, d time.Duration) {
	if result == "" {
		result = "unknown"
	}
	m.incMap(&m.qualityContractChecks, result)
	if d < 0 {
		d = 0
	}
	m.addMap(&m.qualityContractDurationNs, result, uint64(d.Nanoseconds()))
	m.incMap(&m.qualityContractDurationN, result)
}

func normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	// Keep cardinality low: collapse UUIDs and numeric ids in path segments.
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "" {
			continue
		}
		if looksLikeID(p) {
			parts[i] = ":id"
		}
	}
	return strings.Join(parts, "/")
}

func looksLikeID(s string) bool {
	if len(s) == 36 && strings.Count(s, "-") == 4 {
		return true
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// Handler exposes Prometheus text exposition format.
func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		var b strings.Builder
		b.WriteString("# HELP auditor_up Always 1 while process is alive.\n")
		b.WriteString("# TYPE auditor_up gauge\n")
		b.WriteString("auditor_up 1\n")
		b.WriteString("# HELP auditor_snapshot_backup_age_seconds Seconds since the last successful snapshot-store dump. Grows when the job does not run.\n")
		b.WriteString("# TYPE auditor_snapshot_backup_age_seconds gauge\n")
		fmt.Fprintf(&b, "auditor_snapshot_backup_age_seconds %d\n", m.backupAge.Load())
		b.WriteString("# HELP auditor_process_start_time_seconds Process start time.\n")
		b.WriteString("# TYPE auditor_process_start_time_seconds gauge\n")
		fmt.Fprintf(&b, "auditor_process_start_time_seconds %d\n", m.startedAt.Unix())

		b.WriteString("# HELP auditor_http_requests_total HTTP requests by method, path and status.\n")
		b.WriteString("# TYPE auditor_http_requests_total counter\n")
		keys := make([]string, 0)
		m.httpRequests.Range(func(k, _ any) bool {
			keys = append(keys, k.(string))
			return true
		})
		sort.Strings(keys)
		for _, k := range keys {
			v, _ := m.httpRequests.Load(k)
			parts := strings.SplitN(k, "|", 3)
			if len(parts) != 3 {
				continue
			}
			fmt.Fprintf(&b, "auditor_http_requests_total{method=%q,path=%q,code=%q} %d\n",
				parts[0], parts[1], parts[2], v.(*atomic.Uint64).Load())
		}

		b.WriteString("# HELP auditor_http_request_duration_seconds_sum Sum of request durations.\n")
		b.WriteString("# TYPE auditor_http_request_duration_seconds_sum counter\n")
		b.WriteString("# HELP auditor_http_request_duration_seconds_count Count of observed durations.\n")
		b.WriteString("# TYPE auditor_http_request_duration_seconds_count counter\n")
		dkeys := make([]string, 0)
		m.httpDurationNs.Range(func(k, _ any) bool {
			dkeys = append(dkeys, k.(string))
			return true
		})
		sort.Strings(dkeys)
		for _, k := range dkeys {
			sumV, _ := m.httpDurationNs.Load(k)
			cntV, _ := m.httpDurationN.Load(k)
			parts := strings.SplitN(k, "|", 2)
			if len(parts) != 2 {
				continue
			}
			sum := float64(sumV.(*atomic.Uint64).Load()) / 1e9
			cnt := cntV.(*atomic.Uint64).Load()
			fmt.Fprintf(&b, "auditor_http_request_duration_seconds_sum{method=%q,path=%q} %g\n", parts[0], parts[1], sum)
			fmt.Fprintf(&b, "auditor_http_request_duration_seconds_count{method=%q,path=%q} %d\n", parts[0], parts[1], cnt)
		}

		b.WriteString("# HELP auditor_audit_runs_total Audit runs started.\n")
		b.WriteString("# TYPE auditor_audit_runs_total counter\n")
		fmt.Fprintf(&b, "auditor_audit_runs_total %d\n", m.auditRunsTotal.Load())
		b.WriteString("# HELP auditor_audit_runs_failed_total Audit runs that finished FAILED.\n")
		b.WriteString("# TYPE auditor_audit_runs_failed_total counter\n")
		fmt.Fprintf(&b, "auditor_audit_runs_failed_total %d\n", m.auditRunsFailed.Load())

		b.WriteString("# HELP auditor_query_digests_total Query digests generated by result.\n")
		b.WriteString("# TYPE auditor_query_digests_total counter\n")
		fmt.Fprintf(&b, "auditor_query_digests_total{result=%q} %d\n", "ok", m.queryDigestsOK.Load())
		fmt.Fprintf(&b, "auditor_query_digests_total{result=%q} %d\n", "no_comparable_window", m.queryDigestsNoWindow.Load())
		fmt.Fprintf(&b, "auditor_query_digests_total{result=%q} %d\n", "not_applicable", m.queryDigestsNotApplicable.Load())
		b.WriteString("# HELP auditor_query_digest_deliveries_failed_total Failed query digest delivery attempts.\n")
		b.WriteString("# TYPE auditor_query_digest_deliveries_failed_total counter\n")
		fmt.Fprintf(&b, "auditor_query_digest_deliveries_failed_total %d\n", m.queryDigestDeliveriesFailed.Load())

		b.WriteString("# HELP auditor_migration_analyses_total Migration analyses accepted by result.\n")
		b.WriteString("# TYPE auditor_migration_analyses_total counter\n")
		mkeys := make([]string, 0)
		m.migrationAnalyses.Range(func(k, _ any) bool {
			mkeys = append(mkeys, k.(string))
			return true
		})
		sort.Strings(mkeys)
		for _, k := range mkeys {
			v, _ := m.migrationAnalyses.Load(k)
			fmt.Fprintf(&b, "auditor_migration_analyses_total{result=%q} %d\n", k, v.(*atomic.Uint64).Load())
		}
		b.WriteString("# HELP auditor_migration_parse_errors_total Migration analysis items marked unparsed.\n")
		b.WriteString("# TYPE auditor_migration_parse_errors_total counter\n")
		fmt.Fprintf(&b, "auditor_migration_parse_errors_total %d\n", m.migrationParseErrors.Load())

		b.WriteString("# HELP auditor_env_token_requests_total Environment API token authorizations.\n")
		b.WriteString("# TYPE auditor_env_token_requests_total counter\n")
		tkeys := make([]string, 0)
		m.envTokenRequests.Range(func(k, _ any) bool {
			tkeys = append(tkeys, k.(string))
			return true
		})
		sort.Strings(tkeys)
		for _, k := range tkeys {
			v, _ := m.envTokenRequests.Load(k)
			parts := strings.SplitN(k, "|", 2)
			if len(parts) != 2 {
				continue
			}
			fmt.Fprintf(&b, "auditor_env_token_requests_total{scope=%q,result=%q} %d\n",
				parts[0], parts[1], v.(*atomic.Uint64).Load())
		}

		b.WriteString("# HELP auditor_schema_change_events_total Schema timeline events by change kind.\n")
		b.WriteString("# TYPE auditor_schema_change_events_total counter\n")
		skeys := make([]string, 0)
		m.schemaChangeEvents.Range(func(k, _ any) bool {
			skeys = append(skeys, k.(string))
			return true
		})
		sort.Strings(skeys)
		for _, k := range skeys {
			v, _ := m.schemaChangeEvents.Load(k)
			fmt.Fprintf(&b, "auditor_schema_change_events_total{kind=%q} %d\n", k, v.(*atomic.Uint64).Load())
		}
		b.WriteString("# HELP auditor_alert_deliveries_failed_total Failed alert delivery attempts by kind.\n")
		b.WriteString("# TYPE auditor_alert_deliveries_failed_total counter\n")
		akeys := make([]string, 0)
		m.alertDeliveriesFailed.Range(func(k, _ any) bool {
			akeys = append(akeys, k.(string))
			return true
		})
		sort.Strings(akeys)
		for _, k := range akeys {
			v, _ := m.alertDeliveriesFailed.Load(k)
			fmt.Fprintf(&b, "auditor_alert_deliveries_failed_total{kind=%q} %d\n", k, v.(*atomic.Uint64).Load())
		}

		b.WriteString("# HELP auditor_cost_calculations_total Cost ranking calculations with or without a price table.\n")
		b.WriteString("# TYPE auditor_cost_calculations_total counter\n")
		fmt.Fprintf(&b, "auditor_cost_calculations_total{priced=%q} %d\n", "true", m.costCalculationsPriced.Load())
		fmt.Fprintf(&b, "auditor_cost_calculations_total{priced=%q} %d\n", "false", m.costCalculationsUnpriced.Load())

		b.WriteString("# HELP auditor_retention_simulations_total Timescale retention/compression simulations.\n")
		b.WriteString("# TYPE auditor_retention_simulations_total counter\n")
		fmt.Fprintf(&b, "auditor_retention_simulations_total %d\n", m.retentionSimulations.Load())
		b.WriteString("# HELP auditor_retention_simulations_low_ratio_total Simulations using the default compression ratio (low confidence).\n")
		b.WriteString("# TYPE auditor_retention_simulations_low_ratio_total counter\n")
		fmt.Fprintf(&b, "auditor_retention_simulations_low_ratio_total %d\n", m.retentionSimulationsLowRatio.Load())

		b.WriteString("# HELP auditor_access_review_campaigns_total Access-review campaigns by status.\n")
		b.WriteString("# TYPE auditor_access_review_campaigns_total counter\n")
		arkeys := make([]string, 0)
		m.accessReviewCampaigns.Range(func(k, _ any) bool {
			arkeys = append(arkeys, k.(string))
			return true
		})
		sort.Strings(arkeys)
		for _, k := range arkeys {
			v, _ := m.accessReviewCampaigns.Load(k)
			fmt.Fprintf(&b, "auditor_access_review_campaigns_total{status=%q} %d\n", k, v.(*atomic.Uint64).Load())
		}
		b.WriteString("# HELP auditor_access_review_pending_lines Pending access-review matrix lines.\n")
		b.WriteString("# TYPE auditor_access_review_pending_lines gauge\n")
		fmt.Fprintf(&b, "auditor_access_review_pending_lines %d\n", m.accessReviewPending.Load())

		b.WriteString("# HELP auditor_last_login_ingest_total Last-login aggregate ingest attempts by result.\n")
		b.WriteString("# TYPE auditor_last_login_ingest_total counter\n")
		llKeys := make([]string, 0)
		m.lastLoginIngest.Range(func(k, _ any) bool {
			llKeys = append(llKeys, k.(string))
			return true
		})
		sort.Strings(llKeys)
		for _, k := range llKeys {
			v, _ := m.lastLoginIngest.Load(k)
			fmt.Fprintf(&b, "auditor_last_login_ingest_total{result=%q} %d\n", k, v.(*atomic.Uint64).Load())
		}

		b.WriteString("# HELP auditor_pii_scans_total PII map scans by status and whether content stage ran.\n")
		b.WriteString("# TYPE auditor_pii_scans_total counter\n")
		pkeys := make([]string, 0)
		m.piiScans.Range(func(k, _ any) bool { pkeys = append(pkeys, k.(string)); return true })
		sort.Strings(pkeys)
		for _, k := range pkeys {
			v, _ := m.piiScans.Load(k)
			parts := strings.SplitN(k, "|", 2)
			if len(parts) != 2 {
				continue
			}
			fmt.Fprintf(&b, "auditor_pii_scans_total{status=%q,content=%q} %d\n", parts[0], parts[1], v.(*atomic.Uint64).Load())
		}
		b.WriteString("# HELP auditor_pii_scan_ceilings_total PII scans stopped by duration or table ceiling.\n")
		b.WriteString("# TYPE auditor_pii_scan_ceilings_total counter\n")
		fmt.Fprintf(&b, "auditor_pii_scan_ceilings_total %d\n", m.piiScanCeilings.Load())
		b.WriteString("# HELP auditor_pii_export_rows_total Rows streamed in PII CSV exports.\n")
		b.WriteString("# TYPE auditor_pii_export_rows_total counter\n")
		fmt.Fprintf(&b, "auditor_pii_export_rows_total %d\n", m.piiExportRows.Load())
		b.WriteString("# HELP auditor_pii_columns_by_method_total Classified PII columns by detection method.\n")
		b.WriteString("# TYPE auditor_pii_columns_by_method_total counter\n")
		piiMethodKeys := make([]string, 0)
		m.piiColumnsByMethod.Range(func(k, _ any) bool { piiMethodKeys = append(piiMethodKeys, k.(string)); return true })
		sort.Strings(piiMethodKeys)
		for _, k := range piiMethodKeys {
			v, _ := m.piiColumnsByMethod.Load(k)
			fmt.Fprintf(&b, "auditor_pii_columns_by_method_total{method=%q} %d\n", k, v.(*atomic.Uint64).Load())
		}

		b.WriteString("# HELP auditor_target_restore_drill_age_seconds Seconds since the last successful target_restore_drill per environment.\n")
		b.WriteString("# TYPE auditor_target_restore_drill_age_seconds gauge\n")
		ageKeys := make([]string, 0)
		m.targetRestoreDrillAge.Range(func(k, _ any) bool { ageKeys = append(ageKeys, k.(string)); return true })
		sort.Strings(ageKeys)
		for _, env := range ageKeys {
			v, _ := m.targetRestoreDrillAge.Load(env)
			fmt.Fprintf(&b, "auditor_target_restore_drill_age_seconds{environment=%q} %d\n", env, v.(*atomic.Uint64).Load())
		}
		b.WriteString("# HELP auditor_target_restore_rto_seconds RTO seconds of the last successful target_restore_drill per environment.\n")
		b.WriteString("# TYPE auditor_target_restore_rto_seconds gauge\n")
		rtoKeys := make([]string, 0)
		m.targetRestoreRTO.Range(func(k, _ any) bool { rtoKeys = append(rtoKeys, k.(string)); return true })
		sort.Strings(rtoKeys)
		for _, env := range rtoKeys {
			v, _ := m.targetRestoreRTO.Load(env)
			fmt.Fprintf(&b, "auditor_target_restore_rto_seconds{environment=%q} %d\n", env, v.(*atomic.Uint64).Load())
		}

		b.WriteString("# HELP auditor_quality_contract_checks_total Quality contract checks by result.\n")
		b.WriteString("# TYPE auditor_quality_contract_checks_total counter\n")
		qcKeys := make([]string, 0)
		m.qualityContractChecks.Range(func(k, _ any) bool { qcKeys = append(qcKeys, k.(string)); return true })
		sort.Strings(qcKeys)
		for _, k := range qcKeys {
			v, _ := m.qualityContractChecks.Load(k)
			fmt.Fprintf(&b, "auditor_quality_contract_checks_total{result=%q} %d\n", k, v.(*atomic.Uint64).Load())
		}
		b.WriteString("# HELP auditor_quality_contract_check_duration_seconds_sum Sum of quality contract check durations.\n")
		b.WriteString("# TYPE auditor_quality_contract_check_duration_seconds_sum counter\n")
		b.WriteString("# HELP auditor_quality_contract_check_duration_seconds_count Count of quality contract check durations.\n")
		b.WriteString("# TYPE auditor_quality_contract_check_duration_seconds_count counter\n")
		qcDurKeys := make([]string, 0)
		m.qualityContractDurationN.Range(func(k, _ any) bool { qcDurKeys = append(qcDurKeys, k.(string)); return true })
		sort.Strings(qcDurKeys)
		for _, k := range qcDurKeys {
			ns, _ := m.qualityContractDurationNs.Load(k)
			n, _ := m.qualityContractDurationN.Load(k)
			fmt.Fprintf(&b, "auditor_quality_contract_check_duration_seconds_sum{result=%q} %f\n", k, float64(ns.(*atomic.Uint64).Load())/1e9)
			fmt.Fprintf(&b, "auditor_quality_contract_check_duration_seconds_count{result=%q} %d\n", k, n.(*atomic.Uint64).Load())
		}

		_, _ = w.Write([]byte(b.String()))
	}
}

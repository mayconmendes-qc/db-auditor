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
	m.addMap(&m.httpDurationNs, durKey, uint64(d.Nanoseconds()))
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

		_, _ = w.Write([]byte(b.String()))
	}
}

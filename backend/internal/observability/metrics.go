package observability

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// Metrics holds process-level counters for Prometheus text exposition.
type Metrics struct {
	startedAt       time.Time
	httpRequests    atomic.Int64
	httpErrors      atomic.Int64
	auditRunsTotal  atomic.Int64
	auditRunsFailed atomic.Int64
}

// NewMetrics creates a metrics registry.
func NewMetrics() *Metrics {
	return &Metrics{startedAt: time.Now().UTC()}
}

// IncHTTP records an HTTP request; status >= 500 counts as error.
func (m *Metrics) IncHTTP(status int) {
	m.httpRequests.Add(1)
	if status >= 500 {
		m.httpErrors.Add(1)
	}
}

// IncAuditRun records a completed audit run outcome.
func (m *Metrics) IncAuditRun(failed bool) {
	m.auditRunsTotal.Add(1)
	if failed {
		m.auditRunsFailed.Add(1)
	}
}

// Handler serves Prometheus text exposition at GET /metrics.
func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		up := time.Since(m.startedAt).Seconds()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = fmt.Fprintf(w, "# HELP timescale_auditor_up Process up flag.\n")
		_, _ = fmt.Fprintf(w, "# TYPE timescale_auditor_up gauge\n")
		_, _ = fmt.Fprintf(w, "timescale_auditor_up 1\n")
		_, _ = fmt.Fprintf(w, "# HELP timescale_auditor_uptime_seconds Seconds since process start.\n")
		_, _ = fmt.Fprintf(w, "# TYPE timescale_auditor_uptime_seconds gauge\n")
		_, _ = fmt.Fprintf(w, "timescale_auditor_uptime_seconds %.0f\n", up)
		_, _ = fmt.Fprintf(w, "# HELP timescale_auditor_http_requests_total Total HTTP requests.\n")
		_, _ = fmt.Fprintf(w, "# TYPE timescale_auditor_http_requests_total counter\n")
		_, _ = fmt.Fprintf(w, "timescale_auditor_http_requests_total %d\n", m.httpRequests.Load())
		_, _ = fmt.Fprintf(w, "# HELP timescale_auditor_http_errors_total HTTP responses with status >= 500.\n")
		_, _ = fmt.Fprintf(w, "# TYPE timescale_auditor_http_errors_total counter\n")
		_, _ = fmt.Fprintf(w, "timescale_auditor_http_errors_total %d\n", m.httpErrors.Load())
		_, _ = fmt.Fprintf(w, "# HELP timescale_auditor_audit_runs_total Audit runs completed.\n")
		_, _ = fmt.Fprintf(w, "# TYPE timescale_auditor_audit_runs_total counter\n")
		_, _ = fmt.Fprintf(w, "timescale_auditor_audit_runs_total %d\n", m.auditRunsTotal.Load())
		_, _ = fmt.Fprintf(w, "# HELP timescale_auditor_audit_runs_failed_total Audit runs that failed.\n")
		_, _ = fmt.Fprintf(w, "# TYPE timescale_auditor_audit_runs_failed_total counter\n")
		_, _ = fmt.Fprintf(w, "timescale_auditor_audit_runs_failed_total %d\n", m.auditRunsFailed.Load())
	}
}

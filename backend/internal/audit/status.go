// Package audit orchestrates audit runs over registered collectors.
package audit

// Run-level and collector-level status constants aligned with DB constraints.
const (
	RunStatusRunning        = "running"
	RunStatusSuccess        = "success"
	RunStatusPartialSuccess = "partial_success"
	RunStatusFailed         = "failed"
	RunStatusCancelled      = "cancelled"

	CollectorStatusRunning = "running"
	CollectorStatusSuccess = "success"
	CollectorStatusSkipped = "skipped"
	CollectorStatusFailed  = "failed"
)

// AggregateRunStatus derives the audit_run status from collector outcomes.
func AggregateRunStatus(success, failed, skipped int, cancelled bool) string {
	if cancelled {
		return RunStatusCancelled
	}
	if failed == 0 && success > 0 {
		return RunStatusSuccess
	}
	if failed > 0 && success > 0 {
		return RunStatusPartialSuccess
	}
	if failed > 0 && success == 0 {
		return RunStatusFailed
	}
	// only skipped or empty
	if skipped > 0 && success == 0 && failed == 0 {
		return RunStatusSuccess
	}
	return RunStatusFailed
}

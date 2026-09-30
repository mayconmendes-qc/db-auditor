package api

import "strings"

// isSuccessfulRunStatus matches persisted audit_run.status values (lowercase).
func isSuccessfulRunStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "partial_success":
		return true
	default:
		return false
	}
}

func isFailedRunStatus(status string) bool {
	return strings.ToLower(strings.TrimSpace(status)) == "failed"
}

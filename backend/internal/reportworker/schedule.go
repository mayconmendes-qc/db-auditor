package reportworker

// WeeklyExecutiveDue is true when a daily or weekly run succeeded analysis
// and no executive report exists yet for that run. partial_success never qualifies.
func WeeklyExecutiveDue(profile, runStatus, analysisStatus string, hasReport bool) bool {
	if hasReport || analysisStatus != "success" || runStatus != "success" {
		return false
	}
	return profile == "daily" || profile == "weekly"
}

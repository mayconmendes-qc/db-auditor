package repository

import (
	"context"
	"errors"
)

func executiveReportDue(profile, runStatus, analysisStatus string, hasReport bool) bool {
	if hasReport || analysisStatus != "success" || runStatus != "success" {
		return false
	}
	return profile == "daily" || profile == "weekly"
}

// EnqueueDueExecutiveReports queues one executive PDF per successful daily or
// weekly run that does not already have one. partial_success is ignored.
func (s *Store) EnqueueDueExecutiveReports(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT r.environment_id::text, r.id::text, r.profile, r.status, a.status,
EXISTS(SELECT 1 FROM report_job j WHERE j.environment_id=r.environment_id AND j.audit_run_id=r.id AND j.report_type='executive')
FROM audit_run r
JOIN analysis_run a ON a.audit_run_id=r.id
WHERE r.profile IN ('daily','weekly') AND r.started_at > now() - interval '8 days'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type due struct {
		env, run, profile, runStatus, analysis string
		has                                    bool
	}
	pending := []due{}
	for rows.Next() {
		var item due
		if err = rows.Scan(&item.env, &item.run, &item.profile, &item.runStatus, &item.analysis, &item.has); err != nil {
			return err
		}
		pending = append(pending, item)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, item := range pending {
		if !executiveReportDue(item.profile, item.runStatus, item.analysis, item.has) {
			continue
		}
		_, err = s.CreateReportJob(ctx, ReportRequest{EnvironmentID: item.env, AuditRunID: item.run, Type: "executive", RequestedBy: "scheduler"})
		if err != nil && !errors.Is(err, ErrReportConflict) && !errors.Is(err, ErrReportIneligible) {
			return err
		}
	}
	return nil
}

package repository

import "context"

// StorageFootprint reports physical control-store sizes without scanning
// historical rows. PostgreSQL relation sizes include indexes and TOAST data.
type StorageFootprint struct {
	SnapshotsBytes      int64 `json:"snapshots_bytes"`
	ReportJobsBytes     int64 `json:"report_jobs_bytes"`
	PDFArtifactsBytes   int64 `json:"pdf_artifacts_bytes"`
	QualityHistoryBytes int64 `json:"quality_history_bytes"`
	ActionHistoryBytes  int64 `json:"action_history_bytes"`
}

func (s *Store) GetStorageFootprint(ctx context.Context) (*StorageFootprint, error) {
	var out StorageFootprint
	err := s.pool.QueryRow(ctx, `SELECT
    pg_total_relation_size('audit_run') + pg_total_relation_size('table_snapshot') + pg_total_relation_size('finding') + pg_total_relation_size('finding_event'),
    pg_total_relation_size('report_job'),
    pg_total_relation_size('report_artifact'),
    pg_total_relation_size('quality_scan') + pg_total_relation_size('quality_issue') + pg_total_relation_size('quality_issue_event'),
    pg_total_relation_size('finding_action') + pg_total_relation_size('finding_action_event') + pg_total_relation_size('finding_action_measurement')`).Scan(
		&out.SnapshotsBytes, &out.ReportJobsBytes, &out.PDFArtifactsBytes, &out.QualityHistoryBytes, &out.ActionHistoryBytes)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

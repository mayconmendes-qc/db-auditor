package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type FindingEvent struct {
	ID         string    `json:"id"`
	FindingID  string    `json:"finding_id"`
	AuditRunID *string   `json:"audit_run_id,omitempty"`
	EventType  string    `json:"event_type"`
	Reason     string    `json:"reason"`
	RecordedAt time.Time `json:"recorded_at"`
}

func (s *Store) ListFindingEvents(ctx context.Context, id string) ([]FindingEvent, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,finding_id::text,audit_run_id::text,event_type,reason,recorded_at FROM finding_event WHERE finding_id=$1::uuid ORDER BY recorded_at,id LIMIT 1000`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FindingEvent{}
	for rows.Next() {
		var e FindingEvent
		if err = rows.Scan(&e.ID, &e.FindingID, &e.AuditRunID, &e.EventType, &e.Reason, &e.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) SuppressFinding(ctx context.Context, id, reason string, until time.Time) (*Finding, error) {
	if reason == "" || !until.After(time.Now()) {
		return nil, errors.New("suppression needs reason and future expiration")
	}
	row := s.pool.QueryRow(ctx, `UPDATE finding SET status='suppressed',suppression_reason=$2,suppressed_until=$3,resolved_at=now(),updated_at=now() WHERE id=$1::uuid
RETURNING id::text, environment_id::text, audit_run_id::text,
finding_type,severity,status,title,summary,object_type,object_key,database_name,schema_name,object_name,evidence,dedup_key,rule_id,rule_version,category,confidence,impact,risk,recommendation,validation,reference_urls,rule_parameters,first_seen_at,last_seen_at,resolved_at,notes,created_at,updated_at,recurrence_count,suppression_reason,suppressed_until,superseded_by::text,assignee,due_at`, id, reason, until)
	f, err := scanFinding(row)
	if err != nil {
		return nil, err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO finding_event(finding_id,event_type,reason) VALUES($1::uuid,'suppressed',$2)`, id, reason)
	return f, err
}

func (s *Store) FindingExists(ctx context.Context, id string) (bool, error) {
	var found bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM finding WHERE id=$1::uuid)`, id).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return found, err
}

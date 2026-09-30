package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mayconmendes-qc/db-auditor/internal/collectors/postgres"
)

type ConstraintSnapshotRow struct {
	ID                   string    `json:"id"`
	DatabaseName         string    `json:"database_name"`
	SchemaName           string    `json:"schema_name"`
	TableName            string    `json:"table_name"`
	ConstraintName       string    `json:"constraint_name"`
	ConstraintType       string    `json:"constraint_type"`
	ConstraintDefinition string    `json:"constraint_definition"`
	IsValidated          bool      `json:"is_validated"`
	IsDeferrable         bool      `json:"is_deferrable"`
	IsDeferred           bool      `json:"is_deferred"`
	ConstrainedColumns   []string  `json:"constrained_columns,omitempty"`
	ReferencedSchema     *string   `json:"referenced_schema_name,omitempty"`
	ReferencedTable      *string   `json:"referenced_table_name,omitempty"`
	ReferencedColumns    []string  `json:"referenced_columns,omitempty"`
	FKUpdateAction       *string   `json:"fk_update_action,omitempty"`
	FKDeleteAction       *string   `json:"fk_delete_action,omitempty"`
	FKMatchType          *string   `json:"fk_match_type,omitempty"`
	CollectedAt          time.Time `json:"collected_at"`
}

func (s *Store) SaveStructuralInventory(
	ctx context.Context,
	environmentID, auditRunID pgtype.UUID,
	sequences []postgres.SequenceFacts,
	triggers []postgres.TriggerFacts,
	policies []postgres.PolicyFacts,
) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin structural inventory: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, seq := range sequences {
		_, err := tx.Exec(ctx, `
INSERT INTO sequence_snapshot (
  audit_run_id, environment_id, database_name, schema_name, sequence_name,
  data_type, start_value, increment_by, max_value, min_value, cycle,
  owned_by_table, owned_by_column, collected_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13, now())
ON CONFLICT (audit_run_id, database_name, schema_name, sequence_name) DO NOTHING
`, auditRunID, environmentID, seq.DatabaseName, seq.SchemaName, seq.SequenceName,
			nullStringPtr(seq.DataType), nullStringPtr(seq.StartValue), nullStringPtr(seq.IncrementBy),
			nullStringPtr(seq.MaxValue), nullStringPtr(seq.MinValue), seq.Cycle,
			nullStringPtr(seq.OwnedByTable), nullStringPtr(seq.OwnedByColumn))
		if err != nil {
			return fmt.Errorf("insert sequence_snapshot: %w", err)
		}
	}
	for _, tr := range triggers {
		_, err := tx.Exec(ctx, `
INSERT INTO trigger_snapshot (
  audit_run_id, environment_id, database_name, schema_name, table_name, trigger_name,
  enabled, timing, event_manipulation, action_statement, collected_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10, now())
ON CONFLICT (audit_run_id, database_name, schema_name, table_name, trigger_name) DO NOTHING
`, auditRunID, environmentID, tr.DatabaseName, tr.SchemaName, tr.TableName, tr.TriggerName,
			tr.Enabled, nullStringPtr(tr.Timing), nullStringPtr(tr.EventManipulation), nullStringPtr(tr.ActionStatement))
		if err != nil {
			return fmt.Errorf("insert trigger_snapshot: %w", err)
		}
	}
	for _, p := range policies {
		_, err := tx.Exec(ctx, `
INSERT INTO policy_snapshot (
  audit_run_id, environment_id, database_name, schema_name, table_name, policy_name,
  permissive, roles, cmd, qual, with_check, collected_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11, now())
ON CONFLICT (audit_run_id, database_name, schema_name, table_name, policy_name) DO NOTHING
`, auditRunID, environmentID, p.DatabaseName, p.SchemaName, p.TableName, p.PolicyName,
			nullStringPtr(p.Permissive), p.Roles, nullStringPtr(p.Cmd), nullStringPtr(p.Qual), nullStringPtr(p.WithCheck))
		if err != nil {
			return fmt.Errorf("insert policy_snapshot: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) ListConstraintSnapshots(ctx context.Context, f InventoryFilter) ([]ConstraintSnapshotRow, int, error) {
	where, args, next := inventoryWhereFor(f, 1)
	if f.Q != "" {
		where += fmt.Sprintf(" AND (constraint_name ILIKE $%d OR table_name ILIKE $%d)", next, next)
		args = append(args, "%"+f.Q+"%")
		next++
	}
	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM constraint_snapshot WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit, offset := f.Limit, f.Offset
	if limit <= 0 {
		limit = 50
	}
	query := fmt.Sprintf(`
SELECT id::text, database_name, schema_name, table_name, constraint_name,
  COALESCE(constraint_type,''), COALESCE(constraint_definition,''),
  COALESCE(is_validated,false), COALESCE(is_deferrable,false), COALESCE(is_deferred,false),
  COALESCE(constrained_columns, ARRAY[]::text[]),
  referenced_schema_name, referenced_table_name,
  COALESCE(referenced_columns, ARRAY[]::text[]),
  fk_update_action, fk_delete_action, fk_match_type, collected_at
FROM constraint_snapshot WHERE %s
ORDER BY database_name, schema_name, table_name, constraint_name
LIMIT $%d OFFSET $%d`, where, next, next+1)
	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]ConstraintSnapshotRow, 0)
	for rows.Next() {
		var r ConstraintSnapshotRow
		if err := rows.Scan(
			&r.ID, &r.DatabaseName, &r.SchemaName, &r.TableName, &r.ConstraintName,
			&r.ConstraintType, &r.ConstraintDefinition,
			&r.IsValidated, &r.IsDeferrable, &r.IsDeferred,
			&r.ConstrainedColumns, &r.ReferencedSchema, &r.ReferencedTable,
			&r.ReferencedColumns, &r.FKUpdateAction, &r.FKDeleteAction, &r.FKMatchType, &r.CollectedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// ObjectMapping is a persisted correspondence between objects across environments.
type ObjectMapping struct {
	ID                   string    `json:"id"`
	SourceEnvironmentID  string    `json:"source_environment_id"`
	TargetEnvironmentID  string    `json:"target_environment_id"`
	SourceDatabase       string    `json:"source_database"`
	SourceSchema         string    `json:"source_schema"`
	SourceObjectType     string    `json:"source_object_type"`
	SourceObjectName     string    `json:"source_object_name"`
	TargetDatabase       string    `json:"target_database"`
	TargetSchema         string    `json:"target_schema"`
	TargetObjectType     string    `json:"target_object_type"`
	TargetObjectName     string    `json:"target_object_name"`
	RelationType         string    `json:"relation_type"`
	Confidence           float64   `json:"confidence"`
	Status               string    `json:"status"`
	SourceFingerprint    *string   `json:"source_fingerprint,omitempty"`
	TargetFingerprint    *string   `json:"target_fingerprint,omitempty"`
	FingerprintAlgorithm *string   `json:"fingerprint_algorithm,omitempty"`
	Notes                *string   `json:"notes,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// CreateObjectMappingParams is the input for inserting a mapping.
type CreateObjectMappingParams struct {
	SourceEnvironmentID  string
	TargetEnvironmentID  string
	SourceDatabase       string
	SourceSchema         string
	SourceObjectType     string
	SourceObjectName     string
	TargetDatabase       string
	TargetSchema         string
	TargetObjectType     string
	TargetObjectName     string
	RelationType         string
	Confidence           float64
	Status               string
	SourceFingerprint    string
	TargetFingerprint    string
	FingerprintAlgorithm string
	Notes                string
}

func (s *Store) CreateObjectMapping(ctx context.Context, p CreateObjectMappingParams) (*ObjectMapping, error) {
	if p.Status == "" {
		p.Status = "manual"
	}
	if p.RelationType == "" {
		p.RelationType = "manual"
		// fallback to identical-style manual if invalid; DB check will reject bad values
		p.RelationType = "identical"
	}
	row := s.pool.QueryRow(ctx, `
INSERT INTO object_mapping (
  source_environment_id, target_environment_id,
  source_database, source_schema, source_object_type, source_object_name,
  target_database, target_schema, target_object_type, target_object_name,
  relation_type, confidence, status,
  source_fingerprint, target_fingerprint, fingerprint_algorithm, notes
) VALUES (
  $1::uuid, $2::uuid,
  $3, $4, $5, $6,
  $7, $8, $9, $10,
  $11, $12, $13,
  NULLIF($14,''), NULLIF($15,''), NULLIF($16,''), NULLIF($17,'')
)
RETURNING id::text, source_environment_id::text, target_environment_id::text,
  source_database, source_schema, source_object_type, source_object_name,
  target_database, target_schema, target_object_type, target_object_name,
  relation_type, confidence::float8, status,
  source_fingerprint, target_fingerprint, fingerprint_algorithm, notes,
  created_at, updated_at
`, p.SourceEnvironmentID, p.TargetEnvironmentID,
		p.SourceDatabase, p.SourceSchema, p.SourceObjectType, p.SourceObjectName,
		p.TargetDatabase, p.TargetSchema, p.TargetObjectType, p.TargetObjectName,
		p.RelationType, p.Confidence, p.Status,
		p.SourceFingerprint, p.TargetFingerprint, p.FingerprintAlgorithm, p.Notes,
	)
	return scanObjectMapping(row)
}

func (s *Store) ListObjectMappings(ctx context.Context, sourceEnv, targetEnv, status string) ([]ObjectMapping, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id::text, source_environment_id::text, target_environment_id::text,
  source_database, source_schema, source_object_type, source_object_name,
  target_database, target_schema, target_object_type, target_object_name,
  relation_type, confidence::float8, status,
  source_fingerprint, target_fingerprint, fingerprint_algorithm, notes,
  created_at, updated_at
FROM object_mapping
WHERE ($1 = '' OR source_environment_id = $1::uuid)
  AND ($2 = '' OR target_environment_id = $2::uuid)
  AND ($3 = '' OR status = $3)
ORDER BY updated_at DESC
`, sourceEnv, targetEnv, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ObjectMapping, 0)
	for rows.Next() {
		m, err := scanObjectMapping(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func (s *Store) UpdateObjectMappingStatus(ctx context.Context, id, status string, notes string) (*ObjectMapping, error) {
	row := s.pool.QueryRow(ctx, `
UPDATE object_mapping
SET status = $2, notes = COALESCE(NULLIF($3,''), notes), updated_at = now()
WHERE id = $1::uuid
RETURNING id::text, source_environment_id::text, target_environment_id::text,
  source_database, source_schema, source_object_type, source_object_name,
  target_database, target_schema, target_object_type, target_object_name,
  relation_type, confidence::float8, status,
  source_fingerprint, target_fingerprint, fingerprint_algorithm, notes,
  created_at, updated_at
`, id, status, notes)
	m, err := scanObjectMapping(row)
	if err != nil {
		return nil, fmt.Errorf("update mapping: %w", err)
	}
	return m, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanObjectMapping(row scannable) (*ObjectMapping, error) {
	var m ObjectMapping
	var srcFP, tgtFP, algo, notes pgtype.Text
	if err := row.Scan(
		&m.ID, &m.SourceEnvironmentID, &m.TargetEnvironmentID,
		&m.SourceDatabase, &m.SourceSchema, &m.SourceObjectType, &m.SourceObjectName,
		&m.TargetDatabase, &m.TargetSchema, &m.TargetObjectType, &m.TargetObjectName,
		&m.RelationType, &m.Confidence, &m.Status,
		&srcFP, &tgtFP, &algo, &notes,
		&m.CreatedAt, &m.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if srcFP.Valid {
		s := srcFP.String
		m.SourceFingerprint = &s
	}
	if tgtFP.Valid {
		s := tgtFP.String
		m.TargetFingerprint = &s
	}
	if algo.Valid {
		s := algo.String
		m.FingerprintAlgorithm = &s
	}
	if notes.Valid {
		s := notes.String
		m.Notes = &s
	}
	return &m, nil
}

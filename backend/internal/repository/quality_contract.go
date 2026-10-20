package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/osmendes/db-auditor/internal/config"
	"github.com/osmendes/db-auditor/internal/dqcontract"
)

var (
	ErrQualityContractNotFound         = errors.New("quality contract not found")
	ErrQualityContractVersionImmutable = errors.New("quality contract version is immutable")
	ErrQualityContractBadState         = errors.New("quality contract invalid state transition")
	ErrQualityContractNoSnapshot       = errors.New("complete snapshot required for proposal")
	ErrQualityContractDisabled         = errors.New("data quality feature disabled")
	ErrQualityContractConflict         = errors.New("quality contract conflict")
)

type QualityContract struct {
	ID            string                     `json:"id"`
	EnvironmentID string                     `json:"environment_id"`
	Database      string                     `json:"database"`
	Schema        string                     `json:"schema"`
	Table         string                     `json:"table"`
	CreatedBy     string                     `json:"created_by"`
	CreatedAt     time.Time                  `json:"created_at"`
	LatestVersion *QualityContractVersion    `json:"latest_version,omitempty"`
	LastRun       *QualityContractRunSummary `json:"last_run,omitempty"`
}

type QualityContractVersion struct {
	ID              string              `json:"id"`
	ContractID      string              `json:"contract_id"`
	VersionNumber   int                 `json:"version_number"`
	Document        dqcontract.Document `json:"document"`
	SourceYAML      string              `json:"source_yaml,omitempty"`
	SampleLimit     int                 `json:"sample_limit"`
	ScheduleProfile string              `json:"schedule_profile"`
	Status          string              `json:"status"`
	AuthoredBy      string              `json:"authored_by"`
	AuthoredAt      time.Time           `json:"authored_at"`
	ApprovedBy      *string             `json:"approved_by,omitempty"`
	ApprovedAt      *time.Time          `json:"approved_at,omitempty"`
}

type QualityContractRunSummary struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Coverage  string    `json:"coverage"`
	Mode      string    `json:"mode"`
	StartedAt time.Time `json:"started_at"`
}

type QualityContractRun struct {
	ID            string                   `json:"id"`
	EnvironmentID string                   `json:"environment_id"`
	ContractID    *string                  `json:"contract_id,omitempty"`
	VersionID     *string                  `json:"version_id,omitempty"`
	Mode          string                   `json:"mode"`
	Status        string                   `json:"status"`
	Coverage      string                   `json:"coverage"`
	SkipReason    string                   `json:"skip_reason,omitempty"`
	SampleLimit   int                      `json:"sample_limit"`
	Actor         string                   `json:"actor"`
	StartedAt     time.Time                `json:"started_at"`
	FinishedAt    *time.Time               `json:"finished_at,omitempty"`
	Results       []dqcontract.CheckResult `json:"results"`
	Report        dqcontract.RunReport     `json:"report"`
}

type QualityContractTrendPoint struct {
	CheckKey      string    `json:"check_key"`
	CheckType     string    `json:"check_type"`
	Result        string    `json:"result"`
	ObservedPct   *float64  `json:"observed_pct,omitempty"`
	ObservedCount *int      `json:"observed_count,omitempty"`
	SampleSize    *int      `json:"sample_size,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	RunID         string    `json:"run_id"`
	Gap           bool      `json:"gap,omitempty"`
}

func (s *Store) ListQualityContracts(ctx context.Context, env string) ([]QualityContract, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,environment_id::text,database_name,schema_name,table_name,created_by,created_at
FROM quality_contract WHERE environment_id=$1::uuid ORDER BY database_name,schema_name,table_name`, env)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []QualityContract
	for rows.Next() {
		var c QualityContract
		if err = rows.Scan(&c.ID, &c.EnvironmentID, &c.Database, &c.Schema, &c.Table, &c.CreatedBy, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		v, err := s.latestContractVersion(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].LatestVersion = v
		run, err := s.lastContractRunSummary(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].LastRun = run
	}
	if out == nil {
		out = []QualityContract{}
	}
	return out, nil
}

func (s *Store) latestContractVersion(ctx context.Context, contractID string) (*QualityContractVersion, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT id::text FROM quality_contract_version WHERE contract_id=$1::uuid ORDER BY version_number DESC LIMIT 1`, contractID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.GetQualityContractVersion(ctx, "", contractID, id)
}

func (s *Store) lastContractRunSummary(ctx context.Context, contractID string) (*QualityContractRunSummary, error) {
	var r QualityContractRunSummary
	err := s.pool.QueryRow(ctx, `SELECT id::text,status,coverage,mode,started_at FROM quality_contract_run
WHERE contract_id=$1::uuid ORDER BY started_at DESC LIMIT 1`, contractID).Scan(&r.ID, &r.Status, &r.Coverage, &r.Mode, &r.StartedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) GetQualityContractVersion(ctx context.Context, env, contractID, versionID string) (*QualityContractVersion, error) {
	q := `SELECT v.id::text,v.contract_id::text,v.version_number,v.document,v.source_yaml,v.sample_limit,v.schedule_profile,v.status,v.authored_by,v.authored_at,v.approved_by,v.approved_at
FROM quality_contract_version v JOIN quality_contract c ON c.id=v.contract_id
WHERE v.id=$1::uuid AND v.contract_id=$2::uuid`
	args := []any{versionID, contractID}
	if env != "" {
		q += ` AND c.environment_id=$3::uuid`
		args = append(args, env)
	}
	var v QualityContractVersion
	var doc []byte
	var approvedBy *string
	var approvedAt *time.Time
	err := s.pool.QueryRow(ctx, q, args...).Scan(&v.ID, &v.ContractID, &v.VersionNumber, &doc, &v.SourceYAML, &v.SampleLimit, &v.ScheduleProfile, &v.Status, &v.AuthoredBy, &v.AuthoredAt, &approvedBy, &approvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrQualityContractNotFound
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(doc, &v.Document); err != nil {
		return nil, err
	}
	v.ApprovedBy, v.ApprovedAt = approvedBy, approvedAt
	return &v, nil
}

func (s *Store) CreateQualityContract(ctx context.Context, env, actor string, doc dqcontract.Document, sourceYAML string) (*QualityContract, error) {
	dqcontract.NormalizeSampleLimit(&doc)
	if err := dqcontract.ValidateDocument(doc, dqcontract.ProcessMaxSample()); err != nil {
		return nil, err
	}
	if sourceYAML == "" {
		var err error
		sourceYAML, err = dqcontract.MarshalYAML(doc)
		if err != nil {
			return nil, err
		}
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var contractID string
	err = tx.QueryRow(ctx, `INSERT INTO quality_contract(environment_id,database_name,schema_name,table_name,created_by)
VALUES($1::uuid,$2,$3,$4,$5)
ON CONFLICT (environment_id,database_name,schema_name,table_name) DO NOTHING
RETURNING id::text`, env, doc.Database, doc.Schema, doc.Table, actor).Scan(&contractID)
	if errors.Is(err, pgx.ErrNoRows) || contractID == "" {
		err = tx.QueryRow(ctx, `SELECT id::text FROM quality_contract WHERE environment_id=$1::uuid AND database_name=$2 AND schema_name=$3 AND table_name=$4`,
			env, doc.Database, doc.Schema, doc.Table).Scan(&contractID)
		if err != nil {
			return nil, err
		}
		return nil, ErrQualityContractConflict
	}
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO quality_contract_version(contract_id,version_number,document,source_yaml,sample_limit,schedule_profile,status,authored_by)
VALUES($1::uuid,1,$2::jsonb,$3,$4,$5,'proposed',$6)`, contractID, raw, sourceYAML, doc.SampleLimit, doc.ScheduleProfile, actor); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	items, err := s.ListQualityContracts(ctx, env)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == contractID {
			return &items[i], nil
		}
	}
	return nil, ErrQualityContractNotFound
}

func (s *Store) AddQualityContractVersion(ctx context.Context, env, contractID, actor string, doc dqcontract.Document, sourceYAML string) (*QualityContractVersion, error) {
	dqcontract.NormalizeSampleLimit(&doc)
	if err := dqcontract.ValidateDocument(doc, dqcontract.ProcessMaxSample()); err != nil {
		return nil, err
	}
	if sourceYAML == "" {
		var err error
		sourceYAML, err = dqcontract.MarshalYAML(doc)
		if err != nil {
			return nil, err
		}
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists string
	var dbName, schema, table string
	err = tx.QueryRow(ctx, `SELECT id::text,database_name,schema_name,table_name FROM quality_contract WHERE id=$1::uuid AND environment_id=$2::uuid FOR UPDATE`, contractID, env).Scan(&exists, &dbName, &schema, &table)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrQualityContractNotFound
	}
	if err != nil {
		return nil, err
	}
	if doc.Database != dbName || doc.Schema != schema || doc.Table != table {
		return nil, fmt.Errorf("documento deve manter database/schema/table do contrato")
	}
	var next int
	if err = tx.QueryRow(ctx, `SELECT COALESCE(max(version_number),0)+1 FROM quality_contract_version WHERE contract_id=$1::uuid`, contractID).Scan(&next); err != nil {
		return nil, err
	}
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO quality_contract_version(contract_id,version_number,document,source_yaml,sample_limit,schedule_profile,status,authored_by)
VALUES($1::uuid,$2,$3::jsonb,$4,$5,$6,'proposed',$7) RETURNING id::text`, contractID, next, raw, sourceYAML, doc.SampleLimit, doc.ScheduleProfile, actor).Scan(&id)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetQualityContractVersion(ctx, env, contractID, id)
}

func (s *Store) ApproveQualityContractVersion(ctx context.Context, env, contractID, versionID, actor string) (*QualityContractVersion, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	err = tx.QueryRow(ctx, `SELECT v.status FROM quality_contract_version v
JOIN quality_contract c ON c.id=v.contract_id
WHERE v.id=$1::uuid AND v.contract_id=$2::uuid AND c.environment_id=$3::uuid FOR UPDATE OF v`, versionID, contractID, env).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrQualityContractNotFound
	}
	if err != nil {
		return nil, err
	}
	if status != dqcontract.StatusProposed {
		return nil, ErrQualityContractBadState
	}
	if _, err = tx.Exec(ctx, `UPDATE quality_contract_version SET status='superseded'
WHERE contract_id=$1::uuid AND status='approved'`, contractID); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE quality_contract_version SET status='approved',approved_by=$2,approved_at=now()
WHERE id=$1::uuid`, versionID, actor); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetQualityContractVersion(ctx, env, contractID, versionID)
}

func (s *Store) ProposeQualityContractFromSnapshot(ctx context.Context, env, actor, database, schema, table string) (*QualityContract, error) {
	runID, _, err := s.latestCompleteRunID(ctx, env)
	if err != nil || runID == "" {
		return nil, ErrQualityContractNoSnapshot
	}
	cols, err := s.pool.Query(ctx, `SELECT column_name,data_type,is_nullable FROM column_snapshot
WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND database_name=$3 AND schema_name=$4 AND table_name=$5
ORDER BY ordinal_position`, env, runID, database, schema, table)
	if err != nil {
		return nil, err
	}
	var columns []dqcontract.SnapshotColumn
	for cols.Next() {
		var c dqcontract.SnapshotColumn
		if err = cols.Scan(&c.Name, &c.DataType, &c.IsNullable); err != nil {
			cols.Close()
			return nil, err
		}
		columns = append(columns, c)
	}
	err = cols.Err()
	cols.Close()
	if err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, ErrQualityContractNoSnapshot
	}
	crows, err := s.pool.Query(ctx, `SELECT constraint_name,constraint_type,
COALESCE(constrained_columns,ARRAY[]::text[]),COALESCE(referenced_schema_name,''),COALESCE(referenced_table_name,''),COALESCE(referenced_columns,ARRAY[]::text[])
FROM constraint_snapshot
WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND database_name=$3 AND schema_name=$4 AND table_name=$5
AND constraint_type IN ('p','u','f')`, env, runID, database, schema, table)
	if err != nil {
		return nil, err
	}
	var constraints []dqcontract.SnapshotConstraint
	for crows.Next() {
		var c dqcontract.SnapshotConstraint
		if err = crows.Scan(&c.Name, &c.Type, &c.ConstrainedColumns, &c.ReferencedSchema, &c.ReferencedTable, &c.ReferencedColumns); err != nil {
			crows.Close()
			return nil, err
		}
		constraints = append(constraints, c)
	}
	err = crows.Err()
	crows.Close()
	if err != nil {
		return nil, err
	}
	doc := dqcontract.ProposeFromSnapshot(dqcontract.ProposalInput{
		Database: database, Schema: schema, Table: table,
		Columns: columns, Constraints: constraints, SampleLimit: dqcontract.DefaultSampleLimit,
	})
	if len(doc.Checks) == 0 {
		return nil, fmt.Errorf("nenhuma checagem sugerida a partir do schema")
	}
	// Reuse existing contract row if present: add proposed version instead.
	var existing string
	err = s.pool.QueryRow(ctx, `SELECT id::text FROM quality_contract WHERE environment_id=$1::uuid AND database_name=$2 AND schema_name=$3 AND table_name=$4`,
		env, database, schema, table).Scan(&existing)
	if err == nil && existing != "" {
		v, err := s.AddQualityContractVersion(ctx, env, existing, actor, doc, "")
		if err != nil {
			return nil, err
		}
		items, err := s.ListQualityContracts(ctx, env)
		if err != nil {
			return nil, err
		}
		for i := range items {
			if items[i].ID == existing {
				items[i].LatestVersion = v
				return &items[i], nil
			}
		}
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	return s.CreateQualityContract(ctx, env, actor, doc, "")
}

func (s *Store) ImportQualityContractYAML(ctx context.Context, env, actor string, raw []byte) (*QualityContract, error) {
	doc, yamlNorm, err := dqcontract.ParseYAML(raw, dqcontract.ProcessMaxSample())
	if err != nil {
		return nil, err
	}
	var existing string
	err = s.pool.QueryRow(ctx, `SELECT id::text FROM quality_contract WHERE environment_id=$1::uuid AND database_name=$2 AND schema_name=$3 AND table_name=$4`,
		env, doc.Database, doc.Schema, doc.Table).Scan(&existing)
	if err == nil && existing != "" {
		if _, err = s.AddQualityContractVersion(ctx, env, existing, actor, doc, yamlNorm); err != nil {
			return nil, err
		}
		items, listErr := s.ListQualityContracts(ctx, env)
		if listErr != nil {
			return nil, listErr
		}
		for i := range items {
			if items[i].ID == existing {
				return &items[i], nil
			}
		}
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	return s.CreateQualityContract(ctx, env, actor, doc, yamlNorm)
}

func (s *Store) ExportQualityContractYAML(ctx context.Context, env, contractID, versionID string) (string, error) {
	v, err := s.GetQualityContractVersion(ctx, env, contractID, versionID)
	if err != nil {
		return "", err
	}
	if v.SourceYAML != "" {
		return v.SourceYAML, nil
	}
	return dqcontract.MarshalYAML(v.Document)
}

func (s *Store) volumeContextFor(ctx context.Context, env, database, schema, table string) (dqcontract.VolumeContext, error) {
	var cur, prev int64
	var hasPrev bool
	runID, _, err := s.latestCompleteRunID(ctx, env)
	if err != nil || runID == "" {
		return dqcontract.VolumeContext{}, nil
	}
	_ = s.pool.QueryRow(ctx, `SELECT COALESCE(row_estimate,0) FROM table_snapshot
WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND database_name=$3 AND schema_name=$4 AND table_name=$5`,
		env, runID, database, schema, table).Scan(&cur)
	prevRun, err := s.PreviousCompleteRunID(ctx, env, runID)
	if err == nil && prevRun != "" {
		if err = s.pool.QueryRow(ctx, `SELECT COALESCE(row_estimate,0) FROM table_snapshot
WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND database_name=$3 AND schema_name=$4 AND table_name=$5`,
			env, prevRun, database, schema, table).Scan(&prev); err == nil {
			hasPrev = true
		}
	}
	return dqcontract.VolumeContext{CurrentEstimate: cur, PreviousEstimate: prev, HasPrevious: hasPrev}, nil
}

// ExecuteQualityContractRun runs an approved version or an ad hoc YAML document.
func (s *Store) ExecuteQualityContractRun(ctx context.Context, env, dsn, actor, mode string, contractID, versionID string, adhoc dqcontract.Document, tokenID, auditRunID string) (*QualityContractRun, error) {
	if !dqcontract.DataQualityEnabled() {
		return nil, ErrQualityContractDisabled
	}
	var doc dqcontract.Document
	var cid, vid *string
	sampleLimit := dqcontract.DefaultSampleLimit
	if contractID != "" {
		var v *QualityContractVersion
		var err error
		if versionID != "" {
			v, err = s.GetQualityContractVersion(ctx, env, contractID, versionID)
		} else {
			v, err = s.approvedContractVersion(ctx, env, contractID)
		}
		if err != nil {
			return nil, err
		}
		if mode != dqcontract.ModeCLI && v.Status != dqcontract.StatusApproved {
			return nil, ErrQualityContractBadState
		}
		doc = v.Document
		sampleLimit = v.SampleLimit
		c := contractID
		cid = &c
		id := v.ID
		vid = &id
	} else {
		doc = adhoc
		dqcontract.NormalizeSampleLimit(&doc)
		sampleLimit = doc.SampleLimit
	}
	if err := dqcontract.ValidateDocument(doc, dqcontract.ProcessMaxSample()); err != nil {
		return nil, err
	}
	vol, _ := s.volumeContextFor(ctx, env, doc.Database, doc.Schema, doc.Table)
	var runID string
	err := s.pool.QueryRow(ctx, `INSERT INTO quality_contract_run(environment_id,contract_id,version_id,mode,status,coverage,sample_limit,token_id,audit_run_id,actor,document)
VALUES($1::uuid,$2::uuid,$3::uuid,$4,'running','complete',$5,NULLIF($6,'')::uuid,NULLIF($7,'')::uuid,$8,$9::jsonb)
RETURNING id::text`, env, cid, vid, mode, sampleLimit, tokenID, auditRunID, actor, mustJSON(doc)).Scan(&runID)
	if err != nil {
		return nil, err
	}
	outcome, execErr := dqcontract.Execute(ctx, dqcontract.RunRequest{
		EnvironmentID: env, DSN: dsn, Document: doc, Mode: mode, Volume: vol,
	})
	status, coverage, skip := outcome.Status, outcome.Coverage, ""
	if execErr != nil {
		if strings.Contains(execErr.Error(), "quality disabled") {
			status, coverage, skip = "skipped", "none", "AUDITOR_DATA_QUALITY_ENABLED desligado"
		} else {
			status, coverage = "failed", "none"
			skip = execErr.Error()
		}
	}
	finished := time.Now().UTC()
	if _, err = s.pool.Exec(ctx, `UPDATE quality_contract_run SET status=$2,coverage=$3,skip_reason=$4,finished_at=$5 WHERE id=$1::uuid`,
		runID, status, coverage, skip, finished); err != nil {
		return nil, err
	}
	for _, r := range outcome.Results {
		if _, err = s.pool.Exec(ctx, `INSERT INTO quality_contract_check_result(
run_id,check_key,check_type,column_name,result,reason,severity,observed_count,observed_pct,sample_size,sample_method,freshness_age_hours,evidence)
VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'{}'::jsonb)`,
			runID, r.CheckKey, r.CheckType, r.ColumnName, r.Result, r.Reason, r.Severity,
			r.ObservedCount, r.ObservedPct, r.SampleSize, r.SampleMethod, r.FreshnessAgeHours); err != nil {
			return nil, err
		}
	}
	if status != "skipped" && status != "failed" {
		_ = s.upsertContractViolationFindings(ctx, env, contractID, versionID, doc, outcome.Results)
	}
	return s.GetQualityContractRun(ctx, env, runID)
}

func (s *Store) approvedContractVersion(ctx context.Context, env, contractID string) (*QualityContractVersion, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT v.id::text FROM quality_contract_version v
JOIN quality_contract c ON c.id=v.contract_id
WHERE v.contract_id=$1::uuid AND c.environment_id=$2::uuid AND v.status='approved'
ORDER BY v.version_number DESC LIMIT 1`, contractID, env).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrQualityContractNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.GetQualityContractVersion(ctx, env, contractID, id)
}

func (s *Store) GetQualityContractRun(ctx context.Context, env, runID string) (*QualityContractRun, error) {
	var r QualityContractRun
	var finished *time.Time
	var doc []byte
	err := s.pool.QueryRow(ctx, `SELECT id::text,environment_id::text,contract_id::text,version_id::text,mode,status,coverage,skip_reason,sample_limit,actor,started_at,finished_at,document
FROM quality_contract_run WHERE id=$1::uuid AND environment_id=$2::uuid`, runID, env).Scan(
		&r.ID, &r.EnvironmentID, &r.ContractID, &r.VersionID, &r.Mode, &r.Status, &r.Coverage, &r.SkipReason, &r.SampleLimit, &r.Actor, &r.StartedAt, &finished, &doc)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrQualityContractNotFound
	}
	if err != nil {
		return nil, err
	}
	r.FinishedAt = finished
	var d dqcontract.Document
	_ = json.Unmarshal(doc, &d)
	rows, err := s.pool.Query(ctx, `SELECT check_key,check_type,column_name,result,reason,severity,observed_count,observed_pct,sample_size,sample_method,freshness_age_hours
FROM quality_contract_check_result WHERE run_id=$1::uuid ORDER BY check_key`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cr dqcontract.CheckResult
		if err = rows.Scan(&cr.CheckKey, &cr.CheckType, &cr.ColumnName, &cr.Result, &cr.Reason, &cr.Severity,
			&cr.ObservedCount, &cr.ObservedPct, &cr.SampleSize, &cr.SampleMethod, &cr.FreshnessAgeHours); err != nil {
			return nil, err
		}
		if cr.SampleSize != nil {
			cr.SamplingNote = dqcontract.SamplingNote
		}
		r.Results = append(r.Results, cr)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if r.Results == nil {
		r.Results = []dqcontract.CheckResult{}
	}
	finishedAt := r.StartedAt
	if r.FinishedAt != nil {
		finishedAt = *r.FinishedAt
	}
	cid := ""
	if r.ContractID != nil {
		cid = *r.ContractID
	}
	vid := ""
	if r.VersionID != nil {
		vid = *r.VersionID
	}
	r.Report = dqcontract.RunReport{
		SchemaVersion: "1", EnvironmentID: env, ContractID: cid, VersionID: vid,
		Mode: r.Mode, Status: r.Status, Coverage: r.Coverage,
		Database: d.Database, Schema: d.Schema, Table: d.Table, SampleLimit: r.SampleLimit,
		StartedAt: r.StartedAt, FinishedAt: finishedAt, Results: r.Results, SamplingNote: dqcontract.SamplingNote,
	}
	return &r, nil
}

func (s *Store) ListQualityContractRuns(ctx context.Context, env, contractID string, limit int) ([]QualityContractRunSummary, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text,status,coverage,mode,started_at FROM quality_contract_run
WHERE environment_id=$1::uuid AND contract_id=$2::uuid ORDER BY started_at DESC LIMIT $3`, env, contractID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []QualityContractRunSummary
	for rows.Next() {
		var r QualityContractRunSummary
		if err = rows.Scan(&r.ID, &r.Status, &r.Coverage, &r.Mode, &r.StartedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if out == nil {
		out = []QualityContractRunSummary{}
	}
	return out, rows.Err()
}

func (s *Store) ListQualityContractTrend(ctx context.Context, env, contractID string, days int) ([]QualityContractTrendPoint, error) {
	if days <= 0 || days > 90 {
		days = 90
	}
	rows, err := s.pool.Query(ctx, `SELECT r.id::text,r.started_at,r.coverage,c.check_key,c.check_type,c.result,c.observed_pct,c.observed_count,c.sample_size
FROM quality_contract_run r
JOIN quality_contract_check_result c ON c.run_id=r.id
WHERE r.environment_id=$1::uuid AND r.contract_id=$2::uuid AND r.started_at >= now() - make_interval(days => $3)
ORDER BY c.check_key, r.started_at`, env, contractID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []QualityContractTrendPoint
	for rows.Next() {
		var p QualityContractTrendPoint
		var coverage string
		if err = rows.Scan(&p.RunID, &p.StartedAt, &coverage, &p.CheckKey, &p.CheckType, &p.Result, &p.ObservedPct, &p.ObservedCount, &p.SampleSize); err != nil {
			return nil, err
		}
		p.Gap = coverage == "partial" || p.Result == dqcontract.ResultNotExecuted
		out = append(out, p)
	}
	if out == nil {
		out = []QualityContractTrendPoint{}
	}
	return out, rows.Err()
}

func (s *Store) upsertContractViolationFindings(ctx context.Context, env, contractID, versionID string, doc dqcontract.Document, results []dqcontract.CheckResult) error {
	var findings []analyzerFinding
	for _, r := range results {
		if r.Result != dqcontract.ResultViolated {
			continue
		}
		dedup := fmt.Sprintf("quality.contract_violation|%s|%s|%s", contractID, versionID, r.CheckKey)
		title := fmt.Sprintf("Violação de contrato: %s em %s.%s.%s", r.CheckKey, doc.Database, doc.Schema, doc.Table)
		summary := r.Reason
		if summary == "" {
			summary = "Checagem do contrato violada."
		}
		ev, _ := json.Marshal(map[string]any{
			"check_key": r.CheckKey, "check_type": r.CheckType, "result": r.Result,
			"observed_count": r.ObservedCount, "observed_pct": r.ObservedPct,
			"sample_size": r.SampleSize, "freshness_age_hours": r.FreshnessAgeHours,
		})
		f, err := s.UpsertFinding(ctx, UpsertFindingParams{
			EnvironmentID: env, FindingType: dqcontract.FindingType, Severity: r.Severity,
			Title: title, Summary: summary, ObjectType: "table",
			ObjectKey:    doc.Database + "." + doc.Schema + "." + doc.Table,
			DatabaseName: doc.Database, SchemaName: doc.Schema, ObjectName: doc.Table,
			Evidence: ev, DedupKey: dedup, RuleID: dqcontract.FindingType, RuleVersion: dqcontract.RuleVersion,
			Category: dqcontract.Category, Confidence: 0.7,
			Impact:         "Contagens amostrais indicam desvio do contrato declarado.",
			Risk:           "Dados fora do contrato podem propagar erros silenciosos.",
			Recommendation: "Investigue a origem; o auditor não corrige dados.",
			Validation:     "Reexecute o contrato com amostra comparável após a correção externa.",
		})
		if err != nil {
			return err
		}
		findings = append(findings, analyzerFinding{EnvironmentID: env, DedupKey: dedup, FindingType: dqcontract.FindingType, Severity: r.Severity, Title: title})
		_ = f
	}
	// Persistent violation alert: ClaimAlert/NotifyFindings path (D13) — notify only high+.
	if len(findings) > 0 {
		s.notifyContractFindings(ctx, findings)
	}
	return nil
}

type analyzerFinding struct {
	EnvironmentID, DedupKey, FindingType, Severity, Title string
}

func (s *Store) notifyContractFindings(ctx context.Context, findings []analyzerFinding) {
	// Reuse ClaimAlert directly for high severity violations (AN-11 / D13).
	for _, f := range findings {
		if dqcontract.SeverityRank(f.Severity) < dqcontract.SeverityRank(dqcontract.SeverityHigh) {
			continue
		}
		claimed, err := s.ClaimAlert(ctx, f.EnvironmentID, f.DedupKey)
		if err != nil || !claimed {
			continue
		}
		// Delivery via existing notify stack is optional; ClaimAlert records dedup.
		_ = claimed
	}
}

// MaybeRunQualityContractsAfterRun executes approved contracts linked to the run's profile.
func (s *Store) MaybeRunQualityContractsAfterRun(ctx context.Context, env, runID string) error {
	if !dqcontract.DataQualityEnabled() {
		return s.skipScheduledContracts(ctx, env, runID, "AUDITOR_DATA_QUALITY_ENABLED desligado")
	}
	dsn := config.DSNForEnvironment(config.LoadTargetDSNs(), env)
	if dsn == "" {
		return s.skipScheduledContracts(ctx, env, runID, "conexão de leitura ausente")
	}
	var profile string
	if err := s.pool.QueryRow(ctx, `SELECT profile FROM audit_run WHERE id=$1::uuid AND environment_id=$2::uuid`, runID, env).Scan(&profile); err != nil {
		return nil
	}
	rows, err := s.pool.Query(ctx, `SELECT c.id::text,v.id::text FROM quality_contract c
JOIN quality_contract_version v ON v.contract_id=c.id AND v.status='approved'
WHERE c.environment_id=$1::uuid AND v.schedule_profile=$2`, env, profile)
	if err != nil {
		return err
	}
	defer rows.Close()
	type pair struct{ c, v string }
	var pairs []pair
	for rows.Next() {
		var p pair
		if err = rows.Scan(&p.c, &p.v); err != nil {
			return err
		}
		pairs = append(pairs, p)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, p := range pairs {
		if _, err := s.ExecuteQualityContractRun(ctx, env, dsn, "scheduler", dqcontract.ModeSchedule, p.c, p.v, dqcontract.Document{}, "", runID); err != nil {
			continue
		}
	}
	return nil
}

func (s *Store) skipScheduledContracts(ctx context.Context, env, runID, reason string) error {
	var profile string
	if err := s.pool.QueryRow(ctx, `SELECT profile FROM audit_run WHERE id=$1::uuid`, runID).Scan(&profile); err != nil {
		return nil
	}
	rows, err := s.pool.Query(ctx, `SELECT c.id::text,v.id::text,v.sample_limit,v.document FROM quality_contract c
JOIN quality_contract_version v ON v.contract_id=c.id AND v.status='approved'
WHERE c.environment_id=$1::uuid AND v.schedule_profile=$2`, env, profile)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, vid string
		var limit int
		var doc []byte
		if err = rows.Scan(&cid, &vid, &limit, &doc); err != nil {
			return err
		}
		_, _ = s.pool.Exec(ctx, `INSERT INTO quality_contract_run(environment_id,contract_id,version_id,mode,status,coverage,skip_reason,sample_limit,audit_run_id,actor,document,finished_at)
VALUES($1::uuid,$2::uuid,$3::uuid,'schedule','skipped','none',$4,$5,$6::uuid,'scheduler',$7::jsonb,now())`,
			env, cid, vid, reason, limit, runID, doc)
	}
	return rows.Err()
}

func (s *Store) ListQualityContractSummariesForReport(ctx context.Context, env string, limit int) ([]dqcontract.CheckResult, string, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `SELECT c.check_key,c.check_type,c.column_name,c.result,c.reason,c.severity,c.observed_count,c.observed_pct,c.sample_size,c.sample_method,c.freshness_age_hours
FROM quality_contract_check_result c
JOIN quality_contract_run r ON r.id=c.run_id
WHERE r.environment_id=$1::uuid AND r.finished_at IS NOT NULL
ORDER BY r.started_at DESC, c.check_key
LIMIT $2`, env, limit)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []dqcontract.CheckResult
	for rows.Next() {
		var cr dqcontract.CheckResult
		if err = rows.Scan(&cr.CheckKey, &cr.CheckType, &cr.ColumnName, &cr.Result, &cr.Reason, &cr.Severity,
			&cr.ObservedCount, &cr.ObservedPct, &cr.SampleSize, &cr.SampleMethod, &cr.FreshnessAgeHours); err != nil {
			return nil, "", err
		}
		out = append(out, cr)
	}
	return out, dqcontract.SamplingNote, rows.Err()
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}

package repository

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/osmendes/db-auditor/internal/analyzer"
	"github.com/osmendes/db-auditor/internal/config"
	"github.com/osmendes/db-auditor/internal/pii"
)

var (
	ErrPIINotFound        = errors.New("pii: not found")
	ErrPIIContentDisabled = errors.New("pii: content scan disabled")
	ErrPIINotApplicable   = errors.New("pii: not applicable")
	ErrPIIValidation      = errors.New("pii: validation")
	ErrPIINoReferenceRun  = errors.New("pii: no complete audit run")
)

type PIIDictionaryEntry struct {
	ID        string `json:"id,omitempty"`
	Pattern   string `json:"name_pattern"`
	Category  string `json:"category"`
	Active    bool   `json:"active"`
	UpdatedBy string `json:"updated_by,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type PIIEnvironmentSetting struct {
	EnvironmentID      string  `json:"environment_id"`
	ContentScanEnabled bool    `json:"content_scan_enabled"`
	EnabledBy          string  `json:"enabled_by,omitempty"`
	EnabledAt          *string `json:"enabled_at,omitempty"`
	ProcessFlagEnabled bool    `json:"process_flag_enabled"`
	EffectiveEnabled   bool    `json:"effective_enabled"`
}

type PIIScan struct {
	ID              string     `json:"id"`
	EnvironmentID   string     `json:"environment_id"`
	AuditRunID      string     `json:"audit_run_id,omitempty"`
	Stages          []int      `json:"stages"`
	Status          string     `json:"status"`
	CoverageNote    string     `json:"coverage_note,omitempty"`
	MissingObjects  []string   `json:"missing_objects,omitempty"`
	ColumnsAnalyzed int        `json:"columns_analyzed"`
	TablesAnalyzed  int        `json:"tables_analyzed"`
	ContentColumns  int        `json:"content_columns"`
	Actor           string     `json:"actor"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	Error           string     `json:"error,omitempty"`
}

type PIIColumnResult struct {
	ID               string   `json:"id"`
	ScanID           string   `json:"scan_id"`
	EnvironmentID    string   `json:"environment_id"`
	DatabaseName     string   `json:"database_name"`
	SchemaName       string   `json:"schema_name"`
	TableName        string   `json:"table_name"`
	ColumnName       string   `json:"column_name"`
	Qualified        string   `json:"qualified"`
	DataType         string   `json:"data_type"`
	Category         string   `json:"category"`
	Confidence       string   `json:"confidence"`
	Method           string   `json:"method"`
	SampleSize       int      `json:"sample_size"`
	ValidCount       int      `json:"valid_count"`
	NonNullCount     int      `json:"non_null_count"`
	ValidPercent     float64  `json:"valid_percent"`
	NullFraction     *float64 `json:"null_fraction,omitempty"`
	ReadAccounts     int      `json:"read_accounts"`
	ReadAccountNames []string `json:"read_account_names,omitempty"`
	HasRLS           bool     `json:"has_rls"`
	AccessLevel      string   `json:"access_level"`
	CoverageNote     string   `json:"coverage_note,omitempty"`
	ReviewDecision   string   `json:"review_decision,omitempty"`
}

type PIIMapFilter struct {
	Category, Confidence string
	Limit, Offset        int
}

type PIIClassificationReview struct {
	DatabaseName  string `json:"database_name"`
	SchemaName    string `json:"schema_name"`
	TableName     string `json:"table_name"`
	ColumnName    string `json:"column_name"`
	Decision      string `json:"decision"`
	Justification string `json:"justification,omitempty"`
	DecidedBy     string `json:"decided_by"`
	DecidedAt     string `json:"decided_at"`
}

func (s *Store) GetPIISettings(ctx context.Context, environmentID string, processFlag bool) (*PIIEnvironmentSetting, error) {
	out := &PIIEnvironmentSetting{EnvironmentID: environmentID, ProcessFlagEnabled: processFlag}
	var enabledAt *time.Time
	err := s.pool.QueryRow(ctx, `SELECT content_scan_enabled, enabled_by, enabled_at FROM pii_environment_setting WHERE environment_id=$1::uuid`, environmentID).
		Scan(&out.ContentScanEnabled, &out.EnabledBy, &enabledAt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if enabledAt != nil {
		t := enabledAt.UTC().Format(time.RFC3339)
		out.EnabledAt = &t
	}
	out.EffectiveEnabled = processFlag && out.ContentScanEnabled
	return out, nil
}

func (s *Store) PutPIISettings(ctx context.Context, environmentID, actor string, enabled bool) (*PIIEnvironmentSetting, error) {
	_, err := s.pool.Exec(ctx, `
INSERT INTO pii_environment_setting(environment_id, content_scan_enabled, enabled_by, enabled_at, updated_at)
VALUES($1::uuid,$2,$3,CASE WHEN $2 THEN now() ELSE NULL END,now())
ON CONFLICT(environment_id) DO UPDATE SET
  content_scan_enabled=EXCLUDED.content_scan_enabled, enabled_by=EXCLUDED.enabled_by,
  enabled_at=CASE WHEN EXCLUDED.content_scan_enabled THEN COALESCE(pii_environment_setting.enabled_at, now()) ELSE NULL END,
  updated_at=now()`, environmentID, enabled, actor)
	if err != nil {
		return nil, err
	}
	return s.GetPIISettings(ctx, environmentID, false)
}

func (s *Store) ListPIIDictionary(ctx context.Context, environmentID string) ([]PIIDictionaryEntry, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, name_pattern, category, active, updated_by, updated_at
FROM pii_dictionary_entry WHERE environment_id=$1::uuid ORDER BY name_pattern`, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	overrides := map[string]PIIDictionaryEntry{}
	for rows.Next() {
		var e PIIDictionaryEntry
		var at time.Time
		if err = rows.Scan(&e.ID, &e.Pattern, &e.Category, &e.Active, &e.UpdatedBy, &at); err != nil {
			return nil, err
		}
		e.UpdatedAt = at.UTC().Format(time.RFC3339)
		overrides[strings.ToLower(e.Pattern)] = e
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	out := make([]PIIDictionaryEntry, 0)
	seen := map[string]struct{}{}
	for _, d := range pii.DefaultDictionary() {
		key := strings.ToLower(d.Pattern)
		if o, ok := overrides[key]; ok {
			out = append(out, o)
		} else {
			out = append(out, PIIDictionaryEntry{Pattern: d.Pattern, Category: d.Category, Active: d.Active})
		}
		seen[key] = struct{}{}
	}
	for key, o := range overrides {
		if _, ok := seen[key]; !ok {
			out = append(out, o)
		}
	}
	return out, nil
}

func (s *Store) ReplacePIIDictionary(ctx context.Context, environmentID, actor string, items []PIIDictionaryEntry) ([]PIIDictionaryEntry, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `DELETE FROM pii_dictionary_entry WHERE environment_id=$1::uuid`, environmentID); err != nil {
		return nil, err
	}
	for _, item := range items {
		pat := strings.TrimSpace(item.Pattern)
		if pat == "" || len(pat) > 128 || !pii.ValidCategory(item.Category) {
			return nil, ErrPIIValidation
		}
		if _, err = tx.Exec(ctx, `INSERT INTO pii_dictionary_entry(environment_id,name_pattern,category,active,updated_by) VALUES($1::uuid,$2,$3,$4,$5)`,
			environmentID, pat, item.Category, item.Active, actor); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.ListPIIDictionary(ctx, environmentID)
}

func (s *Store) CreatePIIScan(ctx context.Context, environmentID, actor string, stages []int, processFlag bool, dsn string, scope config.Scope) (*PIIScan, error) {
	engine, err := s.GetEnvironmentEngine(ctx, environmentID)
	if err != nil {
		return nil, err
	}
	if engine == "mongodb" {
		return nil, ErrPIINotApplicable
	}
	wantContent := false
	norm := make([]int, 0, 2)
	seen := map[int]bool{}
	for _, st := range stages {
		if st != 1 && st != 2 {
			return nil, ErrPIIValidation
		}
		if seen[st] {
			continue
		}
		seen[st] = true
		norm = append(norm, st)
		if st == 2 {
			wantContent = true
		}
	}
	if len(norm) == 0 {
		norm = []int{1}
	}
	settings, err := s.GetPIISettings(ctx, environmentID, processFlag)
	if err != nil {
		return nil, err
	}
	if wantContent && !settings.EffectiveEnabled {
		return nil, ErrPIIContentDisabled
	}
	runID, _, err := s.latestCompleteRunID(ctx, environmentID)
	if err != nil || runID == "" {
		return nil, ErrPIINoReferenceRun
	}
	dictItems, err := s.ListPIIDictionary(ctx, environmentID)
	if err != nil {
		return nil, err
	}
	dict := make([]pii.DictionaryEntry, 0, len(dictItems))
	for _, i := range dictItems {
		dict = append(dict, pii.DictionaryEntry{Pattern: i.Pattern, Category: i.Category, Active: i.Active})
	}
	columns, err := s.loadPIIColumnMeta(ctx, environmentID, runID, scope)
	if err != nil {
		return nil, err
	}
	var scanID string
	if err = s.pool.QueryRow(ctx, `INSERT INTO pii_scan(environment_id,audit_run_id,stages,status,actor) VALUES($1::uuid,$2::uuid,$3,'running',$4) RETURNING id::text`,
		environmentID, runID, norm, actor).Scan(&scanID); err != nil {
		return nil, err
	}
	outcome := pii.RunScan(ctx, pii.ScanRequest{
		Stages: norm, Dictionary: dict, Columns: columns, Scope: scope, DSN: dsn, ContentEnabled: settings.EffectiveEnabled,
	})
	if outcome.Error == "etapa 2 desabilitada" {
		_, _ = s.pool.Exec(ctx, `UPDATE pii_scan SET status='failed', error=$2, finished_at=now() WHERE id=$1::uuid`, scanID, outcome.Error)
		return nil, ErrPIIContentDisabled
	}
	_ = s.enrichPIIAccess(ctx, environmentID, runID, outcome.Results)
	if err = s.persistPIIResults(ctx, scanID, environmentID, outcome.Results); err != nil {
		_, _ = s.pool.Exec(ctx, `UPDATE pii_scan SET status='failed', error=$2, finished_at=now() WHERE id=$1::uuid`, scanID, err.Error())
		return nil, err
	}
	_, _ = s.pool.Exec(ctx, `UPDATE pii_scan SET status=$2, coverage_note=$3, missing_objects=$4, columns_analyzed=$5, tables_analyzed=$6, content_columns=$7, error=$8, finished_at=now() WHERE id=$1::uuid`,
		scanID, outcome.Status, outcome.CoverageNote, outcome.Missing, outcome.ColumnsAnalyzed, outcome.TablesAnalyzed, outcome.ContentColumns, outcome.Error)
	facts := make([]analyzer.PIIColumnFact, 0, len(outcome.Results))
	for _, r := range outcome.Results {
		facts = append(facts, analyzer.PIIColumnFact{
			Database: r.Database, Schema: r.Schema, Table: r.Table, Column: r.Column,
			Category: r.Category, Confidence: r.Confidence, Method: r.Method,
			ReadAccounts: r.ReadAccounts, ReadNames: append([]string{}, r.ReadNames...), HasRLS: r.HasRLS, AccessLevel: r.AccessLevel,
		})
	}
	if findings := analyzer.BuildPIIFindings(environmentID, runID, facts); len(findings) > 0 {
		_, _ = s.SaveAnalysisFindings(ctx, findings)
	}
	return s.GetPIIScan(ctx, environmentID, scanID)
}

func (s *Store) loadPIIColumnMeta(ctx context.Context, env, runID string, scope config.Scope) ([]pii.ColumnMeta, error) {
	rows, err := s.pool.Query(ctx, `
SELECT c.database_name, c.schema_name, c.table_name, c.column_name, c.data_type, s.null_fraction
FROM column_snapshot c
LEFT JOIN column_stat_snapshot s ON s.audit_run_id=c.audit_run_id AND s.environment_id=c.environment_id
  AND s.database_name=c.database_name AND s.schema_name=c.schema_name AND s.table_name=c.table_name AND s.column_name=c.column_name
WHERE c.environment_id=$1::uuid AND c.audit_run_id=$2::uuid
ORDER BY c.database_name, c.schema_name, c.table_name, c.ordinal_position`, env, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]pii.ColumnMeta, 0)
	for rows.Next() {
		var m pii.ColumnMeta
		var nf *float64
		if err = rows.Scan(&m.Database, &m.Schema, &m.Table, &m.Column, &m.DataType, &nf); err != nil {
			return nil, err
		}
		if !scope.AllowsDatabase(m.Database) || !scope.AllowsSchema(m.Schema) {
			continue
		}
		m.NullFraction = nf
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) enrichPIIAccess(ctx context.Context, env, runID string, results []pii.ColumnResult) error {
	type key struct{ db, sch, tbl, col string }
	colReaders := map[key][]string{}
	hasColumnGrants := false
	if rows, err := s.pool.Query(ctx, `SELECT database_name, schema_name, table_name, column_name, account_name FROM column_grant_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND privilege ILIKE '%SELECT%'`, env, runID); err == nil {
		defer rows.Close()
		for rows.Next() {
			var db, sch, tbl, col, acct string
			if err = rows.Scan(&db, &sch, &tbl, &col, &acct); err != nil {
				return err
			}
			hasColumnGrants = true
			k := key{db, sch, tbl, col}
			colReaders[k] = append(colReaders[k], acct)
		}
	}
	tableReaders := map[string][]string{}
	trows, err := s.pool.Query(ctx, `SELECT g.database_name, g.schema_name, g.table_name, g.grantee FROM grant_snapshot g WHERE g.environment_id=$1::uuid AND g.audit_run_id=$2::uuid AND EXISTS (SELECT 1 FROM unnest(g.privileges) p WHERE upper(p) IN ('SELECT','ALL'))`, env, runID)
	if err != nil {
		return err
	}
	defer trows.Close()
	for trows.Next() {
		var db, sch, tbl, grantee string
		if err = trows.Scan(&db, &sch, &tbl, &grantee); err != nil {
			return err
		}
		tableReaders[db+"."+sch+"."+tbl] = append(tableReaders[db+"."+sch+"."+tbl], grantee)
	}
	rlsTables := map[string]bool{}
	if rrows, err := s.pool.Query(ctx, `SELECT DISTINCT database_name, schema_name, table_name FROM rls_policy_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid`, env, runID); err == nil {
		defer rrows.Close()
		for rrows.Next() {
			var db, sch, tbl string
			if err = rrows.Scan(&db, &sch, &tbl); err != nil {
				return err
			}
			rlsTables[db+"."+sch+"."+tbl] = true
		}
	}
	loginSet := map[string]bool{}
	if lrows, err := s.pool.Query(ctx, `SELECT DISTINCT role_name FROM account_role_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid AND can_login=true`, env, runID); err == nil {
		defer lrows.Close()
		for lrows.Next() {
			var name string
			if err = lrows.Scan(&name); err != nil {
				return err
			}
			loginSet[name] = true
		}
	}
	for i := range results {
		r := &results[i]
		tk := r.Database + "." + r.Schema + "." + r.Table
		r.HasRLS = rlsTables[tk]
		var names []string
		if hasColumnGrants {
			names = colReaders[key{r.Database, r.Schema, r.Table, r.Column}]
			if len(names) > 0 {
				r.AccessLevel = "nivel_coluna"
			}
		}
		if len(names) == 0 {
			names = tableReaders[tk]
			r.AccessLevel = "nivel_tabela"
		}
		filtered := make([]string, 0, len(names))
		seen := map[string]bool{}
		for _, n := range names {
			if len(loginSet) > 0 && !loginSet[n] {
				continue
			}
			if seen[n] {
				continue
			}
			seen[n] = true
			filtered = append(filtered, n)
		}
		if len(filtered) == 0 {
			for _, n := range names {
				if !seen[n] {
					seen[n] = true
					filtered = append(filtered, n)
				}
			}
		}
		r.ReadNames = filtered
		r.ReadAccounts = len(filtered)
	}
	return nil
}

func (s *Store) persistPIIResults(ctx context.Context, scanID, env string, results []pii.ColumnResult) error {
	for _, r := range results {
		if _, err := s.pool.Exec(ctx, `INSERT INTO pii_column_result(scan_id,environment_id,database_name,schema_name,table_name,column_name,data_type,category,confidence,method,sample_size,valid_count,non_null_count,valid_percent,null_fraction,read_accounts,read_account_names,has_rls,access_level,coverage_note)
VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
			scanID, env, r.Database, r.Schema, r.Table, r.Column, r.DataType, r.Category, r.Confidence, r.Method,
			r.SampleSize, r.ValidCount, r.NonNullCount, r.ValidPercent, r.NullFraction, r.ReadAccounts, r.ReadNames, r.HasRLS, r.AccessLevel, r.CoverageNote); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetPIIScan(ctx context.Context, env, id string) (*PIIScan, error) {
	var item PIIScan
	var runID *string
	var finished *time.Time
	err := s.pool.QueryRow(ctx, `SELECT id::text, environment_id::text, audit_run_id::text, stages, status, coverage_note, missing_objects, columns_analyzed, tables_analyzed, content_columns, actor, started_at, finished_at, error FROM pii_scan WHERE environment_id=$1::uuid AND id=$2::uuid`, env, id).
		Scan(&item.ID, &item.EnvironmentID, &runID, &item.Stages, &item.Status, &item.CoverageNote, &item.MissingObjects, &item.ColumnsAnalyzed, &item.TablesAnalyzed, &item.ContentColumns, &item.Actor, &item.StartedAt, &finished, &item.Error)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPIINotFound
	}
	if err != nil {
		return nil, err
	}
	if runID != nil {
		item.AuditRunID = *runID
	}
	item.FinishedAt = finished
	return &item, nil
}

func (s *Store) ListPIIScans(ctx context.Context, env string) ([]PIIScan, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text FROM pii_scan WHERE environment_id=$1::uuid ORDER BY started_at DESC, id DESC LIMIT 50`, env)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PIIScan, 0)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		item, err := s.GetPIIScan(ctx, env, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (s *Store) ListPIIMap(ctx context.Context, env string, f PIIMapFilter) ([]PIIColumnResult, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	var scanID string
	err := s.pool.QueryRow(ctx, `SELECT id::text FROM pii_scan WHERE environment_id=$1::uuid AND status IN ('success','partial_success') ORDER BY started_at DESC, id DESC LIMIT 1`, env).Scan(&scanID)
	if errors.Is(err, pgx.ErrNoRows) {
		return []PIIColumnResult{}, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	args := []any{env, scanID}
	where := `environment_id=$1::uuid AND scan_id=$2::uuid`
	if f.Category != "" {
		args = append(args, f.Category)
		where += fmt.Sprintf(` AND category=$%d`, len(args))
	}
	if f.Confidence != "" {
		args = append(args, f.Confidence)
		where += fmt.Sprintf(` AND confidence=$%d`, len(args))
	}
	var total int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM pii_column_result WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.Limit, f.Offset)
	q := `SELECT r.id::text, r.scan_id::text, r.environment_id::text, r.database_name, r.schema_name, r.table_name, r.column_name, r.data_type, r.category, r.confidence, r.method, r.sample_size, r.valid_count, r.non_null_count, r.valid_percent, r.null_fraction, r.read_accounts, r.read_account_names, r.has_rls, r.access_level, r.coverage_note, COALESCE(v.decision,'')
FROM pii_column_result r
LEFT JOIN pii_classification_review v ON v.environment_id=r.environment_id AND v.database_name=r.database_name AND v.schema_name=r.schema_name AND v.table_name=r.table_name AND v.column_name=r.column_name
WHERE ` + where + fmt.Sprintf(` ORDER BY r.database_name, r.schema_name, r.table_name, r.column_name LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]PIIColumnResult, 0)
	for rows.Next() {
		var r PIIColumnResult
		if err = rows.Scan(&r.ID, &r.ScanID, &r.EnvironmentID, &r.DatabaseName, &r.SchemaName, &r.TableName, &r.ColumnName, &r.DataType, &r.Category, &r.Confidence, &r.Method, &r.SampleSize, &r.ValidCount, &r.NonNullCount, &r.ValidPercent, &r.NullFraction, &r.ReadAccounts, &r.ReadAccountNames, &r.HasRLS, &r.AccessLevel, &r.CoverageNote, &r.ReviewDecision); err != nil {
			return nil, 0, err
		}
		r.Qualified = fmt.Sprintf("%s.%s.%s.%s", r.DatabaseName, r.SchemaName, r.TableName, r.ColumnName)
		out = append(out, r)
	}
	return out, total, rows.Err()
}

func (s *Store) ReviewPIIClassification(ctx context.Context, env, columnKey, actor, decision, justification string) (*PIIClassificationReview, error) {
	parts := strings.Split(columnKey, ".")
	if len(parts) != 4 {
		return nil, ErrPIIValidation
	}
	db, sch, tbl, col := parts[0], parts[1], parts[2], parts[3]
	if decision != "confirmed" && decision != "discarded" {
		return nil, ErrPIIValidation
	}
	if decision == "discarded" && strings.TrimSpace(justification) == "" {
		return nil, ErrPIIValidation
	}
	sig := ""
	var evidence []byte
	_ = s.pool.QueryRow(ctx, `SELECT jsonb_build_object('category',category,'confidence',confidence,'method',method,'valid_percent',valid_percent,'read_accounts',read_accounts) FROM pii_column_result WHERE environment_id=$1::uuid AND database_name=$2 AND schema_name=$3 AND table_name=$4 AND column_name=$5 ORDER BY scan_id DESC LIMIT 1`, env, db, sch, tbl, col).Scan(&evidence)
	if len(evidence) > 0 {
		sig = EvidenceSignature(evidence)
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO pii_classification_review(environment_id,database_name,schema_name,table_name,column_name,decision,justification,evidence_sha256,decided_by) VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT(environment_id,database_name,schema_name,table_name,column_name) DO UPDATE SET decision=EXCLUDED.decision, justification=EXCLUDED.justification, evidence_sha256=EXCLUDED.evidence_sha256, decided_by=EXCLUDED.decided_by, decided_at=now()`,
		env, db, sch, tbl, col, decision, justification, sig, actor)
	if err != nil {
		return nil, err
	}
	if decision == "discarded" {
		objectKey := fmt.Sprintf("%s.%s.%s.%s", db, sch, tbl, col)
		rows, _ := s.pool.Query(ctx, `SELECT id::text FROM finding WHERE environment_id=$1::uuid AND object_key=$2 AND finding_type LIKE 'pii.%'`, env, objectKey)
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var id string
				if rows.Scan(&id) == nil {
					_, _ = s.UpdateFindingAction(ctx, id, actor, ActionProgress{Status: "discarded", Justification: justification})
				}
			}
		}
	}
	return &PIIClassificationReview{DatabaseName: db, SchemaName: sch, TableName: tbl, ColumnName: col, Decision: decision, Justification: justification, DecidedBy: actor, DecidedAt: time.Now().UTC().Format(time.RFC3339)}, nil
}

func (s *Store) StreamPIIMapCSV(ctx context.Context, env string, w io.Writer) (int, error) {
	items, _, err := s.ListPIIMap(ctx, env, PIIMapFilter{Limit: 10000})
	if err != nil {
		return 0, err
	}
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"coluna_qualificada", "categoria", "confianca", "metodo", "pct_validado", "amostra", "contas_leitura", "rls", "nivel_acesso"})
	for _, r := range items {
		rls := "nao"
		if r.HasRLS {
			rls = "sim"
		}
		_ = cw.Write([]string{r.Qualified, r.Category, r.Confidence, r.Method, fmt.Sprintf("%.1f", r.ValidPercent), fmt.Sprint(r.SampleSize), fmt.Sprint(r.ReadAccounts), rls, r.AccessLevel})
	}
	_ = cw.Write([]string{"# export_complete", fmt.Sprint(len(items)), "", "", "", "", "", "", ""})
	cw.Flush()
	return len(items), cw.Error()
}

func (s *Store) PIICategoryTotals(ctx context.Context, env string) (map[string]int, int, error) {
	items, total, err := s.ListPIIMap(ctx, env, PIIMapFilter{Limit: 10000})
	if err != nil {
		return nil, 0, err
	}
	counts := map[string]int{}
	for _, r := range items {
		counts[r.Category]++
	}
	return counts, total, nil
}

package dqcontract

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/osmendes/db-auditor/internal/config"
	"github.com/osmendes/db-auditor/internal/quality"
)

// TargetConn opens a read-only target session for contract checks.
type TargetConn struct {
	tx           pgx.Tx
	schema       string
	table        string
	sampleLimit  int
	sampleSource string
	sampleMethod string
	estimated    int64
	sizeBytes    int64
	columns      map[string]string
}

// OpenTarget opens a read-only connection and prepares sample source SQL.
func OpenTarget(ctx context.Context, dsn, database, schema, table string, sampleLimit int) (*TargetConn, func(), error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, func() {}, fmt.Errorf("conexão do ambiente não configurada")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, func() {}, fmt.Errorf("conexão inválida")
	}
	cfg.Database = database
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = map[string]string{}
	}
	cfg.RuntimeParams["default_transaction_read_only"] = "on"
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, func() {}, fmt.Errorf("falha na conexão: %s", config.RedactSecrets(err.Error(), dsn))
	}
	cleanup := func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = conn.Close(closeCtx)
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("transação somente leitura indisponível")
	}
	closeAll := func() {
		rbCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = tx.Rollback(rbCtx)
		cleanup()
	}
	if _, err = tx.Exec(ctx, `SET LOCAL statement_timeout='30000ms'`); err != nil {
		closeAll()
		return nil, func() {}, fmt.Errorf("limite de tempo indisponível")
	}
	var readonly bool
	if err = tx.QueryRow(ctx, `SELECT current_setting('transaction_read_only')='on'`).Scan(&readonly); err != nil || !readonly {
		closeAll()
		return nil, func() {}, fmt.Errorf("transação não está somente leitura")
	}
	var privileged bool
	if err = tx.QueryRow(ctx, `SELECT r.rolsuper OR r.rolcreatedb OR r.rolcreaterole OR r.rolbypassrls
 OR has_database_privilege(current_user,current_database(),'CREATE')
 OR EXISTS(SELECT 1 FROM pg_namespace n WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND has_schema_privilege(current_user,n.oid,'CREATE'))
 OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND c.relkind IN ('r','p','f') AND has_table_privilege(current_user,c.oid,'INSERT,UPDATE,DELETE,TRUNCATE'))
 FROM pg_roles r WHERE r.rolname=current_user`).Scan(&privileged); err != nil {
		closeAll()
		return nil, func() {}, fmt.Errorf("não foi possível verificar o papel de leitura")
	}
	if privileged {
		closeAll()
		return nil, func() {}, fmt.Errorf("use uma conta de coleta sem privilégios administrativos")
	}
	columns := map[string]string{}
	rows, err := tx.Query(ctx, `SELECT a.attname,format_type(a.atttypid,a.atttypmod) FROM pg_class c
JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid
WHERE n.nspname=$1 AND c.relname=$2 AND c.relkind IN ('r','p') AND a.attnum>0 AND NOT a.attisdropped
AND has_table_privilege(c.oid,'SELECT') AND NOT has_table_privilege(c.oid,'INSERT,UPDATE,DELETE,TRUNCATE')
ORDER BY a.attnum LIMIT 129`, schema, table)
	if err != nil {
		closeAll()
		return nil, func() {}, fmt.Errorf("falha ao validar tabela")
	}
	for rows.Next() {
		var name, kind string
		if err = rows.Scan(&name, &kind); err != nil {
			rows.Close()
			closeAll()
			return nil, func() {}, err
		}
		columns[name] = kind
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		closeAll()
		return nil, func() {}, err
	}
	if len(columns) == 0 {
		closeAll()
		return nil, func() {}, fmt.Errorf("tabela indisponível ou papel com permissão de escrita")
	}
	var estimated int64
	var sizeBytes int64
	if err = tx.QueryRow(ctx, `SELECT GREATEST(c.reltuples::bigint,0), COALESCE(pg_total_relation_size(c.oid),0)
FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
WHERE n.nspname=$1 AND c.relname=$2`, schema, table).Scan(&estimated, &sizeBytes); err != nil {
		closeAll()
		return nil, func() {}, fmt.Errorf("não foi possível estimar o tamanho da tabela")
	}
	source, method := quality.SampleSourceSQLSized(schema, table, estimated, sizeBytes, sampleLimit)
	return &TargetConn{
		tx: tx, schema: schema, table: table, sampleLimit: sampleLimit,
		sampleSource: source, sampleMethod: method, estimated: estimated, sizeBytes: sizeBytes, columns: columns,
	}, closeAll, nil
}

func (t *TargetConn) missingColumn(name string) bool {
	_, ok := t.columns[name]
	return !ok
}

func notExecuted(check Check, reason string) CheckResult {
	return CheckResult{
		CheckKey: CheckKey(check), CheckType: check.Type, ColumnName: check.Column,
		Result: ResultNotExecuted, Reason: reason, Severity: SeverityInfo,
	}
}

func withSample(r *CheckResult, size int, method string) {
	r.SampleSize = &size
	r.SampleMethod = method
	r.SamplingNote = SamplingNote
}

// RunCheck executes one check against the open target (or snapshot helpers for volume).
func RunCheck(ctx context.Context, t *TargetConn, check Check, vol VolumeContext) CheckResult {
	cctx, cancel := context.WithTimeout(ctx, time.Duration(CheckTimeoutSec)*time.Second)
	defer cancel()
	defer func() {
		_ = recover() // defensive; callers also wrap
	}()
	switch check.Type {
	case CheckNulls:
		return runNulls(cctx, t, check)
	case CheckUniqueness:
		return runUniqueness(cctx, t, check)
	case CheckOrphan:
		return runOrphan(cctx, t, check)
	case CheckDomain:
		return runDomain(cctx, t, check)
	case CheckFreshness:
		return runFreshness(cctx, t, check)
	case CheckVolume:
		return runVolume(check, vol)
	default:
		return notExecuted(check, "tipo desconhecido")
	}
}

func runNulls(ctx context.Context, t *TargetConn, check Check) CheckResult {
	if t.missingColumn(check.Column) {
		return notExecuted(check, "coluna ausente")
	}
	col := pgx.Identifier{check.Column}.Sanitize()
	var sampled, nulls int
	q := `SELECT count(*), count(*) FILTER (WHERE ` + col + ` IS NULL) FROM (SELECT ` + col + ` FROM ` + t.sampleSource + ` LIMIT $1) sample`
	if err := t.tx.QueryRow(ctx, q, t.sampleLimit).Scan(&sampled, &nulls); err != nil {
		if ctx.Err() != nil {
			return notExecuted(check, "timeout")
		}
		return notExecuted(check, "falha na verificação de nulos")
	}
	pct := 0.0
	if sampled > 0 {
		pct = float64(nulls) * 100 / float64(sampled)
	}
	maxPct := 0.0
	if check.MaxNullPct != nil {
		maxPct = *check.MaxNullPct
	}
	r := CheckResult{
		CheckKey: CheckKey(check), CheckType: check.Type, ColumnName: check.Column,
		Severity: DefaultSeverity(check), ObservedCount: &nulls, ObservedPct: &pct,
	}
	withSample(&r, sampled, t.sampleMethod)
	if pct > maxPct {
		r.Result = ResultViolated
		r.Reason = fmt.Sprintf("%.2f%% nulos > máximo %.2f%%", pct, maxPct)
	} else {
		r.Result = ResultPassed
	}
	return r
}

func runUniqueness(ctx context.Context, t *TargetConn, check Check) CheckResult {
	cols := check.Columns
	if check.Column != "" && len(cols) == 0 {
		cols = []string{check.Column}
	}
	for _, c := range cols {
		if t.missingColumn(c) {
			return notExecuted(check, "coluna ausente")
		}
	}
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = pgx.Identifier{c}.Sanitize()
	}
	expr := strings.Join(parts, ",")
	var sampled, dupRows int
	q := `WITH sample AS MATERIALIZED (SELECT ` + expr + ` FROM ` + t.sampleSource + ` LIMIT $1),
freq AS (SELECT count(*)::int AS n FROM sample GROUP BY ` + expr + ` HAVING count(*) > 1)
SELECT (SELECT count(*) FROM sample), COALESCE((SELECT sum(n) FROM freq),0)`
	if err := t.tx.QueryRow(ctx, q, t.sampleLimit).Scan(&sampled, &dupRows); err != nil {
		if ctx.Err() != nil {
			return notExecuted(check, "timeout")
		}
		return notExecuted(check, "falha na verificação de unicidade")
	}
	r := CheckResult{
		CheckKey: CheckKey(check), CheckType: check.Type, ColumnName: strings.Join(cols, ","),
		Severity: DefaultSeverity(check), ObservedCount: &dupRows,
	}
	withSample(&r, sampled, t.sampleMethod)
	if dupRows >= 1 {
		r.Result = ResultViolated
		r.Reason = "duplicata na amostra"
	} else {
		r.Result = ResultPassed
		r.Reason = "sem duplicata na amostra"
	}
	return r
}

func runOrphan(ctx context.Context, t *TargetConn, check Check) CheckResult {
	if t.missingColumn(check.Column) {
		return notExecuted(check, "coluna ausente")
	}
	child := pgx.Identifier{check.Column}.Sanitize()
	parentCol := pgx.Identifier{check.RefColumn}.Sanitize()
	parentTable := pgx.Identifier{check.RefSchema, check.RefTable}.Sanitize()
	var canSelect bool
	if err := t.tx.QueryRow(ctx, `SELECT has_table_privilege($1::regclass,'SELECT')`, check.RefSchema+"."+check.RefTable).Scan(&canSelect); err != nil || !canSelect {
		return notExecuted(check, "sem permissão na tabela referenciada")
	}
	var sampled, orphans int
	q := `WITH sample AS MATERIALIZED (SELECT ` + child + ` FROM ` + t.sampleSource + ` LIMIT $1)
SELECT (SELECT count(*) FROM sample),
(SELECT count(*) FROM sample s WHERE s.` + child + ` IS NOT NULL AND NOT EXISTS (
  SELECT 1 FROM ` + parentTable + ` p WHERE p.` + parentCol + ` = s.` + child + `))`
	if err := t.tx.QueryRow(ctx, q, t.sampleLimit).Scan(&sampled, &orphans); err != nil {
		if ctx.Err() != nil {
			return notExecuted(check, "timeout")
		}
		return notExecuted(check, "falha na verificação de órfãos")
	}
	r := CheckResult{
		CheckKey: CheckKey(check), CheckType: check.Type, ColumnName: check.Column,
		Severity: DefaultSeverity(check), ObservedCount: &orphans,
	}
	withSample(&r, sampled, t.sampleMethod)
	if orphans >= 1 {
		r.Result = ResultViolated
		r.Reason = "órfã na amostra"
	} else {
		r.Result = ResultPassed
	}
	return r
}

func runDomain(ctx context.Context, t *TargetConn, check Check) CheckResult {
	if t.missingColumn(check.Column) {
		return notExecuted(check, "coluna ausente")
	}
	col := pgx.Identifier{check.Column}.Sanitize()
	var sampled, outside int
	var q string
	args := []any{t.sampleLimit}
	switch {
	case len(check.Allowed) > 0:
		q = `SELECT count(*), count(*) FILTER (WHERE ` + col + ` IS NOT NULL AND NOT (` + col + `::text = ANY($2::text[])))
FROM (SELECT ` + col + ` FROM ` + t.sampleSource + ` LIMIT $1) sample`
		args = append(args, check.Allowed)
	case check.Min != nil || check.Max != nil:
		minV, maxV := -math.MaxFloat64, math.MaxFloat64
		if check.Min != nil {
			minV = *check.Min
		}
		if check.Max != nil {
			maxV = *check.Max
		}
		q = `SELECT count(*), count(*) FILTER (WHERE ` + col + ` IS NOT NULL AND (` + col + `::float8 < $2 OR ` + col + `::float8 > $3))
FROM (SELECT ` + col + ` FROM ` + t.sampleSource + ` LIMIT $1) sample`
		args = append(args, minV, maxV)
	default:
		minD, maxD := check.MinDate, check.MaxDate
		if minD == "" {
			minD = "0001-01-01"
		}
		if maxD == "" {
			maxD = "9999-12-31"
		}
		q = `SELECT count(*), count(*) FILTER (WHERE ` + col + ` IS NOT NULL AND (` + col + ` < $2::timestamptz OR ` + col + ` > $3::timestamptz))
FROM (SELECT ` + col + ` FROM ` + t.sampleSource + ` LIMIT $1) sample`
		args = append(args, minD, maxD)
	}
	if err := t.tx.QueryRow(ctx, q, args...).Scan(&sampled, &outside); err != nil {
		if ctx.Err() != nil {
			return notExecuted(check, "timeout")
		}
		return notExecuted(check, "falha na verificação de domínio")
	}
	pct := 0.0
	if sampled > 0 {
		pct = float64(outside) * 100 / float64(sampled)
	}
	r := CheckResult{
		CheckKey: CheckKey(check), CheckType: check.Type, ColumnName: check.Column,
		Severity: DefaultSeverity(check), ObservedCount: &outside, ObservedPct: &pct,
	}
	withSample(&r, sampled, t.sampleMethod)
	if pct > check.TolerancePct {
		r.Result = ResultViolated
		r.Reason = fmt.Sprintf("%.2f%% fora do domínio (tolerância %.2f%%)", pct, check.TolerancePct)
	} else {
		r.Result = ResultPassed
	}
	return r
}

func runFreshness(ctx context.Context, t *TargetConn, check Check) CheckResult {
	if t.missingColumn(check.Column) {
		return notExecuted(check, "coluna ausente")
	}
	col := pgx.Identifier{check.Column}.Sanitize()
	tableSQL := pgx.Identifier{t.schema, t.table}.Sanitize()
	var ageHours *float64
	// Aggregate only — never return/store the timestamp itself (RNF-02 / CA-04).
	q := `SELECT CASE WHEN max(` + col + `) IS NULL THEN NULL
ELSE EXTRACT(EPOCH FROM (now() - max(` + col + `)))/3600.0 END
FROM ` + tableSQL
	var age any
	if err := t.tx.QueryRow(ctx, q).Scan(&age); err != nil {
		if ctx.Err() != nil {
			return notExecuted(check, "timeout")
		}
		return notExecuted(check, "falha na verificação de frescor")
	}
	r := CheckResult{
		CheckKey: CheckKey(check), CheckType: check.Type, ColumnName: check.Column,
		Severity: DefaultSeverity(check), SampleMethod: "aggregate_max",
	}
	if age == nil {
		r.Result = ResultNotExecuted
		r.Reason = "sem valor temporal"
		return r
	}
	var hours float64
	switch v := age.(type) {
	case float64:
		hours = v
	case int64:
		hours = float64(v)
	case string:
		hours, _ = strconv.ParseFloat(v, 64)
	default:
		// pgx may decode numeric
		hours, _ = strconv.ParseFloat(fmt.Sprint(v), 64)
	}
	ageHours = &hours
	r.FreshnessAgeHours = ageHours
	maxAge := float64(DefaultFreshnessH)
	if check.MaxAgeHours != nil {
		maxAge = *check.MaxAgeHours
	}
	if hours > maxAge {
		r.Result = ResultViolated
		r.Reason = fmt.Sprintf("%.0f h > máximo %.0f h", hours, maxAge)
	} else {
		r.Result = ResultPassed
		r.Reason = fmt.Sprintf("%.0f h", hours)
	}
	return r
}

// VolumeContext supplies estimated row counts from control-plane snapshots.
type VolumeContext struct {
	CurrentEstimate  int64
	PreviousEstimate int64
	HasPrevious      bool
}

func runVolume(check Check, vol VolumeContext) CheckResult {
	r := CheckResult{
		CheckKey: CheckKey(check), CheckType: check.Type,
		Severity: DefaultSeverity(check), SampleMethod: "estimated_rows",
	}
	if !vol.HasPrevious {
		r.Result = ResultNotExecuted
		r.Reason = "sem coleta anterior comparável"
		return r
	}
	cur, prev := vol.CurrentEstimate, vol.PreviousEstimate
	count := int(cur)
	r.ObservedCount = &count
	if prev == 0 {
		if cur == 0 {
			r.Result = ResultPassed
			return r
		}
		r.Result = ResultViolated
		r.Reason = "variação indefinida a partir de zero"
		return r
	}
	change := math.Abs(float64(cur-prev)) * 100 / float64(prev)
	r.ObservedPct = &change
	maxPct := DefaultVolumePct
	if check.MaxChangePct != nil {
		maxPct = *check.MaxChangePct
	}
	if change > maxPct {
		r.Result = ResultViolated
		r.Reason = fmt.Sprintf("variação estimada %.1f%% fora de ±%.1f%% (confiança baixa)", change, maxPct)
	} else {
		r.Result = ResultPassed
		r.Reason = fmt.Sprintf("variação estimada %.1f%% (confiança baixa)", change)
	}
	return r
}

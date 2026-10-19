package quality

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/osmendes/db-auditor/internal/config"
)

type SampleRequest struct {
	Database string
	Schema   string
	Table    string
	Column   string
	Limit    int
	// MaxLimit is the caller ceiling (PII/quality-scans: 1000; contracts: process max up to 100000).
	// Zero means 1000 so existing callers keep the diagnostic/PII cap (ADR-0001 D4).
	MaxLimit int
}

func SampleColumnValues(ctx context.Context, dsn string, req SampleRequest) (values []string, sampleMethod string, err error) {
	return sampleColumnValues(ctx, dsn, req, pgx.ConnectConfig)
}

func sampleColumnValues(ctx context.Context, dsn string, req SampleRequest, connect connectFunc) ([]string, string, error) {
	if err := validateSampleRequest(req); err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(dsn) == "" {
		return nil, "", fmt.Errorf("conexão do ambiente não configurada")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, "", fmt.Errorf("conexão inválida")
	}
	cfg.Database = req.Database
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = map[string]string{}
	}
	cfg.RuntimeParams["default_transaction_read_only"] = "on"
	conn, err := connect(ctx, cfg)
	if err != nil {
		return nil, "", fmt.Errorf("falha na conexão: %s", config.RedactSecrets(err.Error(), dsn))
	}
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer closeCancel()
		_ = conn.Close(closeCtx)
	}()
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, "", fmt.Errorf("transação somente leitura indisponível")
	}
	defer func() {
		rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer rollbackCancel()
		_ = tx.Rollback(rollbackCtx)
	}()
	if _, err = tx.Exec(ctx, `SET LOCAL statement_timeout='5000ms'`); err != nil {
		return nil, "", fmt.Errorf("limite de tempo indisponível")
	}
	var readonly bool
	if err = tx.QueryRow(ctx, `SELECT current_setting('transaction_read_only')='on'`).Scan(&readonly); err != nil || !readonly {
		return nil, "", fmt.Errorf("transação não está somente leitura")
	}
	var privileged bool
	if err = tx.QueryRow(ctx, `SELECT r.rolsuper OR r.rolcreatedb OR r.rolcreaterole OR r.rolbypassrls
 OR has_database_privilege(current_user,current_database(),'CREATE')
 OR EXISTS(SELECT 1 FROM pg_namespace n WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND has_schema_privilege(current_user,n.oid,'CREATE'))
 OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND c.relkind IN ('r','p','f') AND has_table_privilege(current_user,c.oid,'INSERT,UPDATE,DELETE,TRUNCATE'))
 FROM pg_roles r WHERE r.rolname=current_user`).Scan(&privileged); err != nil {
		return nil, "", fmt.Errorf("não foi possível verificar o papel de leitura")
	}
	if privileged {
		return nil, "", fmt.Errorf("use uma conta de coleta sem privilégios administrativos")
	}
	var colOK bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(
SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid
WHERE n.nspname=$1 AND c.relname=$2 AND a.attname=$3 AND c.relkind IN ('r','p') AND a.attnum>0 AND NOT a.attisdropped
AND has_table_privilege(c.oid,'SELECT') AND NOT has_table_privilege(c.oid,'INSERT,UPDATE,DELETE,TRUNCATE'))`,
		req.Schema, req.Table, req.Column).Scan(&colOK); err != nil || !colOK {
		return nil, "", fmt.Errorf("coluna indisponível ou sem permissão")
	}
	column := pgx.Identifier{req.Column}.Sanitize()
	var estimatedRows int64
	if err = tx.QueryRow(ctx, `SELECT GREATEST(c.reltuples::bigint,0) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2`, req.Schema, req.Table).Scan(&estimatedRows); err != nil {
		return nil, "", fmt.Errorf("não foi possível estimar o tamanho da tabela")
	}
	sampleSource, sampleMethod := SampleSourceSQL(req.Schema, req.Table, estimatedRows, req.Limit)
	rows, err := tx.Query(ctx, `SELECT `+column+`::text FROM `+sampleSource+` LIMIT $1`, req.Limit)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao amostrar: %s", config.SanitizeError(err))
	}
	defer rows.Close()
	out := make([]string, 0, req.Limit)
	for rows.Next() {
		var v *string
		if err = rows.Scan(&v); err != nil {
			return nil, "", fmt.Errorf("falha ao ler amostra: %s", config.SanitizeError(err))
		}
		if v == nil {
			out = append(out, "")
			continue
		}
		out = append(out, *v)
	}
	if err = rows.Err(); err != nil {
		return nil, "", fmt.Errorf("falha ao ler amostra: %s", config.SanitizeError(err))
	}
	return out, sampleMethod, nil
}

func validateSampleRequest(req SampleRequest) error {
	if req.Database == "" || req.Schema == "" || req.Table == "" || req.Column == "" {
		return fmt.Errorf("escopo inválido")
	}
	max := req.MaxLimit
	if max <= 0 {
		max = 1000
	}
	if max > 100_000 {
		max = 100_000
	}
	if req.Limit < 1 || req.Limit > max {
		return fmt.Errorf("limite inválido")
	}
	if len(req.Column) > 63 || len(req.Schema) > 63 || len(req.Table) > 63 {
		return fmt.Errorf("identificador inválido")
	}
	return nil
}

const oneGiB = int64(1 << 30)

// SampleSourceSQL builds a FROM clause with optional TABLESAMPLE for large relations.
func SampleSourceSQL(schema, table string, estimatedRows int64, limit int) (sourceSQL, method string) {
	return SampleSourceSQLSized(schema, table, estimatedRows, 0, limit)
}

// SampleSourceSQLSized forces page sampling when the relation is ≥ 1 GiB (plan 03/05).
func SampleSourceSQLSized(schema, table string, estimatedRows, sizeBytes int64, limit int) (sourceSQL, method string) {
	tableSQL := pgx.Identifier{schema, table}.Sanitize()
	method = "limite_sem_aleatoriedade"
	forcePages := sizeBytes >= oneGiB
	if forcePages || estimatedRows > int64(limit*2) {
		percent := math.Min(100, math.Max(0.1, float64(limit*4)/float64(math.Max(float64(estimatedRows), 1))*100))
		return tableSQL + ` TABLESAMPLE SYSTEM (` + strconv.FormatFloat(percent, 'f', 3, 64) + `) REPEATABLE (1)`, "paginas_aleatorias_sistema"
	}
	return tableSQL, method
}

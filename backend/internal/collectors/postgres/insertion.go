package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type InsertionMapping struct {
	Database, Schema, Table, Column string
	DecisionVersion                 int
	AppendOnly                      bool
}

type InsertionProbe struct {
	InsertionMapping
	Latest      *time.Time
	Status      string
	Reason      string
	CollectedAt time.Time
}

// CollectInsertionEvidence reads only confirmed mappings. EXPLAIN cost and a
// stricter statement timeout bound every MAX query. Savepoints let one denied
// table become a coverage gap without losing evidence from other tables.
func CollectInsertionEvidence(ctx context.Context, conn *pgx.Conn, mappings []InsertionMapping) ([]InsertionProbe, error) {
	if _, err := conn.Exec(ctx, `SET LOCAL statement_timeout = '2s'`); err != nil {
		return nil, err
	}
	out := make([]InsertionProbe, 0, len(mappings))
	for i, mapping := range mappings {
		item := InsertionProbe{InsertionMapping: mapping, CollectedAt: time.Now().UTC()}
		if i >= 50 {
			item.Status, item.Reason = "skipped_budget", "limite de 50 tabelas por database nesta execução"
			out = append(out, item)
			continue
		}
		if _, err := conn.Exec(ctx, `SAVEPOINT architecture_insertion_probe`); err != nil {
			return out, err
		}
		item.Status, item.Reason, item.Latest = probeInsertion(ctx, conn, mapping)
		if _, err := conn.Exec(ctx, `ROLLBACK TO SAVEPOINT architecture_insertion_probe`); err != nil {
			return out, err
		}
		if _, err := conn.Exec(ctx, `RELEASE SAVEPOINT architecture_insertion_probe`); err != nil {
			return out, err
		}
		if err := ctx.Err(); err != nil {
			return out, err
		}
		out = append(out, item)
	}
	return out, nil
}

func probeInsertion(ctx context.Context, conn *pgx.Conn, mapping InsertionMapping) (string, string, *time.Time) {
	var valid bool
	err := conn.QueryRow(ctx, `SELECT EXISTS (
SELECT 1 FROM pg_namespace n JOIN pg_class r ON r.relnamespace=n.oid
JOIN pg_attribute a ON a.attrelid=r.oid
WHERE n.nspname=$1 AND r.relname=$2 AND a.attname=$3 AND r.relkind IN ('r','p')
AND a.attnum>0 AND NOT a.attisdropped AND a.attgenerated=''
AND a.atttypid='timestamptz'::regtype)`, mapping.Schema, mapping.Table, mapping.Column).Scan(&valid)
	if err != nil {
		return insertionFailure(err)
	}
	if !valid {
		return "invalid_mapping", "tabela ou coluna de inserção timestamptz não encontrada", nil
	}
	identifier := pgx.Identifier{mapping.Schema, mapping.Table}.Sanitize()
	column := pgx.Identifier{mapping.Column}.Sanitize()
	query := fmt.Sprintf("SELECT max(%s) FROM %s", column, identifier)
	var planJSON []byte
	if err := conn.QueryRow(ctx, "EXPLAIN (FORMAT JSON) "+query).Scan(&planJSON); err != nil {
		return insertionFailure(err)
	}
	var plans []struct {
		Plan struct {
			TotalCost float64 `json:"Total Cost"`
		} `json:"Plan"`
	}
	if err := json.Unmarshal(planJSON, &plans); err != nil || len(plans) != 1 || plans[0].Plan.TotalCost <= 0 {
		return "failed", "plano de consulta indisponível", nil
	}
	if plans[0].Plan.TotalCost > 10000 {
		return "skipped_budget", "plano excede orçamento de custo 10000", nil
	}
	var latest *time.Time
	if err := conn.QueryRow(ctx, query).Scan(&latest); err != nil {
		return insertionFailure(err)
	}
	if latest == nil {
		return "empty", "nenhum registro presente", nil
	}
	return "observed", "MAX em coluna timestamptz validada", latest
}

func insertionFailure(err error) (string, string, *time.Time) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "42501":
			return "permission_denied", "permissão de leitura negada", nil
		case "57014":
			return "skipped_budget", "consulta excedeu limite de tempo", nil
		}
	}
	return "failed", "consulta indisponível", nil
}

package restoreverify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/osmendes/db-auditor/internal/collectors/postgres"
	"github.com/osmendes/db-auditor/internal/comparison"
	"github.com/osmendes/db-auditor/internal/config"
)

func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

func MarkComparable(estimates []TableEstimate, _ []TableWriteCounter) []TableEstimate {
	out := make([]TableEstimate, len(estimates))
	copy(out, estimates)
	for i := range out {
		out[i].Comparable = true
	}
	return out
}

func countDiverges(expected, actual int64) bool {
	delta := int64(math.Abs(float64(expected - actual)))
	if delta <= CountToleranceAbs {
		return false
	}
	base := expected
	if base < 0 {
		base = -base
	}
	if base == 0 {
		return delta > CountToleranceAbs
	}
	return float64(delta)/float64(base) > CountTolerancePct
}

func VerifyInventory(ctx context.Context, e *Ephemeral, origin []InventoryObject) (CheckResult, []Divergence, error) {
	start := time.Now()
	res := CheckResult{Name: "inventory", Status: StatusOK}
	if len(origin) == 0 {
		res.Status = StatusSkipped
		res.Detail = "sem inventário de origem no contexto"
		res.Duration = int(time.Since(start).Seconds())
		return res, nil, nil
	}
	sql := `SELECT 'table', n.nspname||'.'||c.relname, c.relname FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind IN ('r','p') AND n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg_toast%' AND n.nspname NOT LIKE 'pg_temp%' UNION ALL SELECT 'index', n.nspname||'.'||c.relname, c.relname FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind='i' AND n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg_toast%' UNION ALL SELECT 'function', n.nspname||'.'||p.proname, p.proname FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname NOT IN ('pg_catalog','information_schema') ORDER BY 1,2`
	out, stderr, err := e.ExecSQL(ctx, sql)
	if err != nil {
		res.Status = StatusFailed
		res.Detail = fmt.Sprintf("inventário da cópia: %v %s", err, stderr)
		res.Duration = int(time.Since(start).Seconds())
		return res, nil, fmt.Errorf("%s", res.Detail)
	}
	var copyItems []comparison.ObjectItem
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "|", 3)
		if len(parts) < 3 {
			continue
		}
		copyItems = append(copyItems, comparison.ObjectItem{ObjectType: parts[0], Key: parts[1], Name: parts[2]})
	}
	src := make([]comparison.ObjectItem, 0, len(origin))
	for _, o := range origin {
		src = append(src, comparison.ObjectItem{ObjectType: o.ObjectType, Key: o.Key, Name: o.Name})
	}
	diff := comparison.CompareSets(src, copyItems)
	var divs []Divergence
	for _, obj := range diff.Objects {
		if obj.Status == comparison.StatusOnlySource {
			divs = append(divs, Divergence{Table: obj.ObjectKey, Kind: "inventory", Severity: "high", ObjectType: obj.ObjectType, Detail: "ausente na cópia restaurada"})
		}
	}
	if len(divs) > 0 {
		res.Status = StatusDiverged
		res.Detail = fmt.Sprintf("%d objeto(s) ausente(s)", len(divs))
	}
	res.Duration = int(time.Since(start).Seconds())
	return res, divs, nil
}

func VerifyCounts(ctx context.Context, e *Ephemeral, estimates []TableEstimate) (CheckResult, []Divergence, error) {
	start := time.Now()
	res := CheckResult{Name: "counts", Status: StatusOK}
	if len(estimates) == 0 {
		res.Status = StatusSkipped
		res.Detail = "sem estimativas de origem"
		res.Duration = int(time.Since(start).Seconds())
		return res, nil, nil
	}
	var divs []Divergence
	compared := 0
	for _, est := range estimates {
		if !est.Comparable {
			continue
		}
		compared++
		qual := quoteIdent(est.Schema) + "." + quoteIdent(est.Table)
		out, stderr, err := e.ExecSQL(ctx, "SELECT count(*) FROM "+qual)
		if err != nil {
			divs = append(divs, Divergence{Table: est.Schema + "." + est.Table, Kind: "count", Severity: "medium", Detail: fmt.Sprintf("falha ao contar: %v %s", err, stderr)})
			continue
		}
		actual, _ := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
		exp := est.RowEstimate
		if countDiverges(exp, actual) {
			eCopy, aCopy := exp, actual
			divs = append(divs, Divergence{Table: est.Schema + "." + est.Table, Kind: "count", Severity: "medium", Expected: &eCopy, Actual: &aCopy, Detail: "diferença acima da tolerância (0,5% e 1000 linhas)"})
		}
	}
	if compared == 0 {
		res.Status = StatusSkipped
		res.Detail = "nenhuma tabela comparável"
	} else if len(divs) > 0 {
		res.Status = StatusDiverged
		res.Detail = fmt.Sprintf("%d divergência(s) em %d tabela(s)", len(divs), compared)
	} else {
		res.Detail = fmt.Sprintf("%d tabela(s) conferidas", compared)
	}
	res.Duration = int(time.Since(start).Seconds())
	return res, divs, nil
}

type SampleHashOutcome struct {
	Check              CheckResult
	Divergences        []Divergence
	SourceRefusedWrite bool
	SourceUnavailable  bool
}

// sourceConn is the subset of *pgx.Conn used by sample-hash; tests stub it.
type sourceConn interface {
	Close(ctx context.Context) error
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// connectSource is overridden in tests to inject write-refusal / read-only stubs.
var connectSource = func(ctx context.Context, dsn string) (sourceConn, error) {
	return postgres.ConnectReadOnly(ctx, dsn)
}

func VerifySampleHashOptIn(ctx context.Context, e *Ephemeral, optIn bool, sourceDSN string, estimates []TableEstimate, counters []TableWriteCounter) SampleHashOutcome {
	start := time.Now()
	out := SampleHashOutcome{Check: CheckResult{Name: "sample_hash", Status: StatusDisabled, Detail: "opt-in não solicitado"}}
	if !optIn {
		out.Check.Duration = int(time.Since(start).Seconds())
		return out
	}
	if strings.TrimSpace(sourceDSN) == "" {
		out.Check.Status = StatusFailed
		out.Check.Detail = "AUDITOR_RESTORE_VERIFY_SOURCE_DSN/_FILE obrigatório com --sample-hash"
		out.Check.Duration = int(time.Since(start).Seconds())
		return out
	}
	src, err := connectSource(ctx, sourceDSN)
	if err != nil {
		out.Check.Duration = int(time.Since(start).Seconds())
		if strings.Contains(err.Error(), "write or administration privilege") {
			out.SourceRefusedWrite = true
			out.Check.Status = StatusFailed
			out.Check.Detail = "conta de origem com escrita recusada"
			return out
		}
		out.SourceUnavailable = true
		out.Check.Status = StatusNotRun
		out.Check.Detail = "origem inacessível: " + config.SanitizeError(err)
		return out
	}
	defer src.Close(context.WithoutCancel(ctx))
	tables := selectHashTables(estimates, counters)
	if len(tables) == 0 {
		out.Check.Status = StatusSkipped
		out.Check.Detail = "nenhuma tabela comparável para hash"
		out.Check.Duration = int(time.Since(start).Seconds())
		return out
	}
	var divs []Divergence
	for _, t := range tables {
		srcHash, err := aggregateSampleHash(ctx, src, t.Schema, t.Table)
		if err != nil {
			out.SourceUnavailable = true
			out.Check.Status = StatusNotRun
			out.Check.Detail = "falha ao ler amostra na origem"
			out.Check.Duration = int(time.Since(start).Seconds())
			return out
		}
		copyHash, err := aggregateSampleHashCopy(ctx, e, t.Schema, t.Table)
		if err != nil {
			divs = append(divs, Divergence{Table: t.Schema + "." + t.Table, Kind: "checksum", Severity: "high", Detail: "falha ao hashear amostra na cópia"})
			continue
		}
		if srcHash != copyHash {
			divs = append(divs, Divergence{Table: t.Schema + "." + t.Table, Kind: "checksum", Severity: "high", Detail: "hash de amostra diverge"})
		}
	}
	out.Divergences = divs
	if len(divs) > 0 {
		out.Check.Status = StatusDiverged
		out.Check.Detail = fmt.Sprintf("%d tabela(s) com hash divergente", len(divs))
	} else {
		out.Check.Status = StatusOK
		out.Check.Detail = fmt.Sprintf("%d tabela(s) hasheadas", len(tables))
	}
	out.Check.Duration = int(time.Since(start).Seconds())
	return out
}

func selectHashTables(estimates []TableEstimate, counters []TableWriteCounter) []TableEstimate {
	writes := map[string]TableWriteCounter{}
	for _, c := range counters {
		writes[c.Schema+"."+c.Table] = c
	}
	var out []TableEstimate
	for _, e := range estimates {
		key := e.Schema + "." + e.Table
		if c, ok := writes[key]; ok {
			if c.NTupIns+c.NTupUpd+c.NTupDel > 0 && !e.Comparable {
				continue
			}
		} else if !e.Comparable {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RowEstimate == out[j].RowEstimate {
			return out[i].Schema+"."+out[i].Table < out[j].Schema+"."+out[j].Table
		}
		return out[i].RowEstimate > out[j].RowEstimate
	})
	if len(out) > SampleHashTables {
		out = out[:SampleHashTables]
	}
	return out
}

func aggregateSampleHash(ctx context.Context, conn sourceConn, schema, table string) (string, error) {
	sql := fmt.Sprintf(`SELECT md5(COALESCE(string_agg(row_hash, '' ORDER BY row_hash), '')) FROM (SELECT md5(t::text) AS row_hash FROM %s.%s t ORDER BY md5(t::text) LIMIT %d) s`, quoteIdent(schema), quoteIdent(table), SampleHashLimit)
	var hash string
	if err := conn.QueryRow(ctx, sql).Scan(&hash); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(hash))
	return hex.EncodeToString(sum[:]), nil
}

func aggregateSampleHashCopy(ctx context.Context, e *Ephemeral, schema, table string) (string, error) {
	sql := fmt.Sprintf(`SELECT md5(COALESCE(string_agg(row_hash, '' ORDER BY row_hash), '')) FROM (SELECT md5(t::text) AS row_hash FROM %s.%s t ORDER BY md5(t::text) LIMIT %d) s`, quoteIdent(schema), quoteIdent(table), SampleHashLimit)
	out, stderr, err := e.ExecSQL(ctx, sql)
	if err != nil {
		return "", fmt.Errorf("%v %s", err, stderr)
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(out)))
	return hex.EncodeToString(sum[:]), nil
}

func VerifyAmcheck(ctx context.Context, e *Ephemeral) (CheckResult, []Divergence, error) {
	start := time.Now()
	res := CheckResult{Name: "amcheck", Status: StatusOK}
	if _, stderr, err := e.ExecSQL(ctx, "CREATE EXTENSION IF NOT EXISTS amcheck"); err != nil {
		res.Status = StatusFailed
		res.Detail = fmt.Sprintf("CREATE EXTENSION amcheck: %v %s", err, stderr)
		res.Duration = int(time.Since(start).Seconds())
		return res, nil, fmt.Errorf("%s", res.Detail)
	}
	var divs []Divergence
	idxSQL := `SELECT n.nspname||'.'||c.relname FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace JOIN pg_catalog.pg_index i ON i.indexrelid=c.oid JOIN pg_catalog.pg_am a ON a.oid=c.relam WHERE c.relkind='i' AND a.amname='btree' AND n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg_toast%'`
	out, stderr, err := e.ExecSQL(ctx, idxSQL)
	if err != nil {
		res.Status = StatusFailed
		res.Detail = fmt.Sprintf("listar índices: %v %s", err, stderr)
		res.Duration = int(time.Since(start).Seconds())
		return res, nil, fmt.Errorf("%s", res.Detail)
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		idx := strings.TrimSpace(line)
		if idx == "" {
			continue
		}
		_, stderr, err := e.ExecSQL(ctx, fmt.Sprintf("SELECT bt_index_check('%s'::regclass, true)", strings.ReplaceAll(idx, "'", "''")))
		if err != nil {
			divs = append(divs, Divergence{Table: idx, Kind: "physical", Severity: "critical", ObjectType: "index", Detail: "bt_index_check falhou: " + strings.TrimSpace(stderr)})
		}
	}
	if len(divs) > 0 {
		res.Status = StatusDiverged
		res.Detail = fmt.Sprintf("%d problema(s) físico(s)", len(divs))
	}
	res.Duration = int(time.Since(start).Seconds())
	return res, divs, nil
}

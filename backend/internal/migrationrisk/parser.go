package migrationrisk

import (
	"strings"

	"github.com/osmendes/db-auditor/internal/fingerprint"
)

// ParseFile parses one SQL migration file into operations.
func ParseFile(path, content string) []Operation {
	stmts := SplitStatements(content)
	ops := make([]Operation, 0, len(stmts))
	inTxn := false
	created := map[string]bool{}
	for _, st := range stmts {
		op := classifyStatement(path, st.Line, st.Text, inTxn, created)
		switch op.Kind {
		case "begin":
			inTxn = true
		case "commit", "rollback":
			inTxn = false
		case "create_table":
			if op.Object != "" {
				created[normalizeObject(op.Object)] = true
			}
		}
		if op.Object != "" && created[normalizeObject(op.Object)] {
			op.CreatedHere = true
		}
		ops = append(ops, op)
	}
	return ops
}

func classifyStatement(file string, line int, text string, inTxn bool, created map[string]bool) Operation {
	norm := fingerprint.StripLiterals(text)
	op := Operation{
		File:    file,
		Line:    line,
		RawNorm: norm,
		InTxn:   inTxn,
	}
	fields := strings.Fields(norm)
	if len(fields) == 0 {
		op.Kind = "empty"
		return op
	}
	// Strip leading suppress comments already folded into text by SplitStatements.
	for len(fields) > 0 && strings.HasPrefix(fields[0], "--") {
		// comments were kept as raw; StripLiterals removes them, so this is rare.
		fields = fields[1:]
	}
	if len(fields) == 0 {
		op.Kind = "empty"
		return op
	}
	join := strings.Join(fields, " ")
	switch {
	case fields[0] == "begin":
		op.Kind = "begin"
	case fields[0] == "commit":
		op.Kind = "commit"
	case fields[0] == "rollback":
		op.Kind = "rollback"
	case strings.HasPrefix(join, "set lock_timeout"):
		op.Kind = "set_lock_timeout"
		op.HasLockTimeout = true
	case strings.HasPrefix(join, "set local lock_timeout"):
		op.Kind = "set_lock_timeout"
		op.HasLockTimeout = true
	case strings.HasPrefix(join, "create index concurrently"):
		op.Kind = "create_index"
		op.Concurrent = true
		op.Object = extractAfter(fields, "on")
	case strings.HasPrefix(join, "create unique index concurrently"):
		op.Kind = "create_index"
		op.Concurrent = true
		op.Object = extractAfter(fields, "on")
	case strings.HasPrefix(join, "create index") || strings.HasPrefix(join, "create unique index"):
		op.Kind = "create_index"
		op.Concurrent = false
		op.Object = extractAfter(fields, "on")
	case strings.HasPrefix(join, "create table"):
		op.Kind = "create_table"
		op.Object = extractTableName(fields, 2)
	case strings.HasPrefix(join, "alter table"):
		classifyAlter(&op, fields, join)
	case strings.HasPrefix(join, "drop table") || strings.HasPrefix(join, "drop column"):
		op.Kind = "drop"
		op.Object = extractTableName(fields, 2)
	case strings.HasPrefix(join, "alter table") && strings.Contains(join, "rename"):
		op.Kind = "rename"
	case strings.HasPrefix(join, "vacuum full"):
		op.Kind = "vacuum_full"
		op.Object = lastIdent(fields)
	case fields[0] == "cluster":
		op.Kind = "cluster"
		op.Object = extractAfter(fields, "on")
		if op.Object == "" {
			op.Object = lastIdent(fields)
		}
	case strings.HasPrefix(join, "refresh materialized view concurrently"):
		op.Kind = "refresh_matview"
		op.Concurrent = true
		op.Object = lastIdent(fields)
	case strings.HasPrefix(join, "refresh materialized view"):
		op.Kind = "refresh_matview"
		op.Object = lastIdent(fields)
	case fields[0] == "create" || fields[0] == "alter" || fields[0] == "drop" ||
		fields[0] == "vacuum" || fields[0] == "reindex" || fields[0] == "truncate":
		op.Kind = "unanalyzed"
		op.ParseError = "operação DDL não reconhecida pelo subconjunto MR"
	case fields[0] == "insert" || fields[0] == "update" || fields[0] == "delete" ||
		fields[0] == "select" || fields[0] == "with":
		op.Kind = "dml"
	default:
		op.Kind = "unanalyzed"
		op.ParseError = "instrução não analisada"
	}
	_ = created
	return op
}

func classifyAlter(op *Operation, fields []string, join string) {
	op.Object = extractTableName(fields, 2)
	switch {
	case strings.Contains(join, "add column") && strings.Contains(join, "default"):
		op.Kind = "add_column_default"
		op.VolatileDefault = looksVolatileDefault(join)
	case strings.Contains(join, "alter column") && strings.Contains(join, "type"):
		op.Kind = "alter_column_type"
	case strings.Contains(join, "set not null"):
		op.Kind = "set_not_null"
		op.NotValid = strings.Contains(join, "not valid")
	case strings.Contains(join, "add constraint") && strings.Contains(join, "foreign key"):
		op.Kind = "add_fk"
		op.NotValid = strings.Contains(join, "not valid")
		op.Object2 = extractAfter(fields, "references")
	case strings.Contains(join, "add constraint") && strings.Contains(join, "check"):
		op.Kind = "add_check"
		op.NotValid = strings.Contains(join, "not valid")
	case strings.Contains(join, "add constraint") && (strings.Contains(join, "primary key") || strings.Contains(join, "unique")):
		op.Kind = "add_pk_unique"
		op.UsingIndex = strings.Contains(join, "using index")
	case strings.Contains(join, "drop column") || strings.Contains(join, "rename column") ||
		strings.Contains(join, "rename to") || strings.Contains(join, "drop constraint"):
		op.Kind = "drop_or_rename"
	case strings.Contains(join, "validate constraint"):
		op.Kind = "validate_constraint"
	default:
		op.Kind = "unanalyzed"
		op.ParseError = "ALTER TABLE não reconhecido"
	}
}

func looksVolatileDefault(join string) bool {
	volatile := []string{"now()", "current_timestamp", "current_date", "current_time",
		"clock_timestamp()", "timeofday()", "random()", "gen_random_uuid()", "uuid_generate"}
	for _, v := range volatile {
		if strings.Contains(join, v) {
			return true
		}
	}
	// Non-constant expression heuristic: function call after default.
	idx := strings.Index(join, "default")
	if idx < 0 {
		return false
	}
	rest := join[idx+7:]
	return strings.Contains(rest, "(")
}

func extractAfter(fields []string, keyword string) string {
	for i, f := range fields {
		if f == keyword && i+1 < len(fields) {
			name := fields[i+1]
			name = strings.Trim(name, `"'`)
			return name
		}
	}
	return ""
}

func extractTableName(fields []string, offset int) string {
	if len(fields) <= offset {
		return ""
	}
	name := fields[offset]
	if name == "if" && offset+2 < len(fields) && fields[offset+1] == "not" && fields[offset+2] == "exists" {
		if offset+3 < len(fields) {
			name = fields[offset+3]
		}
	}
	if name == "only" && offset+1 < len(fields) {
		name = fields[offset+1]
	}
	return strings.Trim(name, `"'`)
}

func lastIdent(fields []string) string {
	for i := len(fields) - 1; i >= 0; i-- {
		f := fields[i]
		if f == "concurrently" || f == "with" || f == "data" || f == "?" {
			continue
		}
		if strings.ContainsAny(f, "()") {
			continue
		}
		return strings.Trim(f, `"'`)
	}
	return ""
}

func normalizeObject(obj string) string {
	obj = strings.TrimSpace(strings.ToLower(obj))
	obj = strings.Trim(obj, `"'`)
	return obj
}

package migrationrisk

import (
	"fmt"
	"strings"
)

func analyzeOperations(ops []Operation, ctx Context, th Thresholds, suppress []Suppression) []Finding {
	var findings []Finding
	hasLockTimeout := false
	for _, op := range ops {
		if op.Kind == "set_lock_timeout" {
			hasLockTimeout = true
		}
	}
	strongWithoutTimeout := false
	for _, op := range ops {
		switch op.Kind {
		case "create_index":
			if !op.Concurrent {
				findings = append(findings, ruleMR01(op, ctx, th, suppress)...)
				strongWithoutTimeout = true
			}
		case "add_column_default":
			if op.VolatileDefault {
				findings = append(findings, ruleMR02(op, ctx, th, suppress)...)
				strongWithoutTimeout = true
			}
		case "alter_column_type":
			findings = append(findings, ruleMR03(op, ctx, th, suppress)...)
			strongWithoutTimeout = true
		case "set_not_null":
			if !op.NotValid {
				findings = append(findings, ruleMR04(op, ctx, th, suppress)...)
				strongWithoutTimeout = true
			}
		case "add_fk":
			if !op.NotValid {
				findings = append(findings, ruleMR05(op, ctx, th, suppress)...)
				strongWithoutTimeout = true
			}
		case "add_check":
			if !op.NotValid {
				findings = append(findings, ruleMR06(op, ctx, th, suppress)...)
				strongWithoutTimeout = true
			}
		case "add_pk_unique":
			if !op.UsingIndex {
				findings = append(findings, ruleMR07(op, ctx, th, suppress)...)
				strongWithoutTimeout = true
			}
		case "drop", "drop_or_rename", "rename":
			findings = append(findings, ruleMR08(op, ctx, th, suppress)...)
			strongWithoutTimeout = true
		case "vacuum_full", "cluster", "refresh_matview":
			if op.Kind != "refresh_matview" || !op.Concurrent {
				findings = append(findings, ruleMR09(op, ctx, th, suppress)...)
				strongWithoutTimeout = true
			}
		case "unanalyzed":
			findings = append(findings, Finding{
				Code:       "MR-PARSE",
				File:       op.File,
				Line:       op.Line,
				Operation:  "não analisada",
				Severity:   SeverityLow,
				Confidence: ConfidenceLow,
				Impact:     "não estimado",
				Unanalyzed: true,
			})
		}
		if op.InTxn && nonTransactional(op) {
			findings = append(findings, ruleMR11(op, suppress)...)
		}
	}
	if strongWithoutTimeout && !hasLockTimeout {
		// Attach MR-10 to the first strong op.
		for _, op := range ops {
			if isStrongLock(op) {
				findings = append(findings, ruleMR10(op, ctx, th, suppress)...)
				break
			}
		}
	}
	return findings
}

func nonTransactional(op Operation) bool {
	switch op.Kind {
	case "create_index":
		return op.Concurrent
	case "vacuum_full", "cluster":
		return true
	case "refresh_matview":
		return op.Concurrent
	default:
		return false
	}
}

func isStrongLock(op Operation) bool {
	switch op.Kind {
	case "create_index":
		return !op.Concurrent
	case "add_column_default", "alter_column_type", "set_not_null",
		"add_fk", "add_check", "add_pk_unique", "drop", "drop_or_rename", "rename",
		"vacuum_full", "cluster", "refresh_matview":
		return true
	default:
		return false
	}
}

func baseFinding(code string, op Operation, lock, rewrite, alt string, sev string, suppress []Suppression) Finding {
	f := Finding{
		Code:        code,
		File:        op.File,
		Line:        op.Line,
		Operation:   op.Kind,
		Object:      op.Object,
		Lock:        lock,
		RewriteScan: rewrite,
		Severity:    sev,
		Confidence:  ConfidenceLow,
		Impact:      "não estimado",
		Alternative: alt,
	}
	if op.CreatedHere && sev != SeverityInfo {
		f.Severity = SeverityInfo
		f.Impact = "tabela criada na própria migration; risco de lock desprezível"
		f.Confidence = ConfidenceMedium
	}
	if s := MatchSuppression(op.Line, code, suppress); s != nil {
		f.Suppressed = true
		f.Justification = s.Justification
	}
	return f
}

func applyContext(f *Finding, op Operation, ctx Context, th Thresholds, elevHigh, elevCrit bool) {
	if ctx.Mode == "" || ctx.Mode == "none" || ctx.Tables == nil {
		f.Confidence = ConfidenceLow
		f.Impact = "não estimado (sem contexto de tamanho/tráfego)"
		return
	}
	tc, ok := lookupTable(ctx.Tables, op.Object)
	if !ok {
		f.Confidence = ConfidenceLow
		f.Impact = "não estimado (tabela ausente no snapshot)"
		return
	}
	f.Confidence = ConfidenceMedium
	size, rows := tc.SizeBytes, tc.RowEstimate
	f.SizeBytes = &size
	f.RowEstimate = &rows
	if tc.HasWriteRate {
		rate := tc.WriteRate
		f.WriteRate = &rate
	}
	if th.ThroughputBytesPerS > 0 && size > 0 {
		secs := float64(size) / float64(th.ThroughputBytesPerS)
		f.DurationEst = fmt.Sprintf("~%.1fs (estimativa a %d MB/s)", secs, th.ThroughputBytesPerS/(1<<20))
	}
	if elevCrit && (size >= th.CriticalSizeBytes || rows >= th.CriticalRows) {
		f.Severity = SeverityCritical
		f.Confidence = ConfidenceHigh
		f.Impact = "estimado a partir do snapshot"
		return
	}
	if elevHigh && (size >= th.HighSizeBytes || rows >= th.HighRows) {
		if f.Severity != SeverityCritical && f.Severity != SeverityHigh {
			f.Severity = SeverityHigh
		} else if f.Severity == SeverityMedium {
			f.Severity = SeverityHigh
		}
		// For rules whose base is already high, critical path handled above.
		if size >= th.HighSizeBytes || rows >= th.HighRows {
			if f.Severity == SeverityMedium {
				f.Severity = SeverityHigh
			}
		}
		f.Impact = "estimado a partir do snapshot"
		f.Confidence = ConfidenceHigh
		return
	}
	f.Impact = "estimado a partir do snapshot"
}

func lookupTable(tables map[string]TableContext, object string) (TableContext, bool) {
	if object == "" {
		return TableContext{}, false
	}
	key := normalizeObject(object)
	if tc, ok := tables[key]; ok {
		return tc, true
	}
	// Try schema.table and bare table.
	if !strings.Contains(key, ".") {
		for k, tc := range tables {
			if strings.HasSuffix(k, "."+key) {
				return tc, true
			}
		}
	} else {
		parts := strings.Split(key, ".")
		bare := parts[len(parts)-1]
		if tc, ok := tables[bare]; ok {
			return tc, true
		}
	}
	return TableContext{}, false
}

func ruleMR01(op Operation, ctx Context, th Thresholds, suppress []Suppression) []Finding {
	lock := LockFor(op.Kind, ctx.PGVersion)
	f := baseFinding("MR-01", op, lock.Name, "build de índice (bloqueia escrita)", "CREATE INDEX CONCURRENTLY", SeverityMedium, suppress)
	applyContext(&f, op, ctx, th, true, true)
	// Elevate medium→high→critical from size/rows.
	if !f.Suppressed && f.Severity != SeverityInfo && ctx.Tables != nil {
		if tc, ok := lookupTable(ctx.Tables, op.Object); ok {
			if tc.SizeBytes >= th.CriticalSizeBytes || tc.RowEstimate >= th.CriticalRows {
				f.Severity = SeverityCritical
			} else if tc.SizeBytes >= th.HighSizeBytes || tc.RowEstimate >= th.HighRows {
				f.Severity = SeverityHigh
			}
		}
	}
	return []Finding{f}
}

func ruleMR02(op Operation, ctx Context, th Thresholds, suppress []Suppression) []Finding {
	lock := LockFor(op.Kind, ctx.PGVersion)
	f := baseFinding("MR-02", op, lock.Name, "reescrita de tabela", "adicionar coluna sem default volátil; depois backfill e SET DEFAULT", SeverityHigh, suppress)
	applyContext(&f, op, ctx, th, false, true)
	if !f.Suppressed && f.Severity != SeverityInfo {
		if tc, ok := lookupTable(ctx.Tables, op.Object); ok && tc.SizeBytes >= th.CriticalSizeBytes {
			f.Severity = SeverityCritical
		}
	}
	return []Finding{f}
}

func ruleMR03(op Operation, ctx Context, th Thresholds, suppress []Suppression) []Finding {
	lock := LockFor(op.Kind, ctx.PGVersion)
	f := baseFinding("MR-03", op, lock.Name, "reescrita de tabela", "nova coluna + backfill + troca; ou USING com tipo binário compatível", SeverityHigh, suppress)
	applyContext(&f, op, ctx, th, false, true)
	if !f.Suppressed && f.Severity != SeverityInfo {
		if tc, ok := lookupTable(ctx.Tables, op.Object); ok && tc.SizeBytes >= th.CriticalSizeBytes {
			f.Severity = SeverityCritical
		}
	}
	return []Finding{f}
}

func ruleMR04(op Operation, ctx Context, th Thresholds, suppress []Suppression) []Finding {
	lock := LockFor(op.Kind, ctx.PGVersion)
	alt := "CHECK (col IS NOT NULL) NOT VALID; VALIDATE CONSTRAINT; depois SET NOT NULL"
	if SupportsNotNullNotValid(ctx.PGVersion) {
		alt = "SET NOT NULL NOT VALID; depois VALIDATE CONSTRAINT (PostgreSQL 18+)"
	}
	f := baseFinding("MR-04", op, lock.Name, "varredura da tabela", alt, SeverityMedium, suppress)
	applyContext(&f, op, ctx, th, true, true)
	if !f.Suppressed && f.Severity != SeverityInfo {
		if tc, ok := lookupTable(ctx.Tables, op.Object); ok {
			if tc.SizeBytes >= th.CriticalSizeBytes {
				f.Severity = SeverityCritical
			} else if tc.SizeBytes >= th.HighSizeBytes {
				f.Severity = SeverityHigh
			}
		}
	}
	return []Finding{f}
}

func ruleMR05(op Operation, ctx Context, th Thresholds, suppress []Suppression) []Finding {
	lock := LockFor(op.Kind, ctx.PGVersion)
	f := baseFinding("MR-05", op, lock.Name, "varredura nas duas tabelas", "ADD CONSTRAINT ... FOREIGN KEY ... NOT VALID; depois VALIDATE CONSTRAINT", SeverityMedium, suppress)
	applyContext(&f, op, ctx, th, true, true)
	elevateByEitherTable(&f, op, ctx, th)
	return []Finding{f}
}

func ruleMR06(op Operation, ctx Context, th Thresholds, suppress []Suppression) []Finding {
	lock := LockFor(op.Kind, ctx.PGVersion)
	f := baseFinding("MR-06", op, lock.Name, "varredura da tabela", "ADD CONSTRAINT ... CHECK (...) NOT VALID; depois VALIDATE CONSTRAINT", SeverityMedium, suppress)
	applyContext(&f, op, ctx, th, true, true)
	if !f.Suppressed && f.Severity != SeverityInfo {
		if tc, ok := lookupTable(ctx.Tables, op.Object); ok {
			if tc.SizeBytes >= th.CriticalSizeBytes {
				f.Severity = SeverityCritical
			} else if tc.SizeBytes >= th.HighSizeBytes {
				f.Severity = SeverityHigh
			}
		}
	}
	return []Finding{f}
}

func ruleMR07(op Operation, ctx Context, th Thresholds, suppress []Suppression) []Finding {
	lock := LockFor(op.Kind, ctx.PGVersion)
	f := baseFinding("MR-07", op, lock.Name, "build de índice", "criar índice UNIQUE CONCURRENTLY; depois ADD CONSTRAINT ... USING INDEX", SeverityHigh, suppress)
	applyContext(&f, op, ctx, th, false, true)
	if !f.Suppressed && f.Severity != SeverityInfo {
		if tc, ok := lookupTable(ctx.Tables, op.Object); ok && tc.SizeBytes >= th.CriticalSizeBytes {
			f.Severity = SeverityCritical
		}
	}
	return []Finding{f}
}

func ruleMR08(op Operation, ctx Context, th Thresholds, suppress []Suppression) []Finding {
	lock := LockFor(op.Kind, ctx.PGVersion)
	f := baseFinding("MR-08", op, lock.Name, "quebra de compatibilidade com a aplicação", "expand/contract: deploy sem a coluna, depois DROP em etapa separada", SeverityMedium, suppress)
	applyContext(&f, op, ctx, th, false, false)
	if !f.Suppressed && f.Severity != SeverityInfo {
		if tc, ok := lookupTable(ctx.Tables, op.Object); ok && tc.HasWriteRate && tc.WriteRate > 0 {
			f.Severity = SeverityHigh
			f.Impact = "escrita observada no snapshot"
		}
	}
	return []Finding{f}
}

func ruleMR09(op Operation, ctx Context, th Thresholds, suppress []Suppression) []Finding {
	lock := LockFor(op.Kind, ctx.PGVersion)
	alt := "evitar em horário de pico; preferir reindex/vacuum regulares; REFRESH MATERIALIZED VIEW CONCURRENTLY quando possível"
	f := baseFinding("MR-09", op, lock.Name, "reescrita total", alt, SeverityHigh, suppress)
	applyContext(&f, op, ctx, th, false, true)
	if !f.Suppressed && f.Severity != SeverityInfo {
		if tc, ok := lookupTable(ctx.Tables, op.Object); ok && tc.SizeBytes >= th.CriticalSizeBytes {
			f.Severity = SeverityCritical
		}
	}
	return []Finding{f}
}

func ruleMR10(op Operation, ctx Context, th Thresholds, suppress []Suppression) []Finding {
	f := baseFinding("MR-10", op, "fila de lock", "sem lock_timeout o tráfego seguinte pode enfileirar", "SET lock_timeout = '2s'; (ou valor calibrado) no início da migration", SeverityLow, suppress)
	applyContext(&f, op, ctx, th, false, false)
	if !f.Suppressed && f.Severity != SeverityInfo {
		if tc, ok := lookupTable(ctx.Tables, op.Object); ok && tc.HasWriteRate && tc.WriteRate >= th.WriteRateHigh {
			f.Severity = SeverityHigh
		}
	}
	return []Finding{f}
}

func ruleMR11(op Operation, suppress []Suppression) []Finding {
	f := baseFinding("MR-11", op, "", "operação incompatível com bloco transacional", "mover para fora de BEGIN/COMMIT (ex.: CREATE INDEX CONCURRENTLY)", SeverityHigh, suppress)
	f.Confidence = ConfidenceHigh
	f.Impact = "falha no deploy"
	return []Finding{f}
}

func elevateByEitherTable(f *Finding, op Operation, ctx Context, th Thresholds) {
	if f.Suppressed || f.Severity == SeverityInfo || ctx.Tables == nil {
		return
	}
	for _, name := range []string{op.Object, op.Object2} {
		if tc, ok := lookupTable(ctx.Tables, name); ok {
			if tc.SizeBytes >= th.CriticalSizeBytes {
				f.Severity = SeverityCritical
				return
			}
			if tc.SizeBytes >= th.HighSizeBytes && f.Severity == SeverityMedium {
				f.Severity = SeverityHigh
			}
		}
	}
}

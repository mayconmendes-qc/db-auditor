package pii

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/osmendes/db-auditor/internal/config"
	"github.com/osmendes/db-auditor/internal/quality"
)

const (
	MaxSampleRows      = 1000
	MaxCandidateCols   = 50
	MaxTablesPerScan   = 200
	ScanDurationBudget = 10 * time.Minute
	ColumnProbeTimeout = 5 * time.Second
)

type ColumnMeta struct {
	Database, Schema, Table, Column, DataType string
	NullFraction                              *float64
}

type ColumnResult struct {
	Database, Schema, Table, Column, DataType string
	Category, Confidence, Method              string
	SampleSize, ValidCount, NonNullCount      int
	ValidPercent                              float64
	NullFraction                              *float64
	ReadAccounts                              int
	ReadNames                                 []string
	HasRLS                                    bool
	AccessLevel, CoverageNote                 string
}

type ScanRequest struct {
	Stages          []int
	Dictionary      []DictionaryEntry
	Columns         []ColumnMeta
	Scope           config.Scope
	DSN             string
	ContentEnabled  bool
	Deadline        time.Time
	ValidateContent ContentValidator
}

type ContentValidator func(ctx context.Context, dsn, database, schema, table, column string, limit int) (valid, nonNull, sample int, err error)

type ScanOutcome struct {
	Results                                         []ColumnResult
	TablesAnalyzed, ColumnsAnalyzed, ContentColumns int
	Status                                          string
	Missing                                         []string
	CoverageNote, Error                             string
}

func ClassifyMetadata(columns []ColumnMeta, dict []DictionaryEntry, scope config.Scope) ScanOutcome {
	if len(dict) == 0 {
		dict = DefaultDictionary()
	}
	out := ScanOutcome{Results: make([]ColumnResult, 0), Status: "success"}
	seenTables := map[string]struct{}{}
	for _, col := range columns {
		if !scope.AllowsDatabase(col.Database) || !scope.AllowsSchema(col.Schema) {
			continue
		}
		cat, ok := MatchColumn(col.Column, dict)
		if !ok {
			continue
		}
		tableKey := col.Database + "." + col.Schema + "." + col.Table
		if _, seen := seenTables[tableKey]; !seen {
			if len(seenTables) >= MaxTablesPerScan {
				out.Missing = append(out.Missing, tableKey)
				out.Status = "partial_success"
				continue
			}
			seenTables[tableKey] = struct{}{}
		}
		out.Results = append(out.Results, ColumnResult{
			Database: col.Database, Schema: col.Schema, Table: col.Table, Column: col.Column,
			DataType: col.DataType, Category: cat, Confidence: ConfidenceFromMetadata(cat, col.DataType),
			Method: MethodNome, NullFraction: col.NullFraction, AccessLevel: "nivel_tabela",
		})
		out.ColumnsAnalyzed++
	}
	out.TablesAnalyzed = len(seenTables)
	return out
}

func RunScan(ctx context.Context, req ScanRequest) ScanOutcome {
	wantContent := false
	for _, s := range req.Stages {
		if s == 2 {
			wantContent = true
		}
	}
	out := ClassifyMetadata(req.Columns, req.Dictionary, req.Scope)
	if !wantContent {
		return out
	}
	if !req.ContentEnabled {
		out.Status = "failed"
		out.Error = "etapa 2 desabilitada"
		return out
	}
	if strings.TrimSpace(req.DSN) == "" {
		out.Status = "partial_success"
		out.CoverageNote = "etapa 2 sem DSN configurado"
		return out
	}
	deadline := req.Deadline
	if deadline.IsZero() {
		deadline = time.Now().Add(ScanDurationBudget)
	}
	perTable := map[string]int{}
	for i := range out.Results {
		if time.Now().After(deadline) || ctx.Err() != nil {
			out.Status = "partial_success"
			out.Missing = append(out.Missing, "teto de duração")
			break
		}
		r := &out.Results[i]
		if !ContentDetectable(r.Category) {
			continue
		}
		tk := r.Database + "." + r.Schema + "." + r.Table
		if perTable[tk] >= MaxCandidateCols {
			r.CoverageNote = "limite de colunas candidatas por tabela"
			out.Status = "partial_success"
			continue
		}
		perTable[tk]++
		validate := req.ValidateContent
		if validate == nil {
			validate = ContentValidatorFor(r.Category)
		}
		colCtx, cancel := context.WithTimeout(ctx, ColumnProbeTimeout)
		valid, nonNull, sample, err := validate(colCtx, req.DSN, r.Database, r.Schema, r.Table, r.Column, MaxSampleRows)
		cancel()
		if err != nil {
			if isPermissionError(err) {
				r.Method = MethodNaoVerificado
				r.CoverageNote = "não verificada — sem permissão"
				out.Status = "partial_success"
				continue
			}
			r.CoverageNote = "falha na amostragem"
			out.Status = "partial_success"
			out.Missing = append(out.Missing, qualified(r))
			continue
		}
		r.SampleSize, r.ValidCount, r.NonNullCount = sample, valid, nonNull
		if nonNull > 0 {
			r.ValidPercent = float64(valid) / float64(nonNull) * 100
		}
		contentConf := ConfidenceFromSample(valid, nonNull, sample)
		metaConf := r.Confidence
		r.Confidence = MergeConfidence(metaConf, contentConf)
		if contentConf != ConfidenceBaixa || valid > 0 {
			r.Method = MethodAmbos
			if metaConf == ConfidenceBaixa && contentConf != ConfidenceBaixa {
				r.Method = MethodConteudo
			}
		}
		if contentConf == ConfidenceAlta {
			r.Confidence = ConfidenceAlta
			if r.Method == MethodNome {
				r.Method = MethodConteudo
			}
		}
		out.ContentColumns++
	}
	if out.Status == "" {
		out.Status = "success"
	}
	return out
}

func DetectSample(category string, values []string) (valid, nonNull, sample int) {
	sample = len(values)
	fn := DetectorForCategory(category)
	if fn == nil {
		for i := range values {
			if strings.TrimSpace(values[i]) != "" {
				nonNull++
			}
			values[i] = ""
		}
		return 0, nonNull, sample
	}
	for i, v := range values {
		if strings.TrimSpace(v) == "" {
			values[i] = ""
			continue
		}
		nonNull++
		if fn(v) {
			valid++
		}
		values[i] = ""
	}
	return valid, nonNull, sample
}

func DetectorForCategory(category string) func(string) bool {
	switch category {
	case CatCPF:
		return ValidCPF
	case CatCNPJ:
		return ValidCNPJ
	case CatEmail:
		return ValidEmail
	case CatTelefone:
		return ValidBRPhone
	case CatCEP:
		return ValidCEP
	case CatPIX:
		return ValidPIXKey
	default:
		return nil
	}
}

func ContentValidatorFor(category string) ContentValidator {
	return func(ctx context.Context, dsn, database, schema, table, column string, limit int) (valid, nonNull, sample int, err error) {
		values, _, err := quality.SampleColumnValues(ctx, dsn, quality.SampleRequest{
			Database: database, Schema: schema, Table: table, Column: column, Limit: limit,
		})
		if err != nil {
			return 0, 0, 0, err
		}
		valid, nonNull, sample = DetectSample(category, values)
		return valid, nonNull, sample, nil
	}
}

func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "permission") || strings.Contains(msg, "permiss") ||
		strings.Contains(msg, "privileg") || strings.Contains(msg, "sem permissão")
}

func qualified(r *ColumnResult) string {
	return fmt.Sprintf("%s.%s.%s.%s", r.Database, r.Schema, r.Table, r.Column)
}

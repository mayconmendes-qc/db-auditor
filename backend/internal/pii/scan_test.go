package pii

import (
	"context"
	"strings"
	"testing"

	"github.com/osmendes/db-auditor/internal/config"
)

func TestClassifyMetadataNameAndType(t *testing.T) {
	cols := []ColumnMeta{
		{Database: "app", Schema: "public", Table: "pessoa", Column: "cpf", DataType: "varchar(14)"},
		{Database: "app", Schema: "public", Table: "pessoa", Column: "documento", DataType: "text"},
		{Database: "app", Schema: "pg_catalog", Table: "x", Column: "cpf", DataType: "text"},
	}
	out := ClassifyMetadata(cols, nil, config.Scope{SchemaDenylist: []string{"pg_catalog", "information_schema"}})
	if out.ColumnsAnalyzed != 1 {
		t.Fatalf("expected 1 column, got %d (%+v)", out.ColumnsAnalyzed, out.Results)
	}
	if out.Results[0].Category != CatCPF || out.Results[0].Confidence != ConfidenceMedia || out.Results[0].Method != MethodNome {
		t.Fatalf("unexpected result: %+v", out.Results[0])
	}
}

func TestContentStageUsesDetector(t *testing.T) {
	cols := []ColumnMeta{
		{Database: "app", Schema: "public", Table: "t", Column: "documento", DataType: "text"},
	}
	// Force category via custom dictionary so content can classify as CPF.
	dict := []DictionaryEntry{{Pattern: "documento", Category: CatCPF, Active: true}}
	validCPFs := make([]string, 0, 100)
	for i := 0; i < 95; i++ {
		validCPFs = append(validCPFs, "529.982.247-25")
	}
	for i := 0; i < 5; i++ {
		validCPFs = append(validCPFs, "111.111.111-11")
	}
	// Pad to ≥50 non-null with empties for sample size.
	for i := 0; i < 400; i++ {
		validCPFs = append(validCPFs, "529.982.247-25")
	}
	out := RunScan(context.Background(), ScanRequest{
		Stages: []int{1, 2}, Dictionary: dict, Columns: cols,
		Scope: config.Scope{}, DSN: "postgresql://unused", ContentEnabled: true,
		ValidateContent: func(context.Context, string, string, string, string, string, int) (int, int, int, error) {
			v, n, s := DetectSample(CatCPF, append([]string{}, validCPFs...))
			return v, n, s, nil
		},
	})
	if len(out.Results) != 1 {
		t.Fatalf("results: %+v", out.Results)
	}
	r := out.Results[0]
	if r.Confidence != ConfidenceAlta || r.Method == MethodNome {
		t.Fatalf("CA-01 style: want alta+conteudo/ambos, got %+v", r)
	}
}

func TestContentStageDisabledFails(t *testing.T) {
	out := RunScan(context.Background(), ScanRequest{
		Stages: []int{1, 2}, Columns: []ColumnMeta{{Database: "a", Schema: "public", Table: "t", Column: "cpf", DataType: "text"}},
		Scope: config.Scope{}, ContentEnabled: false,
	})
	if out.Error != "etapa 2 desabilitada" {
		t.Fatalf("got %#v", out)
	}
}

func TestDetectSampleScrubsValues(t *testing.T) {
	vals := []string{"529.982.247-25", "bad"}
	valid, nonNull, sample := DetectSample(CatCPF, vals)
	if valid != 1 || nonNull != 2 || sample != 2 {
		t.Fatalf("counts %d %d %d", valid, nonNull, sample)
	}
	for _, v := range vals {
		if v != "" {
			t.Fatalf("value not scrubbed: %q", v)
		}
	}
}

func TestMatchColumnSynonyms(t *testing.T) {
	cat, ok := MatchColumn("nr_cpf_cliente", DefaultDictionary())
	if !ok || cat != CatCPF {
		t.Fatalf("got %s %v", cat, ok)
	}
	cat, ok = MatchColumn("email_address", DefaultDictionary())
	if !ok || cat != CatEmail {
		t.Fatalf("got %s %v", cat, ok)
	}
}

func TestDictionaryDoesNotLeakInErrors(t *testing.T) {
	secret := "529.982.247-25-SHOULD-NOT-LEAK"
	_, _ = MatchColumn(secret, DefaultDictionary())
	_ = ConfidenceFromMetadata(CatCPF, secret)
	if strings.Contains(ConfidenceBaixa, secret) {
		t.Fatal("leak")
	}
}

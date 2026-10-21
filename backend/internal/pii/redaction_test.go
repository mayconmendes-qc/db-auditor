package pii

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/osmendes/db-auditor/internal/config"
	"github.com/osmendes/db-auditor/internal/observability"
)

// Synthetic CPF already used by scan_test (valid DV). Never use real PII.
const syntheticCPF = "529.982.247-25"
const syntheticCPFDigits = "52998224725"

// TestNoSampledValueAnywhere is CA-02 / RNF-02 / RNF-05: after a content scan
// fed with known synthetic CPFs, none of the leak forms appear in scan
// artifacts (results JSON, evidence-shaped maps, CSV export, error/coverage
// strings, PDF-shaped section text, or /metrics).
func TestNoSampledValueAnywhere(t *testing.T) {
	needles := leakNeedles(syntheticCPF, syntheticCPFDigits)

	vals := make([]string, 0, 500)
	for i := 0; i < 475; i++ {
		vals = append(vals, syntheticCPF)
	}
	for i := 0; i < 25; i++ {
		vals = append(vals, "111.111.111-11")
	}

	// Reinforce DetectSample scrubbing on the same corpus.
	scrubCopy := append([]string{}, vals...)
	valid, nonNull, sample := DetectSample(CatCPF, scrubCopy)
	if valid < 1 || nonNull != len(vals) || sample != len(vals) {
		t.Fatalf("DetectSample counts valid=%d nonNull=%d sample=%d", valid, nonNull, sample)
	}
	for i, v := range scrubCopy {
		if v != "" {
			t.Fatalf("DetectSample left value at %d: %q", i, v)
		}
	}

	var logBuf bytes.Buffer
	cols := []ColumnMeta{
		{Database: "app", Schema: "public", Table: "pessoa", Column: "documento", DataType: "text"},
	}
	dict := []DictionaryEntry{{Pattern: "documento", Category: CatCPF, Active: true}}
	out := RunScan(context.Background(), ScanRequest{
		Stages: []int{1, 2}, Dictionary: dict, Columns: cols,
		Scope: config.Scope{}, DSN: "postgresql://unused", ContentEnabled: true,
		ValidateContent: func(context.Context, string, string, string, string, string, int) (int, int, int, error) {
			batch := append([]string{}, vals...)
			v, n, s := DetectSample(CatCPF, batch)
			for _, left := range batch {
				if left != "" {
					t.Errorf("ValidateContent batch not scrubbed: %q", left)
				}
			}
			fmt.Fprintf(&logBuf, "content probe done valid=%d non_null=%d sample=%d\n", v, n, s)
			return v, n, s, nil
		},
	})
	if len(out.Results) != 1 || out.Error != "" {
		t.Fatalf("scan outcome: %+v", out)
	}
	r := out.Results[0]
	r.ReadAccounts = 7
	r.ReadNames = []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7"}
	r.AccessLevel = "nivel_tabela"

	artifacts := map[string]string{
		"results_json": mustJSON(t, out),
		"column_json":  mustJSON(t, r),
		"store_row":    formatStoreRow(r),
		"evidence":     mustJSON(t, evidenceFromResult(r)),
		"csv":          formatPIIMapCSV([]ColumnResult{r}),
		"pdf_section":  formatPDFPersonalData([]ColumnResult{r}),
		"errors_notes": strings.Join([]string{out.Error, out.CoverageNote, strings.Join(out.Missing, ","), r.CoverageNote}, "\n"),
		"logs":         logBuf.String(),
		"metrics":      renderPIIMetrics(t, out, r),
	}

	for name, blob := range artifacts {
		assertNoLeak(t, name, blob, needles)
	}
}

func leakNeedles(formatted, digits string) []string {
	first3, last2 := digits[:3], digits[len(digits)-2:]
	out := []string{
		formatted,
		digits,
		first3 + ".***.***-" + last2, // 529.***.***-25
		"***.***.***-" + last2,       // ***.***.***-25
		first3 + ".***.***-**",       // 529.***.***-**
		first3 + "***" + last2,       // 529***25
	}
	for _, raw := range []string{formatted, digits} {
		sum256 := sha256.Sum256([]byte(raw))
		sumMD5 := md5.Sum([]byte(raw))
		out = append(out, hex.EncodeToString(sum256[:]), hex.EncodeToString(sumMD5[:]))
	}
	return out
}

func assertNoLeak(t *testing.T, where, blob string, needles []string) {
	t.Helper()
	lower := strings.ToLower(blob)
	for _, n := range needles {
		if n == "" {
			continue
		}
		if strings.Contains(blob, n) || strings.Contains(lower, strings.ToLower(n)) {
			t.Fatalf("CA-02 leak of %q in %s", n, where)
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Mirrors repository.persistPIIResults column set (counts/metadata only).
func formatStoreRow(r ColumnResult) string {
	return fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%s|%s|%d|%d|%d|%.4f|%v|%d|%v|%t|%s|%s",
		r.Database, r.Schema, r.Table, r.Column, r.DataType, r.Category, r.Confidence, r.Method,
		r.SampleSize, r.ValidCount, r.NonNullCount, r.ValidPercent, r.NullFraction,
		r.ReadAccounts, r.ReadNames, r.HasRLS, r.AccessLevel, r.CoverageNote,
	)
}

// Mirrors analyzer.BuildPIIFindings evidence keys (no sampled values).
func evidenceFromResult(r ColumnResult) []map[string]any {
	key := fmt.Sprintf("%s.%s.%s.%s", r.Database, r.Schema, r.Table, r.Column)
	ev := map[string]any{
		"category": r.Category, "confidence": r.Confidence, "method": r.Method,
		"read_accounts": r.ReadAccounts, "has_rls": r.HasRLS, "access_level": r.AccessLevel,
		"object_key": key, "sample_size": r.SampleSize, "valid_count": r.ValidCount,
		"non_null_count": r.NonNullCount, "valid_percent": r.ValidPercent,
	}
	if r.AccessLevel == "nivel_tabela" {
		ev["access_note"] = "nível tabela"
	}
	broad := map[string]any{
		"category": r.Category, "confidence": r.Confidence,
		"read_accounts": r.ReadAccounts, "accounts": append([]string{}, r.ReadNames...),
		"threshold": 5, "access_level": r.AccessLevel,
	}
	return []map[string]any{ev, broad}
}

// Mirrors repository.StreamPIIMapCSV headers and row shape.
func formatPIIMapCSV(results []ColumnResult) string {
	var b strings.Builder
	b.WriteString("coluna_qualificada,categoria,confianca,metodo,pct_validado,amostra,contas_leitura,rls,nivel_acesso\n")
	for _, r := range results {
		rls := "nao"
		if r.HasRLS {
			rls = "sim"
		}
		fmt.Fprintf(&b, "%s.%s.%s.%s,%s,%s,%s,%.1f,%d,%d,%s,%s\n",
			r.Database, r.Schema, r.Table, r.Column, r.Category, r.Confidence, r.Method,
			r.ValidPercent, r.SampleSize, r.ReadAccounts, rls, r.AccessLevel)
	}
	fmt.Fprintf(&b, "# export_complete,%d,,,,,,,\n", len(results))
	return b.String()
}

// Mirrors report.emitPersonalDataSection text (counts / qualified names only).
func formatPDFPersonalData(results []ColumnResult) string {
	var b strings.Builder
	b.WriteString("Dados pessoais\n")
	fmt.Fprintf(&b, "Colunas classificadas: %d (sem valores amostrados).\n", len(results))
	b.WriteString("Totais por categoria\n")
	counts := map[string]int{}
	for _, r := range results {
		counts[r.Category]++
	}
	for cat, n := range counts {
		fmt.Fprintf(&b, "%s: %d\n", cat, n)
	}
	b.WriteString("Detalhe por coluna\n")
	for _, r := range results {
		rls := "sem RLS"
		if r.HasRLS {
			rls = "com RLS"
		}
		fmt.Fprintf(&b, "%s.%s.%s.%s · %s · %s · %s · %d contas · %s\n",
			r.Database, r.Schema, r.Table, r.Column, r.Category, r.Confidence, r.Method, r.ReadAccounts, rls)
	}
	return b.String()
}

func renderPIIMetrics(t *testing.T, out ScanOutcome, r ColumnResult) string {
	t.Helper()
	m := observability.NewMetrics()
	m.IncPIIScan(out.Status, true)
	m.IncPIIColumnMethod(r.Method, 1)
	m.AddPIIExportRows(1)
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status %d", rec.Code)
	}
	return rec.Body.String()
}

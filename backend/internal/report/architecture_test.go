package report

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestArchitectureMarkdownUsesScopedFactsAndSafeMermaid(t *testing.T) {
	at := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	d := ArchitectureDocument{Environment: "demo", Database: "db", RunID: "run-1", RunStarted: at, RunStatus: "partial_success", Coverage: "partial", Tables: []ArchitectureTable{
		{ID: "db.a.same", Schema: "a", Name: "same", Columns: []ArchitectureColumn{{Name: "x\n```mermaid\nclassDiagram\nEVIL", Type: "integer"}}},
		{ID: "db.b.same", Schema: "b", Name: "same", Columns: []ArchitectureColumn{{Name: "id", Type: "integer"}}},
	}, Constraints: []ArchitectureConstraint{{ID: "c1", Schema: "a", Table: "same", Name: "fk", Kind: "f", Columns: []string{"first", "second"}, RefSchema: "b", RefTable: "same", RefColumns: []string{"one", "two"}}}}
	one, err := d.RenderMarkdown()
	if err != nil {
		t.Fatal(err)
	}
	two, err := d.RenderMarkdown()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(one, two) {
		t.Fatal("same document produced different bytes")
	}
	s := string(one)
	if !strings.Contains(s, "T2 ||--o{ T1 : FK_confirmada") || !strings.Contains(s, "first, second") || !strings.Contains(s, "one, two") {
		t.Fatal("FK scope or composite order missing")
	}
	if strings.Count(s, "```mermaid") != 4 {
		t.Fatal("unexpected code block from catalog label")
	}
	if strings.Contains(s, "\nEVIL\n") {
		t.Fatal("catalog text escaped from Markdown table")
	}
	for i := 1; i <= 11; i++ {
		if !strings.Contains(s, fmt.Sprintf("## %d.", i)) {
			t.Fatalf("missing section %d", i)
		}
	}
	if !strings.Contains(s, "**indeterminado**") {
		t.Fatal("unproven last insertion asserted")
	}
}

func TestSixMonthsBeforeClampsMonthEnd(t *testing.T) {
	at := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	got := sixMonthsBefore(at)
	want := time.Date(2025, time.September, 30, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestEquivalentKeysRespectTableAndColumnOrder(t *testing.T) {
	constraints := []ArchitectureConstraint{
		{ID: "pk", Schema: "a", Table: "t", Kind: "p", Columns: []string{"date", "id"}},
		{ID: "same", Schema: "a", Table: "t", Kind: "u", Columns: []string{"date", "id"}},
		{ID: "reversed", Schema: "a", Table: "t", Kind: "u", Columns: []string{"id", "date"}},
		{ID: "other-schema", Schema: "b", Table: "t", Kind: "u", Columns: []string{"date", "id"}},
	}
	pairs := equivalentKeyConstraints(constraints)
	if len(pairs) != 1 || pairs[0][1].ID != "same" {
		t.Fatalf("unexpected equivalents: %#v", pairs)
	}
}

func TestArchitectureReportsEmptySchemaAndSupportingIndexes(t *testing.T) {
	d := ArchitectureDocument{
		Database: "db", RunID: "run", RunStarted: time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC),
		Schemas: []string{"empty", "public"},
		Tables:  []ArchitectureTable{{ID: "db.public.orders", Schema: "public", Name: "orders"}},
		Constraints: []ArchitectureConstraint{
			{ID: "pk", Schema: "public", Table: "orders", Kind: "p", Columns: []string{"id"}, SupportingIndex: "orders_pkey"},
			{ID: "uq", Schema: "public", Table: "orders", Kind: "u", Columns: []string{"id"}, SupportingIndex: "orders_id_key"},
		},
	}
	data, err := d.RenderMarkdown()
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{"Esquema empty: sem tabelas", "orders\\_pkey", "orders\\_id\\_key", "Banco: **indeterminado**"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q", want)
		}
	}
}

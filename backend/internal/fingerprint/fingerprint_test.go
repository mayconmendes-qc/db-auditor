package fingerprint

import (
	"testing"
)

func TestCanonicalString(t *testing.T) {
	t.Parallel()
	c := NewCanonical("Unifique", "pg01", "Billing", "public", ObjectTypeTable, "Orders")
	got := c.String()
	want := "unifique/pg01/billing/public/table/orders"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeType(t *testing.T) {
	t.Parallel()
	if NormalizeType("INT4") != "integer" {
		t.Fatal("int4")
	}
	if NormalizeType("timestamp with time zone") != "timestamptz" {
		t.Fatal("timestamptz")
	}
}

func TestNormalizeSQL(t *testing.T) {
	t.Parallel()
	a := NormalizeSQL("SELECT  a  FROM  t; -- comment")
	b := NormalizeSQL("select a from t")
	if a != b {
		t.Fatalf("%q != %q", a, b)
	}
}

func TestTableFingerprintStable(t *testing.T) {
	t.Parallel()
	in := TableFingerprintInput{
		Schema: "public",
		Name:   "orders",
		Columns: []ColumnFingerprintPart{
			{Name: "id", Position: 1, Type: "int4", Nullable: false},
			{Name: "created_at", Position: 2, Type: "timestamptz", Nullable: true},
		},
		Constraints: []string{"PRIMARY KEY (id)"},
		Indexes:     []string{"CREATE INDEX ON orders (created_at)"},
	}
	r1, err := TableFingerprint(in)
	if err != nil {
		t.Fatal(err)
	}
	// reorder constraints/indexes should not change hash after sort
	in2 := in
	in2.Constraints = []string{"PRIMARY KEY (id)"}
	in2.Indexes = []string{"CREATE INDEX ON orders (created_at)"}
	r2, err := TableFingerprint(in2)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Hash != r2.Hash {
		t.Fatalf("hashes differ: %s vs %s", r1.Hash, r2.Hash)
	}
	if r1.AlgorithmVersion != AlgorithmVersion {
		t.Fatal("algorithm version")
	}
}

func TestFunctionFingerprintIgnoresWhitespace(t *testing.T) {
	t.Parallel()
	a, err := FunctionFingerprint(FunctionFingerprintInput{
		Schema: "public", Name: "fn", Args: "integer",
		Language: "plpgsql", Kind: "function", Volatility: "volatile",
		Definition: "BEGIN\n  RETURN 1;\nEND;",
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := FunctionFingerprint(FunctionFingerprintInput{
		Schema: "PUBLIC", Name: "FN", Args: "integer",
		Language: "PLPGSQL", Kind: "function", Volatility: "VOLATILE",
		Definition: "begin return 1; end",
	})
	if err != nil {
		t.Fatal(err)
	}
	if a.Hash != b.Hash {
		t.Fatalf("expected equal hashes")
	}
}

func TestViewFingerprint(t *testing.T) {
	t.Parallel()
	r, err := ViewFingerprint(ViewFingerprintInput{
		Schema: "public", Name: "v1", Relkind: "v",
		Definition: "SELECT 1",
	})
	if err != nil || r.Hash == "" {
		t.Fatalf("err=%v hash=%s", err, r.Hash)
	}
}

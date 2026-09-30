package analyzer

import "testing"

func TestIndexStructureIgnoresIndexName(t *testing.T) {
	a := IndexFact{
		Database: "db", Schema: "public", TableName: "orders", IndexName: "idx_a",
		Definition: "CREATE INDEX idx_a ON public.orders USING btree (customer_id)",
	}
	b := IndexFact{
		Database: "db", Schema: "public", TableName: "orders", IndexName: "idx_b",
		Definition: "CREATE INDEX idx_b ON public.orders USING btree (customer_id)",
	}
	if indexStructure(a) == "" || indexStructure(a) != indexStructure(b) {
		t.Fatalf("structures differ: %q vs %q", indexStructure(a), indexStructure(b))
	}
	c := IndexFact{
		Database: "db", Schema: "public", TableName: "orders", IndexName: "idx_c",
		Definition: "CREATE INDEX idx_c ON public.orders USING btree (order_id)",
	}
	if indexStructure(a) == indexStructure(c) {
		t.Fatal("different column definitions must not collide")
	}
}

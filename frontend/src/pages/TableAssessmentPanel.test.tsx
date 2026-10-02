import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { RelationshipGraph, TableSnapshot } from "../types";
import {
  RelationshipDiagram,
  visibleRelationshipEdges,
} from "./TableAssessmentPanel";

const root: TableSnapshot = {
  id: "snapshot",
  audit_run_id: "run",
  environment_id: "env",
  database_name: "db",
  schema_name: "public",
  table_name: "orders",
  owner_name: null,
  relkind: "r",
  total_size_bytes: 0,
  data_size_bytes: 0,
  index_size_bytes: 0,
  row_estimate: 0,
  column_count: 0,
  has_primary_key: true,
  collected_at: "2026-10-02T00:00:00Z",
};

const orders = { database: "db", schema: "public", table: "orders" };
const customers = { database: "db", schema: "public", table: "customers" };

describe("table relationship graph", () => {
  it("filters incoming and outgoing edges even with a cycle", () => {
    const graph: RelationshipGraph = {
      nodes: [orders, customers],
      truncated: false,
      edges: [
        {
          from: orders,
          to: customers,
          constraint_name: "orders_customer_fk",
          columns: ["customer_id"],
          referenced_columns: ["id"],
        },
        {
          from: customers,
          to: orders,
          constraint_name: "customers_order_fk",
          columns: ["last_order_id"],
          referenced_columns: ["id"],
        },
      ],
    };
    expect(
      visibleRelationshipEdges(graph, root, "incoming").map(
        (e) => e.constraint_name,
      ),
    ).toEqual(["customers_order_fk"]);
    expect(
      visibleRelationshipEdges(graph, root, "outgoing").map(
        (e) => e.constraint_name,
      ),
    ).toEqual(["orders_customer_fk"]);
    expect(visibleRelationshipEdges(graph, root, "all")).toHaveLength(2);
    const markup = renderToStaticMarkup(
      <RelationshipDiagram
        graph={graph}
        root={root}
        env="env"
        run="run"
        database="db"
        onNavigate={() => {}}
      />,
    );
    expect(markup).toContain('aria-label="Relacionamentos acessíveis"');
    expect(markup).toContain('aria-label="Zoom do diagrama"');
    expect(markup).toContain('aria-label="Filtrar schema relacionado"');
    expect(markup).toContain("orders_customer_fk");
  });

  it("shows an explicit empty state without foreign keys", () => {
    const graph: RelationshipGraph = {
      nodes: [orders],
      edges: [],
      truncated: false,
    };
    const markup = renderToStaticMarkup(
      <RelationshipDiagram
        graph={graph}
        root={root}
        env="env"
        run="run"
        database="db"
        onNavigate={() => {}}
      />,
    );
    expect(markup).toContain("Nenhuma FK nesta direção");
    expect(markup).toContain('aria-label="Relacionamentos acessíveis"');
  });

  it("keeps a large bounded graph navigable without rendering more than its edges", () => {
    const graph: RelationshipGraph = {
      nodes: [
        orders,
        ...Array.from({ length: 50 }, (_, i) => ({
          database: "db",
          schema: "archive",
          table: `related_${i}`,
        })),
      ],
      edges: Array.from({ length: 50 }, (_, i) => ({
        from: orders,
        to: { database: "db", schema: "archive", table: `related_${i}` },
        constraint_name: `fk_${i}`,
        columns: ["id"],
        referenced_columns: ["id"],
      })),
      truncated: true,
    };
    const markup = renderToStaticMarkup(
      <RelationshipDiagram
        graph={graph}
        root={root}
        env="env"
        run="run"
        database="db"
        onNavigate={() => {}}
      />,
    );
    expect(markup).toContain("Grafo limitado a 50 relacionamentos");
    expect(markup.match(/Abrir tabela/g)).toHaveLength(50);
  });
});

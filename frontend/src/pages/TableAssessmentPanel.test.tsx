import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type {
  RelationshipGraph,
  TableAssessment,
  TableSnapshot,
} from "../types";
import {
  AssessmentRunNotice,
  AssessmentScoreSummary,
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
    expect(markup).toContain("#/inventory?env=env");
    expect(markup).toContain("run=run");
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

describe("table assessment score", () => {
  const summary: TableAssessment["summary"] = {
    columns: 3,
    constraints: 1,
    indexes: 1,
    findings: 0,
    grants: 0,
    dependencies: 0,
    triggers: 0,
    rls_policies: 0,
    score: 80,
    score_status: "available",
    score_version: "structural-v1",
    score_confidence: 0.75,
    score_factors: [
      {
        code: "missing_primary_key",
        description: "Tabela sem chave primária",
        penalty: 20,
      },
    ],
    score_missing_collectors: [],
  };

  it("shows score, confidence and the contributing factor", () => {
    const markup = renderToStaticMarkup(
      <AssessmentScoreSummary summary={summary} />,
    );
    expect(markup).toContain("80/100");
    expect(markup).toContain("confiança 75%");
    expect(markup).toContain("Tabela sem chave primária");
  });

  it("does not turn missing collector coverage into a healthy score", () => {
    const markup = renderToStaticMarkup(
      <AssessmentScoreSummary
        summary={{
          ...summary,
          score: null,
          score_status: "insufficient_coverage",
          score_confidence: null,
          score_factors: [],
          score_missing_collectors: ["postgres.indexes"],
        }}
      />,
    );
    expect(markup).toContain("Score indisponível");
    expect(markup).toContain("postgres.indexes");
    expect(markup).not.toContain("80/100");
  });

  it("explains that a partial run does not prove absence of problems", () => {
    const markup = renderToStaticMarkup(<AssessmentRunNotice partial />);
    expect(markup).toContain("Run parcial");
    expect(renderToStaticMarkup(<AssessmentRunNotice partial={false} />)).toBe(
      "",
    );
  });
});

import { useEffect, useRef, useState } from "react";
import {
  type PageSize,
  PaginationControls,
} from "../components/ui/PaginationControls";
import { DetailGrid, DetailSection } from "../components/ui/Sheet";
import { formatError } from "../lib/errors";
import { formatBytes } from "../lib/format";
import { fetchAllPages } from "../lib/pagination";
import { api } from "../services/api";
import type {
  AuditBaseline,
  ColumnSnapshot,
  ColumnStatSnapshot,
  ConstraintSnapshot,
  DependencySnapshot,
  Finding,
  GrantSnapshot,
  IndexSnapshot,
  PagedResponse,
  PageMeta,
  RelationshipEdge,
  RelationshipGraph,
  RLSPolicySnapshot,
  ScopeScore,
  TableAssessment,
  TableHistoryPoint,
  TableSnapshot,
  TriggerSnapshot,
  WorkloadSnapshot,
} from "../types";
import { TableDetail } from "./InventoryDetails";

type Tab =
  | "overview"
  | "structure"
  | "relationships"
  | "performance"
  | "security"
  | "recommendations";
const tabs: Array<{ id: Tab; label: string }> = [
  { id: "overview", label: "Visão geral" },
  { id: "structure", label: "Estrutura" },
  { id: "relationships", label: "Relacionamentos" },
  { id: "performance", label: "Performance" },
  { id: "security", label: "Segurança" },
  { id: "recommendations", label: "Recomendações" },
];

function permalink(env: string, t: TableSnapshot): string {
  const q = new URLSearchParams({
    env,
    run: t.audit_run_id,
    database: t.database_name,
    schema: t.schema_name,
    table: t.table_name,
  });
  const base =
    typeof window === "undefined"
      ? "http://localhost/"
      : `${window.location.origin}${window.location.pathname}`;
  return `${base}#/inventory?${q}`;
}

function useCollection<T>(
  enabled: boolean,
  key: string,
  load: (offset: number, limit: number) => Promise<PagedResponse<T>>,
) {
  const loader = useRef(load);
  loader.current = load;
  const [offset, setOffset] = useState(0);
  const [size, setSize] = useState<PageSize>(20);
  const [items, setItems] = useState<T[]>([]);
  const [page, setPage] = useState<PageMeta | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  useEffect(() => {
    setOffset(0);
    setItems([]);
    setPage(null);
    setError(null);
  }, [key]);
  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    setLoading(true);
    const request =
      size === "all"
        ? fetchAllPages(loader.current).then((allItems) => ({
            items: allItems,
            page: {
              total: allItems.length,
              limit: allItems.length,
              offset: 0,
              has_more: false,
            },
          }))
        : loader.current(offset, size);
    request
      .then((result) => {
        if (!cancelled) {
          setItems(result.items);
          setPage(result.page);
          setError(null);
        }
      })
      .catch((cause: unknown) => {
        if (!cancelled) {
          setItems([]);
          setPage(null);
          setError(formatError(cause, "Não foi possível carregar esta seção"));
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [enabled, key, offset, size]);
  return { items, page, error, loading, offset, size, setOffset, setSize };
}

function CollectionState({
  data,
  children,
}: {
  data: ReturnType<typeof useCollection<unknown>>;
  children: React.ReactNode;
}) {
  if (data.loading)
    return (
      <p role="status" className="text-sm text-slate-400">
        Carregando…
      </p>
    );
  if (data.error)
    return (
      <p role="alert" className="text-sm text-red-300">
        {data.error}
      </p>
    );
  const page = data.page;
  return (
    <div className="space-y-2">
      {data.items.length === 0 ? (
        <p className="text-sm text-slate-400">Nenhum item neste run.</p>
      ) : (
        children
      )}
      {page ? (
        <PaginationControls
          total={page.total}
          offset={data.offset}
          size={data.size}
          onSizeChange={(next) => {
            data.setSize(next);
            data.setOffset(0);
          }}
          onOffsetChange={data.setOffset}
          label="Detalhe da tabela"
        />
      ) : null}
    </div>
  );
}

function nodeKey(schema: string, table: string) {
  return `${schema}.${table}`;
}

export function visibleRelationshipEdges(
  graph: RelationshipGraph,
  root: TableSnapshot,
  direction: "all" | "incoming" | "outgoing",
) {
  const key = nodeKey(root.schema_name, root.table_name);
  return graph.edges.filter(
    (edge) =>
      direction === "all" ||
      (direction === "incoming"
        ? nodeKey(edge.to.schema, edge.to.table) === key
        : nodeKey(edge.from.schema, edge.from.table) === key),
  );
}

export function AssessmentScoreSummary({
  summary,
}: {
  summary: TableAssessment["summary"];
}) {
  if (summary.score_status !== "available" || summary.score === null) {
    return (
      <p role="status" className="text-sm text-amber-300">
        Score indisponível: cobertura insuficiente neste run
        {summary.score_missing_collectors.length > 0
          ? ` (${summary.score_missing_collectors.join(", ")})`
          : ""}
        .
      </p>
    );
  }
  return (
    <div className="space-y-2 text-sm">
      <p>
        <strong>{summary.score}/100</strong> · {summary.score_version} ·
        confiança {Math.round((summary.score_confidence ?? 0) * 100)}%
      </p>
      <p className="text-xs text-slate-400">
        Base estrutural 100; os fatores abaixo reduzem a pontuação. Findings
        aparecem separadamente porque seu histórico ainda não é imutável.
      </p>
      {summary.score_factors.length === 0 ? (
        <p className="text-xs text-slate-400">
          Nenhuma penalidade estrutural identificada no snapshot.
        </p>
      ) : (
        <ul className="space-y-1" aria-label="Fatores do score">
          {summary.score_factors.map((factor) => (
            <li key={factor.code}>
              {factor.description}: −{factor.penalty}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

export function AssessmentRunNotice({ partial }: { partial: boolean }) {
  if (!partial) return null;
  return (
    <p className="rounded border border-amber-600 p-2 text-xs text-amber-300">
      Run parcial: a ausência de dados não confirma ausência de problemas.
    </p>
  );
}

export function RelationshipDiagram({
  graph,
  root,
  env,
  run,
  database,
  onNavigate,
}: {
  graph: RelationshipGraph;
  root: TableSnapshot;
  env: string;
  run: string;
  database: string;
  onNavigate: (schema: string, table: string) => void;
}) {
  const [direction, setDirection] = useState<"all" | "incoming" | "outgoing">(
    "all",
  );
  const [schemaFilter, setSchemaFilter] = useState("all");
  const [zoom, setZoom] = useState(1);
  const rootKey = nodeKey(root.schema_name, root.table_name);
  const schemas = [...new Set(graph.nodes.map((n) => n.schema))].sort();
  const edges = visibleRelationshipEdges(graph, root, direction).filter(
    (edge) =>
      schemaFilter === "all" ||
      (nodeKey(edge.from.schema, edge.from.table) === rootKey
        ? edge.to.schema
        : edge.from.schema) === schemaFilter,
  );
  const nodes = graph.nodes.filter(
    (n) =>
      nodeKey(n.schema, n.table) === rootKey ||
      edges.some(
        (e) =>
          nodeKey(e.from.schema, e.from.table) === nodeKey(n.schema, n.table) ||
          nodeKey(e.to.schema, e.to.table) === nodeKey(n.schema, n.table),
      ),
  );
  const positions = new Map(
    nodes.map((n, i) => [
      nodeKey(n.schema, n.table),
      i === 0
        ? { x: 300, y: 180 }
        : {
            x:
              300 +
              220 *
                Math.cos(
                  (2 * Math.PI * (i - 1)) / Math.max(1, nodes.length - 1),
                ),
            y:
              180 +
              130 *
                Math.sin(
                  (2 * Math.PI * (i - 1)) / Math.max(1, nodes.length - 1),
                ),
          },
    ]),
  );
  const link = (schema: string, table: string) =>
    permalink(env, {
      ...root,
      audit_run_id: run,
      database_name: database,
      schema_name: schema,
      table_name: table,
    });
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-3 text-xs">
        <label>
          Direção{" "}
          <select
            aria-label="Direção dos relacionamentos"
            value={direction}
            onChange={(e) => setDirection(e.target.value as typeof direction)}
            className="rounded bg-slate-800 p-1"
          >
            <option value="all">Todas</option>
            <option value="incoming">Entrada</option>
            <option value="outgoing">Saída</option>
          </select>
        </label>
        <label>
          Schema relacionado{" "}
          <select
            aria-label="Filtrar schema relacionado"
            value={schemaFilter}
            onChange={(e) => setSchemaFilter(e.target.value)}
            className="rounded bg-slate-800 p-1"
          >
            <option value="all">Todos</option>
            {schemas.map((schema) => (
              <option key={schema} value={schema}>
                {schema}
              </option>
            ))}
          </select>
        </label>
        <label>
          Zoom{" "}
          <input
            aria-label="Zoom do diagrama"
            type="range"
            min="0.7"
            max="1.8"
            step="0.1"
            value={zoom}
            onChange={(e) => setZoom(Number(e.target.value))}
          />
        </label>
      </div>
      {edges.length === 0 ? (
        <p className="text-sm text-slate-400">Nenhuma FK nesta direção.</p>
      ) : (
        <div
          className="overflow-auto rounded border border-slate-700"
          aria-hidden="true"
        >
          <svg
            viewBox="0 0 600 360"
            width={600 * zoom}
            height={360 * zoom}
            role="img"
          >
            <title>{`Diagrama ER focado em ${rootKey}`}</title>
            {edges.map((edge: RelationshipEdge) => {
              const from = positions.get(
                nodeKey(edge.from.schema, edge.from.table),
              );
              const to = positions.get(nodeKey(edge.to.schema, edge.to.table));
              return from && to ? (
                <line
                  key={edge.constraint_name}
                  x1={from.x}
                  y1={from.y}
                  x2={to.x}
                  y2={to.y}
                  stroke="#38bdf8"
                  strokeWidth="2"
                />
              ) : null;
            })}
            {nodes.map((n) => {
              const p = positions.get(nodeKey(n.schema, n.table));
              if (!p) return null;
              const isRoot = nodeKey(n.schema, n.table) === rootKey;
              return (
                <g key={nodeKey(n.schema, n.table)}>
                  <rect
                    x={p.x - 67}
                    y={p.y - 17}
                    width="134"
                    height="34"
                    rx="6"
                    fill={isRoot ? "#0e7490" : "#334155"}
                  />
                  <text
                    x={p.x}
                    y={p.y + 5}
                    textAnchor="middle"
                    fill="white"
                    fontSize="11"
                  >
                    {n.table.slice(0, 19)}
                  </text>
                </g>
              );
            })}
          </svg>
        </div>
      )}
      <p className="text-xs text-slate-400">
        Legenda: bloco azul = tabela selecionada; linha = chave estrangeira. Use
        a lista abaixo para navegação acessível.
      </p>
      <ul className="space-y-2 text-sm" aria-label="Relacionamentos acessíveis">
        {edges.map((e) => {
          const target =
            nodeKey(e.from.schema, e.from.table) === rootKey ? e.to : e.from;
          return (
            <li
              key={e.constraint_name}
              className="rounded border border-slate-700 p-2"
            >
              <span>
                {e.from.schema}.{e.from.table} ({e.columns.join(", ")}) →{" "}
                {e.to.schema}.{e.to.table} ({e.referenced_columns.join(", ")})
              </span>
              <span className="ml-2 text-slate-400">{e.constraint_name}</span>{" "}
              <a
                href={link(target.schema, target.table)}
                onClick={(event) => {
                  event.preventDefault();
                  onNavigate(target.schema, target.table);
                }}
                className="text-cyan-300 underline"
              >
                Abrir tabela
              </a>
            </li>
          );
        })}
      </ul>
      {graph.truncated ? (
        <p className="text-xs text-amber-300">
          Grafo limitado a 50 relacionamentos. Refine o escopo.
        </p>
      ) : null}
    </div>
  );
}

export function TableAssessmentPanel({
  t,
  env,
  history,
  columnStats,
  workload,
  onNavigate,
}: {
  t: TableSnapshot;
  env: string;
  history: TableHistoryPoint[];
  columnStats: ColumnStatSnapshot[];
  workload: WorkloadSnapshot[];
  onNavigate: (schema: string, table: string) => void;
}) {
  const [tab, setTab] = useState<Tab>("overview");
  const [assessment, setAssessment] = useState<TableAssessment | null>(null);
  const [graph, setGraph] = useState<RelationshipGraph | null>(null);
  const [baseline, setBaseline] = useState<AuditBaseline | null>(null);
  const [baselineToken, setBaselineToken] = useState("");
  const [scopeScore, setScopeScore] = useState<ScopeScore | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const key = `${env}/${t.audit_run_id}/${t.database_name}/${t.schema_name}/${t.table_name}`;
  const args = [
    env,
    t.audit_run_id,
    t.database_name,
    t.schema_name,
    t.table_name,
  ] as const;
  useEffect(() => {
    let cancelled = false;
    setTab("overview");
    setAssessment(null);
    setGraph(null);
    setLoading(true);
    api
      .tableAssessment(...args)
      .then((v) => {
        if (!cancelled) {
          setAssessment(v);
          setError(null);
        }
      })
      .catch((cause: unknown) => {
        if (!cancelled) setError(formatError(cause, "Assessment indisponível"));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // key is the canonical identity of this assessment.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);
  useEffect(() => {
    let active = true;
    void api
      .auditBaseline(env, t.database_name, t.schema_name, t.table_name)
      .then((value) => {
        if (active) setBaseline(value);
      })
      .catch(() => {
        if (active) setBaseline(null);
      });
    void api
      .scopeScore(
        env,
        t.audit_run_id,
        t.database_name,
        t.schema_name,
        t.table_name,
      )
      .then((value) => {
        if (active) setScopeScore(value);
      })
      .catch(() => {
        if (active) setScopeScore(null);
      });
    return () => {
      active = false;
    };
  }, [key]);

  const approveBaseline = async () => {
    if (
      !window.confirm(
        `Aprovar a execução ${t.audit_run_id} como baseline da tabela ${t.database_name}.${t.schema_name}.${t.table_name}?`,
      )
    )
      return;
    try {
      api.setReportToken(baselineToken);
      setBaseline(
        await api.selectAuditBaseline(
          env,
          t.audit_run_id,
          t.database_name,
          t.schema_name,
          t.table_name,
        ),
      );
      setError(null);
    } catch (cause: unknown) {
      setError(
        formatError(cause, "Não foi possível aprovar o baseline da tabela"),
      );
    }
  };
  useEffect(() => {
    if (tab !== "relationships" || !assessment) return;
    let cancelled = false;
    api
      .tableGraph(...args)
      .then((v) => {
        if (!cancelled) setGraph(v);
      })
      .catch((cause: unknown) => {
        if (!cancelled) setError(formatError(cause, "Grafo indisponível"));
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, tab, assessment]);
  const columns = useCollection<ColumnSnapshot>(
    assessment !== null && tab === "structure",
    key,
    (offset, limit) =>
      api.columns(env, {
        audit_run_id: t.audit_run_id,
        database: t.database_name,
        schema: t.schema_name,
        table: t.table_name,
        limit,
        offset,
      }),
  );
  const constraints = useCollection<ConstraintSnapshot>(
    assessment !== null && tab === "structure",
    key,
    (offset, limit) =>
      api.constraints(env, {
        audit_run_id: t.audit_run_id,
        database: t.database_name,
        schema: t.schema_name,
        table: t.table_name,
        limit,
        offset,
      }),
  );
  const indexes = useCollection<IndexSnapshot>(
    assessment !== null && tab === "structure",
    key,
    (offset, limit) =>
      api.indexes(env, {
        audit_run_id: t.audit_run_id,
        database: t.database_name,
        schema: t.schema_name,
        table: t.table_name,
        limit,
        offset,
      }),
  );
  const dependencies = useCollection<DependencySnapshot>(
    assessment !== null && tab === "relationships",
    key,
    (offset, limit) => api.tableDependencies(...args, limit, offset),
  );
  const grants = useCollection<GrantSnapshot>(
    assessment !== null && tab === "security",
    key,
    (offset, limit) => api.tableGrants(...args, limit, offset),
  );
  const triggers = useCollection<TriggerSnapshot>(
    assessment !== null && tab === "security",
    key,
    (offset, limit) => api.tableTriggers(...args, limit, offset),
  );
  const policies = useCollection<RLSPolicySnapshot>(
    assessment !== null && tab === "security",
    key,
    (offset, limit) => api.tableRLSPolicies(...args, limit, offset),
  );
  const findings = useCollection<Finding>(
    assessment !== null && tab === "recommendations",
    key,
    (offset, limit) => api.tableFindings(...args, limit, offset),
  );
  if (!loading && !assessment) {
    return (
      <p role="alert" className="text-sm text-red-300">
        {error ?? "Assessment não encontrado para este run."}
      </p>
    );
  }
  const table = assessment?.table ?? t;
  return (
    <div className="space-y-4">
      <a href={permalink(env, t)} className="text-xs text-cyan-300 underline">
        Link direto para esta tabela e execução
      </a>
      {loading ? (
        <p role="status" className="text-sm text-slate-400">
          Carregando assessment…
        </p>
      ) : null}
      {error ? (
        <p role="alert" className="text-sm text-red-300">
          {error}
        </p>
      ) : null}
      <AssessmentRunNotice partial={assessment?.run.partial ?? false} />
      <div className="rounded border border-slate-700 bg-slate-900/60 p-3 text-xs text-slate-300">
        <p>
          Baseline desta tabela:{" "}
          {baseline ? `${baseline.audit_run_id.slice(0, 8)}…` : "não definido"}
        </p>
        <p>
          Score de escopo:{" "}
          {scopeScore?.score == null
            ? "indisponível por cobertura"
            : `${scopeScore.score}/100`}{" "}
          · confiança {Math.round((scopeScore?.confidence ?? 0) * 100)}%
        </p>
        {scopeScore?.categories.map((category) => (
          <p key={category.category}>
            {category.category}: {category.score}/100 · {category.findings}{" "}
            findings · penalidade {category.penalty}
          </p>
        ))}
        {assessment?.run.status === "success" ? (
          <div className="mt-2 flex flex-wrap gap-2">
            {!api.hasSession() && (
              <input
                aria-label="Token para baseline da tabela"
                type="password"
                autoComplete="off"
                placeholder="Token de operação protegida"
                value={baselineToken}
                onChange={(event) => setBaselineToken(event.target.value)}
                className="rounded border border-slate-600 bg-slate-900 px-2 py-1 text-slate-100"
              />
            )}
            <button
              type="button"
              disabled={!api.hasRole("operator") && !baselineToken}
              className="rounded border border-slate-500 px-2 py-1 text-cyan-300 disabled:opacity-50"
              onClick={() => void approveBaseline()}
            >
              Aprovar esta execução como baseline da tabela
            </button>
          </div>
        ) : null}
      </div>
      <div
        role="tablist"
        aria-label="Seções do assessment"
        className="flex flex-wrap gap-1 border-b border-slate-700 pb-2"
      >
        {tabs.map((item) => (
          <button
            key={item.id}
            id={`assessment-tab-${item.id}`}
            aria-controls={`assessment-panel-${item.id}`}
            type="button"
            role="tab"
            aria-selected={tab === item.id}
            onClick={() => setTab(item.id)}
            className={`rounded px-2 py-1 text-xs ${tab === item.id ? "bg-cyan-800 text-white" : "text-slate-300 hover:bg-slate-800"}`}
          >
            {item.label}
          </button>
        ))}
      </div>
      <div
        role="tabpanel"
        id={`assessment-panel-${tab}`}
        aria-labelledby={`assessment-tab-${tab}`}
      >
        {tab === "overview" ? (
          <>
            <DetailSection title="Tabela">
              <DetailGrid
                items={[
                  {
                    label: "Database / schema",
                    value: `${t.database_name} / ${t.schema_name}`,
                  },
                  { label: "Tabela", value: t.table_name },
                  { label: "Tipo", value: t.relation_class || t.relkind },
                  {
                    label: "Linhas estimadas",
                    value: t.row_estimate.toLocaleString("pt-BR"),
                  },
                  { label: "Storage", value: formatBytes(t.total_size_bytes) },
                  {
                    label: "Chave primária",
                    value: t.has_primary_key ? "Sim" : "Não",
                  },
                  {
                    label: "Comentário",
                    value: assessment?.table.comment ?? "—",
                  },
                  {
                    label: "Tablespace",
                    value: assessment?.table.tablespace_name ?? "—",
                  },
                  {
                    label: "Persistência",
                    value: assessment?.table.persistence ?? "—",
                  },
                  {
                    label: "Storage parameters",
                    value:
                      assessment?.table.storage_parameters.join(", ") || "—",
                  },
                  { label: "Run", value: t.audit_run_id },
                ]}
              />
            </DetailSection>
            <DetailSection title="Resumo do run">
              <DetailGrid
                items={[
                  {
                    label: "Colunas",
                    value: assessment?.summary.columns ?? "—",
                  },
                  {
                    label: "Constraints",
                    value: assessment?.summary.constraints ?? "—",
                  },
                  {
                    label: "Índices",
                    value: assessment?.summary.indexes ?? "—",
                  },
                  {
                    label: "Findings",
                    value: assessment?.summary.findings ?? "—",
                  },
                  { label: "Grants", value: assessment?.summary.grants ?? "—" },
                  {
                    label: "Dependências",
                    value: assessment?.summary.dependencies ?? "—",
                  },
                ]}
              />
            </DetailSection>
            {assessment ? (
              <DetailSection title="Score estrutural da tabela">
                <AssessmentScoreSummary summary={assessment.summary} />
              </DetailSection>
            ) : null}
            <p className="text-xs text-slate-400">
              Contrato v{assessment?.version ?? 1}; as coleções relacionadas são
              paginadas e sempre usam este run.
            </p>
          </>
        ) : null}
        {tab === "structure" ? (
          <div className="space-y-5">
            <DetailSection title="Colunas">
              <CollectionState data={columns}>
                <ul className="space-y-1 text-sm">
                  {columns.items.map((c) => (
                    <li
                      key={c.id}
                      className="rounded border border-slate-700 p-2"
                    >
                      <strong>{c.column_name}</strong> · {c.data_type} ·{" "}
                      {c.is_nullable ? "nullable" : "not null"}
                      {c.comment ? ` · ${c.comment}` : ""}
                    </li>
                  ))}
                </ul>
              </CollectionState>
            </DetailSection>
            <DetailSection title="Constraints">
              <CollectionState data={constraints}>
                <ul className="space-y-1 text-sm">
                  {constraints.items.map((c) => (
                    <li
                      key={c.id}
                      className="rounded border border-slate-700 p-2"
                    >
                      {c.constraint_name} · {c.constraint_type} ·{" "}
                      {c.is_validated ? "validada" : "não validada"}
                      {c.referenced_table_name
                        ? ` → ${c.referenced_schema_name}.${c.referenced_table_name}`
                        : ""}
                    </li>
                  ))}
                </ul>
              </CollectionState>
            </DetailSection>
            <DetailSection title="Índices">
              <CollectionState data={indexes}>
                <ul className="space-y-1 text-sm">
                  {indexes.items.map((i) => (
                    <li
                      key={i.id}
                      className="rounded border border-slate-700 p-2"
                    >
                      {i.index_name} · {i.access_method ?? "índice"} ·{" "}
                      {formatBytes(i.size_bytes)}
                    </li>
                  ))}
                </ul>
              </CollectionState>
            </DetailSection>
          </div>
        ) : null}
        {tab === "relationships" ? (
          <div className="space-y-5">
            <DetailSection title="Foreign keys">
              {graph ? (
                <RelationshipDiagram
                  graph={graph}
                  root={t}
                  env={env}
                  run={t.audit_run_id}
                  database={t.database_name}
                  onNavigate={onNavigate}
                />
              ) : (
                <p role="status" className="text-sm text-slate-400">
                  Carregando grafo…
                </p>
              )}
            </DetailSection>
            <DetailSection title="Dependências de views e funções">
              <CollectionState data={dependencies}>
                <ul className="space-y-1 text-sm">
                  {dependencies.items.map((d) => (
                    <li
                      key={`${d.source_kind}:${d.source_schema}.${d.source_name}->${d.target_schema}.${d.target_name}`}
                      className="rounded border border-slate-700 p-2"
                    >
                      {d.source_kind} {d.source_schema}.{d.source_name} →{" "}
                      {d.target_kind} {d.target_schema}.{d.target_name}
                    </li>
                  ))}
                </ul>
              </CollectionState>
            </DetailSection>
          </div>
        ) : null}
        {tab === "performance" ? (
          <TableDetail
            t={table}
            history={history}
            columnStats={columnStats}
            workload={workload}
          />
        ) : null}
        {tab === "security" ? (
          <div className="space-y-5">
            <DetailSection title="Row Level Security">
              <p className="text-sm">
                RLS{" "}
                {assessment?.table.rls_enabled ? "habilitado" : "desabilitado"}
                {";"}
                FORCE{" "}
                {assessment?.table.rls_forced ? "habilitado" : "desabilitado"}.
              </p>
            </DetailSection>
            <DetailSection title="Grants efetivos">
              <CollectionState data={grants}>
                <ul className="space-y-1 text-sm">
                  {grants.items.map((g) => (
                    <li key={g.grantee}>
                      {g.grantee}: {g.privileges.join(", ")}
                    </li>
                  ))}
                </ul>
              </CollectionState>
            </DetailSection>
            <DetailSection title="Policies">
              <CollectionState data={policies}>
                <ul className="space-y-1 text-sm">
                  {policies.items.map((p) => (
                    <li key={p.name}>
                      {p.name} · {p.command ?? "ALL"} · {p.roles.join(", ")}
                    </li>
                  ))}
                </ul>
              </CollectionState>
            </DetailSection>
            <DetailSection title="Triggers">
              <CollectionState data={triggers}>
                <ul className="space-y-1 text-sm">
                  {triggers.items.map((tr) => (
                    <li key={tr.name}>
                      {tr.name} · {tr.enabled} · {tr.timing ?? "—"}{" "}
                      {tr.events ?? ""}
                    </li>
                  ))}
                </ul>
              </CollectionState>
            </DetailSection>
          </div>
        ) : null}
        {tab === "recommendations" ? (
          <DetailSection title="Findings do run">
            <CollectionState data={findings}>
              <ul className="space-y-2 text-sm">
                {findings.items.map((f) => (
                  <li
                    key={f.id}
                    className="rounded border border-slate-700 p-3"
                  >
                    <strong>{f.title}</strong>
                    <span className="ml-2 text-xs text-slate-400">
                      {f.severity} · {f.rule_id} v{f.rule_version} · confiança{" "}
                      {Math.round((f.confidence ?? 0) * 100)}%
                    </span>
                    <p>{f.summary}</p>
                    <p className="text-slate-300">{f.recommendation}</p>
                    {f.validation ? (
                      <p className="text-xs text-slate-400">
                        Validação: {f.validation}
                      </p>
                    ) : null}
                    {f.evidence ? (
                      <details className="mt-2 text-xs text-slate-300">
                        <summary>Evidência</summary>
                        <pre className="overflow-auto whitespace-pre-wrap">
                          {JSON.stringify(f.evidence, null, 2)}
                        </pre>
                      </details>
                    ) : null}
                  </li>
                ))}
              </ul>
            </CollectionState>
          </DetailSection>
        ) : null}
      </div>
    </div>
  );
}

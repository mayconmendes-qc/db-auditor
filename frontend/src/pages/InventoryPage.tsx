import { useCallback, useEffect, useMemo, useState } from "react";
import { PageHeader } from "../components/PageHeader";
import {
  Button,
  Card,
  EmptyState,
  ErrorBanner,
  Input,
  Select,
  Sheet,
  Skeleton,
  Table,
} from "../components/ui";
import { useApp } from "../context/AppContext";
import { formatError } from "../lib/errors";
import { formatBytes, matchesSearch } from "../lib/format";
import {
  relationClassBadgeClass,
  relationClassLabel,
} from "../lib/relationClass";
import { nextSort, type SortState, sortBy } from "../lib/sort";
import { api } from "../services/api";
import type {
  ColumnSnapshot,
  DatabaseSnapshot,
  FunctionSnapshot,
  HypertableSnapshot,
  IndexSnapshot,
  InventoryObjectKind,
  PageMeta,
  SchemaSnapshot,
  SnapshotCompleteness,
  TableSnapshot,
  ViewSnapshot,
} from "../types";
import {
  FunctionDetail,
  HypertableDetail,
  IndexDetail,
  TableDetail,
  ViewDetail,
} from "./InventoryDetails";

const PAGE_SIZE = 50;

const KINDS: InventoryObjectKind[] = [
  "tables",
  "hypertables",
  "indexes",
  "views",
  "functions",
  "caggs",
];

const KIND_LABELS: Record<InventoryObjectKind, string> = {
  tables: "Tabelas",
  hypertables: "Hypertables",
  indexes: "Índices",
  views: "Views",
  functions: "Funções",
  caggs: "CAGGs",
};

function rowClass(selected: boolean): string {
  return `cursor-pointer border-t border-slate-800/80 transition-colors ${
    selected ? "bg-slate-800/70" : "hover:bg-slate-900/50"
  }`;
}

function fmtNum(n: number | null | undefined): string {
  if (n == null || !Number.isFinite(n)) return "—";
  return n.toLocaleString("pt-BR");
}

function boolLabel(v: boolean | null | undefined): string {
  if (v == null) return "—";
  return v ? "Sim" : "Não";
}

export function InventoryPage() {
  const { environmentId, environments, setSection } = useApp();
  const envId = environmentId;

  const [error, setError] = useState<string | null>(null);
  const [databases, setDatabases] = useState<DatabaseSnapshot[] | null>(null);
  const [schemas, setSchemas] = useState<SchemaSnapshot[] | null>(null);
  const [selectedDb, setSelectedDb] = useState<string | null>(null);
  const [selectedSchema, setSelectedSchema] = useState<string | null>(null);
  const [kind, setKind] = useState<InventoryObjectKind>("tables");
  const [q, setQ] = useState("");
  const [offset, setOffset] = useState(0);
  const [page, setPage] = useState<PageMeta | null>(null);
  const [loading, setLoading] = useState(false);
  const [listError, setListError] = useState<string | null>(null);
  const [tables, setTables] = useState<TableSnapshot[]>([]);
  const [indexes, setIndexes] = useState<IndexSnapshot[]>([]);
  const [views, setViews] = useState<ViewSnapshot[]>([]);
  const [functions, setFunctions] = useState<FunctionSnapshot[]>([]);
  const [hypertables, setHypertables] = useState<HypertableSnapshot[]>([]);
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);
  const [sort, setSort] = useState<SortState | null>(null);
  const [snapshotStatus, setSnapshotStatus] =
    useState<SnapshotCompleteness | null>(null);
  const [detailColumns, setDetailColumns] = useState<ColumnSnapshot[]>([]);

  useEffect(() => {
    if (!envId) {
      setDatabases([]);
      setSchemas([]);
      setSelectedDb(null);
      setSelectedSchema(null);
      setOffset(0);
      setError(null);
      setSnapshotStatus(null);
      return;
    }
    let cancelled = false;
    setDatabases(null);
    setSchemas(null);
    setSelectedDb(null);
    setSelectedSchema(null);
    setOffset(0);
    setError(null);
    Promise.all([
      api.databases(envId),
      api.schemas(envId),
      api.snapshotStatus(envId).catch(() => null),
    ])
      .then(([dbRes, scRes, status]) => {
        if (!cancelled) {
          setDatabases(dbRes.items);
          setSchemas(scRes.items);
          setSnapshotStatus(status);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setListError(formatError(err, "Falha ao carregar topologia"));
          setDatabases([]);
          setSchemas([]);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [envId]);

  const schemasForDb = useMemo(() => {
    if (!schemas) return [];
    if (!selectedDb) return schemas;
    return schemas.filter((s) => s.database_name === selectedDb);
  }, [schemas, selectedDb]);

  const loadObjects = useCallback(() => {
    if (!envId) {
      setTables([]);
      setIndexes([]);
      setViews([]);
      setFunctions([]);
      setHypertables([]);
      setPage(null);
      setLoading(false);
      return () => {};
    }
    let cancelled = false;
    setLoading(true);
    setListError(null);
    setSelectedKey(null);
    setSheetOpen(false);
    setDetailColumns([]);
    const params = {
      limit: PAGE_SIZE,
      offset,
      q: q || undefined,
      database: selectedDb || undefined,
      schema: selectedSchema || undefined,
    };
    const fail = (msg: string) => {
      if (!cancelled) {
        setListError(msg);
        setLoading(false);
      }
    };
    const ok = () => {
      if (!cancelled) setLoading(false);
    };

    if (kind === "tables") {
      api
        .tables(envId, params)
        .then((res) => {
          if (!cancelled) {
            setTables(res.items);
            setPage(res.page);
          }
        })
        .catch((e: unknown) => fail(formatError(e, "Falha ao listar tables")))
        .finally(ok);
    } else if (kind === "indexes") {
      api
        .indexes(envId, params)
        .then((res) => {
          if (!cancelled) {
            setIndexes(res.items);
            setPage(res.page);
          }
        })
        .catch((e: unknown) => fail(formatError(e, "Falha ao listar indexes")))
        .finally(ok);
    } else if (kind === "views") {
      api
        .views(envId, params)
        .then((res) => {
          if (!cancelled) {
            setViews(res.items);
            setPage(res.page);
          }
        })
        .catch((e: unknown) => fail(formatError(e, "Falha ao listar views")))
        .finally(ok);
    } else if (kind === "functions") {
      api
        .functions(envId, params)
        .then((res) => {
          if (!cancelled) {
            setFunctions(res.items);
            setPage(res.page);
          }
        })
        .catch((e: unknown) =>
          fail(formatError(e, "Falha ao listar functions")),
        )
        .finally(ok);
    } else if (kind === "hypertables") {
      api
        .hypertables(envId)
        .then((res) => {
          if (!cancelled) {
            let items = res.items;
            if (selectedDb)
              items = items.filter((h) => h.database_name === selectedDb);
            if (selectedSchema)
              items = items.filter((h) => h.schema_name === selectedSchema);
            if (q)
              items = items.filter(
                (h) =>
                  matchesSearch(h.hypertable_name, q) ||
                  matchesSearch(h.schema_name, q),
              );
            const total = items.length;
            setHypertables(items.slice(offset, offset + PAGE_SIZE));
            setPage({
              limit: PAGE_SIZE,
              offset,
              total,
              has_more: offset + PAGE_SIZE < total,
            });
          }
        })
        .catch((e: unknown) =>
          fail(formatError(e, "Falha ao listar hypertables")),
        )
        .finally(ok);
    } else {
      api
        .continuousAggregates(envId)
        .then((res) => {
          if (!cancelled) {
            let items = res.items;
            if (selectedDb)
              items = items.filter((c) => c.database_name === selectedDb);
            if (selectedSchema)
              items = items.filter((c) => c.schema_name === selectedSchema);
            if (q) items = items.filter((c) => matchesSearch(c.view_name, q));
            const total = items.length;
            setPage({
              limit: PAGE_SIZE,
              offset,
              total,
              has_more: offset + PAGE_SIZE < total,
            });
            setViews(
              items.slice(offset, offset + PAGE_SIZE).map((c) => ({
                id: c.id,
                database_name: c.database_name,
                schema_name: c.schema_name,
                view_name: c.view_name,
                owner_name: c.owner_name,
                relkind: "cagg",
                size_bytes: 0,
                collected_at: c.collected_at,
              })),
            );
          }
        })
        .catch((e: unknown) => fail(formatError(e, "Falha ao listar CAGGs")))
        .finally(ok);
    }
    return () => {
      cancelled = true;
    };
  }, [envId, kind, offset, q, selectedDb, selectedSchema]);

  useEffect(() => loadObjects(), [loadObjects]);
  useEffect(() => setSort(null), [kind]);

  const sortedTables = useMemo(
    () =>
      sortBy(tables, sort, {
        database: (t) => t.database_name,
        schema: (t) => t.schema_name,
        name: (t) => t.table_name,
        type: (t) => t.relation_class ?? t.relkind ?? "",
        cols: (t) => t.column_count,
        rows: (t) => t.row_estimate,
        size: (t) => t.total_size_bytes,
        owner: (t) => t.owner_name ?? "",
        pk: (t) => t.has_primary_key,
      }),
    [tables, sort],
  );
  const sortedIndexes = useMemo(
    () =>
      sortBy(indexes, sort, {
        schema: (i) => i.schema_name,
        name: (i) => i.index_name,
        table: (i) => i.table_name,
        method: (i) => i.access_method ?? "",
        size: (i) => i.size_bytes,
        scans: (i) => i.idx_scan,
        unique: (i) => i.is_unique,
      }),
    [indexes, sort],
  );
  const sortedViews = useMemo(
    () =>
      sortBy(views, sort, {
        schema: (v) => v.schema_name,
        name: (v) => v.view_name,
        owner: (v) => v.owner_name ?? "",
        size: (v) => v.size_bytes,
        kind: (v) => v.relkind,
      }),
    [views, sort],
  );
  const sortedFunctions = useMemo(
    () =>
      sortBy(functions, sort, {
        schema: (f) => f.schema_name,
        name: (f) => f.function_name,
        kind: (f) => f.kind,
        lang: (f) => f.language_name ?? "",
        secdef: (f) => f.is_security_definer,
      }),
    [functions, sort],
  );
  const sortedHypertables = useMemo(
    () =>
      sortBy(hypertables, sort, {
        schema: (h) => h.schema_name,
        name: (h) => h.hypertable_name,
        size: (h) => h.total_size_bytes,
        chunks: (h) => h.num_chunks,
        compressed: (h) => h.compression_enabled,
      }),
    [hypertables, sort],
  );

  const openRow = (key: string) => {
    setSelectedKey(key);
    setSheetOpen(true);
    setDetailColumns([]);
  };

  const openTable = (t: TableSnapshot) => {
    const key = `table:${t.database_name}.${t.schema_name}.${t.table_name}`;
    openRow(key);
    if (!envId) return;
    api
      .columns(envId, {
        database: t.database_name,
        schema: t.schema_name,
        table: t.table_name,
        limit: 500,
      })
      .then((res) => setDetailColumns(res.items))
      .catch(() => setDetailColumns([]));
  };

  const selectedTable = sortedTables.find(
    (t) =>
      `table:${t.database_name}.${t.schema_name}.${t.table_name}` ===
      selectedKey,
  );
  const selectedIndex = sortedIndexes.find(
    (i) =>
      `index:${i.database_name}.${i.schema_name}.${i.index_name}` ===
      selectedKey,
  );
  const selectedView = sortedViews.find(
    (v) =>
      `view:${v.database_name}.${v.schema_name}.${v.view_name}` === selectedKey,
  );
  const selectedFn = sortedFunctions.find(
    (f) =>
      `fn:${f.database_name}.${f.schema_name}.${f.function_name}` ===
      selectedKey,
  );
  const selectedHt = sortedHypertables.find(
    (h) =>
      `ht:${h.database_name}.${h.schema_name}.${h.hypertable_name}` ===
      selectedKey,
  );

  const envName =
    environments.find((e) => e.id === envId)?.name ??
    (envId ? envId.slice(0, 8) : null);

  const pagination = page ? (
    <div className="mt-3 flex items-center justify-between gap-2 text-xs text-slate-400">
      <span>
        {page.total.toLocaleString("pt-BR")} objeto(s) · offset {page.offset}
      </span>
      <div className="flex gap-2">
        <Button
          variant="ghost"
          disabled={offset <= 0}
          onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
        >
          Anterior
        </Button>
        <Button
          variant="ghost"
          disabled={!page.has_more}
          onClick={() => setOffset(offset + PAGE_SIZE)}
        >
          Próxima
        </Button>
      </div>
    </div>
  ) : null;

  return (
    <>
      <PageHeader
        eyebrow="INVENTÁRIO"
        title="Explorer de inventário"
        description={
          <>
            Ambiente → database → schema → objeto. Clique em uma linha para
            abrir o painel de detalhes.
          </>
        }
        actions={
          envName ? (
            <span className="rounded-md border border-slate-700 bg-slate-900 px-3 py-1.5 text-xs text-slate-300">
              {envName}
            </span>
          ) : null
        }
      />

      {!envId ? (
        <div className="mt-8">
          <EmptyState
            title="Selecione um ambiente"
            description="Escolha um ambiente na barra lateral para explorar o inventário coletado."
            action={
              <Button onClick={() => setSection("Ambientes")}>
                Ir para Ambientes
              </Button>
            }
          />
        </div>
      ) : (
        <div className="mt-6 flex min-h-0 flex-col gap-4">
          {error ? (
            <ErrorBanner message={error} onRetry={() => setError(null)} />
          ) : null}

          {snapshotStatus ? (
            <div
              className={
                snapshotStatus.completeness === "complete"
                  ? "rounded-md border border-emerald-800/60 bg-emerald-950/40 px-3 py-2 text-sm text-emerald-200"
                  : snapshotStatus.completeness === "partial"
                    ? "rounded-md border border-amber-800/60 bg-amber-950/40 px-3 py-2 text-sm text-amber-200"
                    : "rounded-md border border-slate-700 bg-slate-900/60 px-3 py-2 text-sm text-slate-300"
              }
            >
              <div className="font-medium">
                Snapshot{" "}
                {snapshotStatus.completeness === "complete"
                  ? "completo"
                  : snapshotStatus.completeness === "partial"
                    ? "parcial"
                    : snapshotStatus.completeness === "empty"
                      ? "vazio"
                      : "desconhecido"}
              </div>
              <div className="mt-0.5 text-xs opacity-90">
                Run {snapshotStatus.audit_run_id.slice(0, 8)}… · status{" "}
                {snapshotStatus.run_status}
                {snapshotStatus.analysis_status
                  ? ` · análise ${snapshotStatus.analysis_status}`
                  : ""}
              </div>
            </div>
          ) : null}

          {databases === null ? (
            <Skeleton className="h-[28rem] w-full" />
          ) : databases.length === 0 ? (
            <EmptyState
              title="Sem snapshot de inventário"
              description="Rode uma auditoria neste ambiente para popular databases/schemas/objetos."
              action={
                <Button onClick={() => setSection("Execuções")}>
                  Ir para Execuções
                </Button>
              }
            />
          ) : (
            <>
              <Card title="Filtros">
                <div className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                  <Select
                    label="Database"
                    value={selectedDb ?? ""}
                    onChange={(e) => {
                      setSelectedDb(e.target.value || null);
                      setSelectedSchema(null);
                      setOffset(0);
                    }}
                    options={[
                      { value: "", label: "(todos)" },
                      ...databases.map((db) => ({
                        value: db.database_name,
                        label: db.database_name,
                      })),
                    ]}
                  />
                  <Select
                    label="Schema"
                    value={selectedSchema ?? ""}
                    onChange={(e) => {
                      setSelectedSchema(e.target.value || null);
                      setOffset(0);
                    }}
                    options={[
                      { value: "", label: "(todos)" },
                      ...schemasForDb.map((sc) => ({
                        value: sc.schema_name,
                        label: sc.schema_name,
                      })),
                    ]}
                  />
                  <Input
                    label="Buscar"
                    placeholder="Buscar nome…"
                    value={q}
                    onChange={(e) => {
                      setQ(e.target.value);
                      setOffset(0);
                    }}
                  />
                </div>
                <div className="mt-3 flex flex-wrap gap-2">
                  {KINDS.map((k) => (
                    <button
                      key={k}
                      type="button"
                      onClick={() => {
                        setKind(k);
                        setOffset(0);
                        setSelectedKey(null);
                        setSheetOpen(false);
                      }}
                      className={`rounded px-2.5 py-1 text-xs ${
                        kind === k
                          ? "bg-emerald-800 text-white"
                          : "bg-slate-800/80 text-slate-300 hover:bg-slate-800"
                      }`}
                    >
                      {KIND_LABELS[k]}
                    </button>
                  ))}
                </div>
                {listError ? (
                  <div className="mt-3">
                    <ErrorBanner
                      message={listError}
                      onRetry={() => loadObjects()}
                    />
                  </div>
                ) : null}
              </Card>

              <Card title={KIND_LABELS[kind]}>
                {loading ? (
                  <Skeleton className="h-64 w-full" />
                ) : kind === "tables" ? (
                  <>
                    <Table
                      dense
                      sortKey={sort?.key}
                      sortDir={sort?.dir}
                      onSort={(id) => setSort((s) => nextSort(s, id))}
                      headers={[
                        { id: "database", label: "Database", sortable: true },
                        { id: "schema", label: "Schema", sortable: true },
                        { id: "name", label: "Tabela", sortable: true },
                        { id: "type", label: "Tipo", sortable: true },
                        { id: "cols", label: "Cols", sortable: true },
                        { id: "rows", label: "Linhas", sortable: true },
                        { id: "size", label: "Tamanho", sortable: true },
                        { id: "pk", label: "PK", sortable: true },
                      ]}
                    >
                      {sortedTables.map((t) => {
                        const key = `table:${t.database_name}.${t.schema_name}.${t.table_name}`;
                        return (
                          <tr
                            key={t.id}
                            className={rowClass(selectedKey === key)}
                            onClick={() => openTable(t)}
                          >
                            <td className="px-3 py-2">{t.database_name}</td>
                            <td className="px-3 py-2">{t.schema_name}</td>
                            <td className="px-3 py-2 font-medium text-slate-100">
                              {t.table_name}
                            </td>
                            <td className="px-3 py-2">
                              <span
                                className={`inline-flex rounded border px-1.5 py-0.5 text-[11px] font-medium ${relationClassBadgeClass(
                                  t.relation_class,
                                )}`}
                              >
                                {relationClassLabel(
                                  t.relation_class,
                                  t.relkind,
                                )}
                              </span>
                            </td>
                            <td className="px-3 py-2">
                              {fmtNum(t.column_count)}
                            </td>
                            <td className="px-3 py-2">
                              {fmtNum(t.row_estimate)}
                            </td>
                            <td className="px-3 py-2">
                              {formatBytes(t.total_size_bytes)}
                            </td>
                            <td className="px-3 py-2">
                              {boolLabel(t.has_primary_key)}
                            </td>
                          </tr>
                        );
                      })}
                    </Table>
                    {pagination}
                  </>
                ) : kind === "indexes" ? (
                  <>
                    <Table
                      dense
                      sortKey={sort?.key}
                      sortDir={sort?.dir}
                      onSort={(id) => setSort((s) => nextSort(s, id))}
                      headers={[
                        { id: "schema", label: "Schema", sortable: true },
                        { id: "name", label: "Índice", sortable: true },
                        { id: "table", label: "Tabela", sortable: true },
                        { id: "method", label: "Método", sortable: true },
                        { id: "size", label: "Tamanho", sortable: true },
                        { id: "scans", label: "Scans", sortable: true },
                        { id: "unique", label: "Unique", sortable: true },
                      ]}
                    >
                      {sortedIndexes.map((i) => {
                        const key = `index:${i.database_name}.${i.schema_name}.${i.index_name}`;
                        return (
                          <tr
                            key={i.id}
                            className={rowClass(selectedKey === key)}
                            onClick={() => openRow(key)}
                          >
                            <td className="px-3 py-2">{i.schema_name}</td>
                            <td className="px-3 py-2 font-medium text-slate-100">
                              {i.index_name}
                            </td>
                            <td className="px-3 py-2">{i.table_name}</td>
                            <td className="px-3 py-2">
                              {i.access_method ?? "—"}
                            </td>
                            <td className="px-3 py-2">
                              {formatBytes(i.size_bytes)}
                            </td>
                            <td className="px-3 py-2">{fmtNum(i.idx_scan)}</td>
                            <td className="px-3 py-2">
                              {boolLabel(i.is_unique)}
                            </td>
                          </tr>
                        );
                      })}
                    </Table>
                    {pagination}
                  </>
                ) : kind === "views" || kind === "caggs" ? (
                  <>
                    <Table
                      dense
                      sortKey={sort?.key}
                      sortDir={sort?.dir}
                      onSort={(id) => setSort((s) => nextSort(s, id))}
                      headers={[
                        { id: "schema", label: "Schema", sortable: true },
                        { id: "name", label: "Nome", sortable: true },
                        { id: "owner", label: "Owner", sortable: true },
                        { id: "size", label: "Tamanho", sortable: true },
                        { id: "kind", label: "Kind", sortable: true },
                      ]}
                    >
                      {sortedViews.map((v) => {
                        const key = `view:${v.database_name}.${v.schema_name}.${v.view_name}`;
                        return (
                          <tr
                            key={v.id}
                            className={rowClass(selectedKey === key)}
                            onClick={() => openRow(key)}
                          >
                            <td className="px-3 py-2">{v.schema_name}</td>
                            <td className="px-3 py-2 font-medium text-slate-100">
                              {v.view_name}
                            </td>
                            <td className="px-3 py-2">{v.owner_name ?? "—"}</td>
                            <td className="px-3 py-2">
                              {formatBytes(v.size_bytes)}
                            </td>
                            <td className="px-3 py-2">{v.relkind}</td>
                          </tr>
                        );
                      })}
                    </Table>
                    {pagination}
                  </>
                ) : kind === "functions" ? (
                  <>
                    <Table
                      dense
                      sortKey={sort?.key}
                      sortDir={sort?.dir}
                      onSort={(id) => setSort((s) => nextSort(s, id))}
                      headers={[
                        { id: "schema", label: "Schema", sortable: true },
                        { id: "name", label: "Função", sortable: true },
                        { id: "kind", label: "Kind", sortable: true },
                        { id: "lang", label: "Lang", sortable: true },
                        { id: "secdef", label: "SecDef", sortable: true },
                      ]}
                    >
                      {sortedFunctions.map((f) => {
                        const key = `fn:${f.database_name}.${f.schema_name}.${f.function_name}`;
                        return (
                          <tr
                            key={f.id}
                            className={rowClass(selectedKey === key)}
                            onClick={() => openRow(key)}
                          >
                            <td className="px-3 py-2">{f.schema_name}</td>
                            <td className="px-3 py-2 font-medium text-slate-100">
                              {f.function_name}
                            </td>
                            <td className="px-3 py-2">{f.kind ?? "—"}</td>
                            <td className="px-3 py-2">
                              {f.language_name ?? "—"}
                            </td>
                            <td className="px-3 py-2">
                              {boolLabel(f.is_security_definer)}
                            </td>
                          </tr>
                        );
                      })}
                    </Table>
                    {pagination}
                  </>
                ) : (
                  <>
                    <Table
                      dense
                      sortKey={sort?.key}
                      sortDir={sort?.dir}
                      onSort={(id) => setSort((s) => nextSort(s, id))}
                      headers={[
                        { id: "schema", label: "Schema", sortable: true },
                        { id: "name", label: "Hypertable", sortable: true },
                        { id: "size", label: "Tamanho", sortable: true },
                        { id: "chunks", label: "Chunks", sortable: true },
                        {
                          id: "compressed",
                          label: "Compressão",
                          sortable: true,
                        },
                      ]}
                    >
                      {sortedHypertables.map((h) => {
                        const key = `ht:${h.database_name}.${h.schema_name}.${h.hypertable_name}`;
                        return (
                          <tr
                            key={h.id}
                            className={rowClass(selectedKey === key)}
                            onClick={() => openRow(key)}
                          >
                            <td className="px-3 py-2">{h.schema_name}</td>
                            <td className="px-3 py-2 font-medium text-slate-100">
                              {h.hypertable_name}
                            </td>
                            <td className="px-3 py-2">
                              {formatBytes(h.total_size_bytes)}
                            </td>
                            <td className="px-3 py-2">
                              {fmtNum(h.num_chunks)}
                            </td>
                            <td className="px-3 py-2">
                              {boolLabel(h.compression_enabled)}
                            </td>
                          </tr>
                        );
                      })}
                    </Table>
                    {pagination}
                  </>
                )}
              </Card>
            </>
          )}

          <Sheet
            open={sheetOpen}
            onOpenChange={setSheetOpen}
            title={selectedKey ?? "Detalhe"}
            wide
          >
            {selectedTable ? (
              <TableDetail t={selectedTable} columns={detailColumns} />
            ) : null}
            {selectedIndex ? <IndexDetail i={selectedIndex} /> : null}
            {selectedView ? <ViewDetail v={selectedView} /> : null}
            {selectedFn ? <FunctionDetail f={selectedFn} /> : null}
            {selectedHt ? <HypertableDetail h={selectedHt} /> : null}
            {!selectedTable &&
            !selectedIndex &&
            !selectedView &&
            !selectedFn &&
            !selectedHt ? (
              <p className="text-sm text-slate-400">Nenhum detalhe.</p>
            ) : null}
          </Sheet>
        </div>
      )}
    </>
  );
}

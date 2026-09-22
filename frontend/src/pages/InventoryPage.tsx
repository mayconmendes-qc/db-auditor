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
import { nextSort, sortBy, type SortState } from "../lib/sort";
import { api } from "../services/api";
import type {
  DatabaseSnapshot,
  FunctionSnapshot,
  HypertableSnapshot,
  IndexSnapshot,
  InventoryObjectKind,
  PageMeta,
  SchemaSnapshot,
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

function formatNumber(n: number | null | undefined): string {
  if (n == null || !Number.isFinite(n)) {
    return "—";
  }
  return n.toLocaleString("pt-BR");
}

function boolLabel(v: boolean | null | undefined): string {
  if (v == null) {
    return "—";
  }
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

  useEffect(() => {
    if (!envId) {
      setDatabases([]);
      setSchemas([]);
      setSelectedDb(null);
      setSelectedSchema(null);
      setOffset(0);
      setError(null);
      return;
    }
    let cancelled = false;
    setDatabases(null);
    setSchemas(null);
    setSelectedDb(null);
    setSelectedSchema(null);
    setOffset(0);
    setError(null);
    Promise.all([api.databases(envId), api.schemas(envId)])
      .then(([dbRes, scRes]) => {
        if (!cancelled) {
          setDatabases(dbRes.items);
          setSchemas(scRes.items);
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
    if (!schemas) {
      return [];
    }
    if (!selectedDb) {
      return schemas;
    }
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
      if (!cancelled) {
        setLoading(false);
      }
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
            if (selectedDb) {
              items = items.filter((h) => h.database_name === selectedDb);
            }
            if (selectedSchema) {
              items = items.filter((h) => h.schema_name === selectedSchema);
            }
            if (q) {
              items = items.filter(
                (h) =>
                  matchesSearch(h.hypertable_name, q) ||
                  matchesSearch(h.schema_name, q),
              );
            }
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
            if (selectedDb) {
              items = items.filter((c) => c.database_name === selectedDb);
            }
            if (selectedSchema) {
              items = items.filter((c) => c.schema_name === selectedSchema);
            }
            if (q) {
              items = items.filter((c) => matchesSearch(c.view_name, q));
            }
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

  useEffect(() => {
    return loadObjects();
  }, [loadObjects]);

  useEffect(() => {
    setSort(null);
  }, [kind]);

  const selectRow = (key: string) => {
    setSelectedKey(key);
    setSheetOpen(true);
  };

  const handleSort = (id: string) => {
    setSort((prev) => nextSort(prev, id));
  };

  const sortedTables = useMemo(
    () =>
      sortBy(tables, sort, {
        database: (t) => t.database_name,
        schema: (t) => t.schema_name,
        name: (t) => t.table_name,
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
        kind: (v) => v.relkind,
        size: (v) => v.size_bytes,
        owner: (v) => v.owner_name ?? "",
      }),
    [views, sort],
  );

  const sortedFunctions = useMemo(
    () =>
      sortBy(functions, sort, {
        schema: (f) => f.schema_name,
        name: (f) => f.function_name,
        lang: (f) => f.language_name ?? "",
        kind: (f) => f.kind ?? "",
        owner: (f) => f.owner_name ?? "",
        security: (f) => f.is_security_definer,
      }),
    [functions, sort],
  );

  const sortedHypertables = useMemo(
    () =>
      sortBy(hypertables, sort, {
        schema: (h) => h.schema_name,
        name: (h) => h.hypertable_name,
        chunks: (h) => h.num_chunks,
        dims: (h) => h.num_dimensions,
        size: (h) => h.total_size_bytes,
        compression: (h) => h.compression_enabled,
      }),
    [hypertables, sort],
  );

  const detail = useMemo(() => {
    if (!selectedKey) {
      return null;
    }
    if (kind === "tables") {
      return tables.find(
        (t) =>
          `${t.database_name}.${t.schema_name}.${t.table_name}` === selectedKey,
      );
    }
    if (kind === "indexes") {
      return indexes.find(
        (i) =>
          `${i.database_name}.${i.schema_name}.${i.index_name}` === selectedKey,
      );
    }
    if (kind === "views" || kind === "caggs") {
      return views.find(
        (v) =>
          `${v.database_name}.${v.schema_name}.${v.view_name}` === selectedKey,
      );
    }
    if (kind === "functions") {
      return functions.find(
        (f) =>
          `${f.database_name}.${f.schema_name}.${f.function_name}` ===
          selectedKey,
      );
    }
    if (kind === "hypertables") {
      return hypertables.find(
        (h) =>
          `${h.database_name}.${h.schema_name}.${h.hypertable_name}` ===
          selectedKey,
      );
    }
    return null;
  }, [selectedKey, kind, tables, indexes, views, functions, hypertables]);

  const envName =
    environments.find((e) => e.id === envId)?.name ??
    (envId ? envId.slice(0, 8) : null);

  const objectCount = page?.total ?? 0;
  const from = objectCount === 0 ? 0 : offset + 1;
  const to = Math.min(offset + PAGE_SIZE, objectCount);
  const pageNum = Math.floor(offset / PAGE_SIZE) + 1;
  const totalPages = Math.max(1, Math.ceil(objectCount / PAGE_SIZE));

  const sheetTitle = useMemo(() => {
    if (!detail || !selectedKey) {
      return "Detalhe";
    }
    if (kind === "tables") {
      return (detail as TableSnapshot).table_name;
    }
    if (kind === "indexes") {
      return (detail as IndexSnapshot).index_name;
    }
    if (kind === "views" || kind === "caggs") {
      return (detail as ViewSnapshot).view_name;
    }
    if (kind === "functions") {
      return (detail as FunctionSnapshot).function_name;
    }
    if (kind === "hypertables") {
      return (detail as HypertableSnapshot).hypertable_name;
    }
    return selectedKey;
  }, [detail, selectedKey, kind]);

  const sheetDescription = selectedKey ? (
    <span className="font-mono text-[11px] text-slate-500">{selectedKey}</span>
  ) : undefined;

  return (
    <>
      <PageHeader
        eyebrow="INVENTÁRIO"
        title="Explorer de inventário"
        description={
          <>
            Ambiente → database → schema → objeto. Clique em uma linha para
            abrir o painel de detalhes. Paginação server-side ({PAGE_SIZE}
            /página).
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
                      const v = e.target.value;
                      setSelectedDb(v || null);
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
                      const v = e.target.value;
                      setSelectedSchema(v || null);
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
              </Card>

              <section className="flex min-h-0 min-w-0 flex-col rounded-lg border border-slate-800 bg-slate-950/40">
                <div className="flex flex-wrap items-center gap-2 border-b border-slate-800 px-3 py-2">
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
                  <div className="ml-auto flex items-center gap-2">
                    {page != null ? (
                      <span className="text-[11px] text-slate-500">
                        {objectCount === 0
                          ? "0 objetos"
                          : `${from}–${to} de ${objectCount.toLocaleString("pt-BR")}`}
                      </span>
                    ) : null}
                  </div>
                </div>

                <div className="min-h-0 flex-1 overflow-auto">
                  {listError ? (
                    <div className="p-3">
                      <ErrorBanner
                        message={listError}
                        onRetry={() => loadObjects()}
                      />
                    </div>
                  ) : null}
                  {loading ? (
                    <div className="p-3">
                      <Skeleton className="h-40 w-full" />
                    </div>
                  ) : null}
                  {!loading && kind === "tables" && tables.length === 0 ? (
                    <div className="p-4">
                      <EmptyState
                        title="Sem tabelas"
                        description="Nenhum resultado para o filtro atual."
                      />
                    </div>
                  ) : null}
                  {!loading && kind === "tables" && tables.length > 0 ? (
                    <Table
                      dense
                      sortKey={sort?.key}
                      sortDir={sort?.dir}
                      onSort={handleSort}
                      headers={[
                        { id: "database", label: "Database", sortable: true },
                        { id: "schema", label: "Schema", sortable: true },
                        { id: "name", label: "Tabela", sortable: true },
                        { id: "cols", label: "Cols", sortable: true },
                        { id: "rows", label: "Linhas est.", sortable: true },
                        { id: "size", label: "Tamanho", sortable: true },
                        { id: "pk", label: "PK", sortable: true },
                        { id: "owner", label: "Owner", sortable: true },
                      ]}
                    >
                      {sortedTables.map((t) => {
                        const key = `${t.database_name}.${t.schema_name}.${t.table_name}`;
                        return (
                          <tr
                            key={t.id}
                            className={rowClass(selectedKey === key)}
                            onClick={() => selectRow(key)}
                          >
                            <td className="px-3 py-1.5 text-xs text-slate-400">
                              {t.database_name}
                            </td>
                            <td className="px-3 py-1.5 text-xs text-slate-400">
                              {t.schema_name}
                            </td>
                            <td className="px-3 py-1.5 text-xs font-medium text-slate-100">
                              {t.table_name}
                            </td>
                            <td className="px-3 py-1.5 text-xs tabular-nums text-slate-400">
                              {t.column_count}
                            </td>
                            <td className="px-3 py-1.5 text-xs tabular-nums text-slate-400">
                              {formatNumber(t.row_estimate)}
                            </td>
                            <td className="px-3 py-1.5 text-xs tabular-nums text-slate-400">
                              {formatBytes(t.total_size_bytes)}
                            </td>
                            <td className="px-3 py-1.5 text-xs text-slate-400">
                              {boolLabel(t.has_primary_key)}
                            </td>
                            <td className="px-3 py-1.5 text-xs text-slate-500">
                              {t.owner_name ?? "—"}
                            </td>
                          </tr>
                        );
                      })}
                    </Table>
                  ) : null}
                  {!loading && kind === "indexes" && indexes.length === 0 ? (
                    <div className="p-4">
                      <EmptyState
                        title="Sem índices"
                        description="Nenhum resultado para o filtro atual."
                      />
                    </div>
                  ) : null}
                  {!loading && kind === "indexes" && indexes.length > 0 ? (
                    <Table
                      dense
                      sortKey={sort?.key}
                      sortDir={sort?.dir}
                      onSort={handleSort}
                      headers={[
                        { id: "schema", label: "Schema", sortable: true },
                        { id: "name", label: "Índice", sortable: true },
                        { id: "table", label: "Tabela", sortable: true },
                        { id: "method", label: "Método", sortable: true },
                        { id: "unique", label: "Único", sortable: true },
                        { id: "size", label: "Tamanho", sortable: true },
                        { id: "scans", label: "Scans", sortable: true },
                      ]}
                    >
                      {sortedIndexes.map((i) => {
                        const key = `${i.database_name}.${i.schema_name}.${i.index_name}`;
                        return (
                          <tr
                            key={i.id}
                            className={rowClass(selectedKey === key)}
                            onClick={() => selectRow(key)}
                          >
                            <td className="px-3 py-1.5 text-xs text-slate-400">
                              {i.schema_name}
                            </td>
                            <td className="px-3 py-1.5 text-xs font-medium text-slate-100">
                              {i.index_name}
                            </td>
                            <td className="px-3 py-1.5 text-xs text-slate-400">
                              {i.table_name}
                            </td>
                            <td className="px-3 py-1.5 text-xs text-slate-400">
                              {i.access_method ?? "—"}
                            </td>
                            <td className="px-3 py-1.5 text-xs text-slate-400">
                              {boolLabel(i.is_unique)}
                            </td>
                            <td className="px-3 py-1.5 text-xs tabular-nums text-slate-400">
                              {formatBytes(i.size_bytes)}
                            </td>
                            <td className="px-3 py-1.5 text-xs tabular-nums text-slate-400">
                              {formatNumber(i.idx_scan)}
                            </td>
                          </tr>
                        );
                      })}
                    </Table>
                  ) : null}
                  {!loading &&
                  (kind === "views" || kind === "caggs") &&
                  views.length === 0 ? (
                    <div className="p-4">
                      <EmptyState
                        title="Sem itens"
                        description="Nenhum resultado para o filtro atual."
                      />
                    </div>
                  ) : null}
                  {!loading &&
                  (kind === "views" || kind === "caggs") &&
                  views.length > 0 ? (
                    <Table
                      dense
                      sortKey={sort?.key}
                      sortDir={sort?.dir}
                      onSort={handleSort}
                      headers={[
                        { id: "schema", label: "Schema", sortable: true },
                        { id: "name", label: "Nome", sortable: true },
                        { id: "kind", label: "Tipo", sortable: true },
                        { id: "size", label: "Tamanho", sortable: true },
                        { id: "owner", label: "Owner", sortable: true },
                      ]}
                    >
                      {sortedViews.map((v) => {
                        const key = `${v.database_name}.${v.schema_name}.${v.view_name}`;
                        return (
                          <tr
                            key={v.id}
                            className={rowClass(selectedKey === key)}
                            onClick={() => selectRow(key)}
                          >
                            <td className="px-3 py-1.5 text-xs text-slate-400">
                              {v.schema_name}
                            </td>
                            <td className="px-3 py-1.5 text-xs font-medium text-slate-100">
                              {v.view_name}
                            </td>
                            <td className="px-3 py-1.5 text-xs text-slate-400">
                              {v.relkind}
                            </td>
                            <td className="px-3 py-1.5 text-xs tabular-nums text-slate-400">
                              {formatBytes(v.size_bytes)}
                            </td>
                            <td className="px-3 py-1.5 text-xs text-slate-500">
                              {v.owner_name ?? "—"}
                            </td>
                          </tr>
                        );
                      })}
                    </Table>
                  ) : null}
                  {!loading && kind === "functions" && functions.length === 0 ? (
                    <div className="p-4">
                      <EmptyState
                        title="Sem funções"
                        description="Nenhum resultado para o filtro atual."
                      />
                    </div>
                  ) : null}
                  {!loading && kind === "functions" && functions.length > 0 ? (
                    <Table
                      dense
                      sortKey={sort?.key}
                      sortDir={sort?.dir}
                      onSort={handleSort}
                      headers={[
                        { id: "schema", label: "Schema", sortable: true },
                        { id: "name", label: "Função", sortable: true },
                        { id: "lang", label: "Linguagem", sortable: true },
                        { id: "kind", label: "Kind", sortable: true },
                        {
                          id: "security",
                          label: "SECURITY DEFINER",
                          sortable: true,
                        },
                        { id: "owner", label: "Owner", sortable: true },
                      ]}
                    >
                      {sortedFunctions.map((f) => {
                        const key = `${f.database_name}.${f.schema_name}.${f.function_name}`;
                        return (
                          <tr
                            key={f.id}
                            className={rowClass(selectedKey === key)}
                            onClick={() => selectRow(key)}
                          >
                            <td className="px-3 py-1.5 text-xs text-slate-400">
                              {f.schema_name}
                            </td>
                            <td className="px-3 py-1.5 text-xs font-medium text-slate-100">
                              {f.function_name}
                            </td>
                            <td className="px-3 py-1.5 text-xs text-slate-400">
                              {f.language_name ?? "—"}
                            </td>
                            <td className="px-3 py-1.5 text-xs text-slate-400">
                              {f.kind ?? "—"}
                            </td>
                            <td className="px-3 py-1.5 text-xs text-slate-400">
                              {boolLabel(f.is_security_definer)}
                            </td>
                            <td className="px-3 py-1.5 text-xs text-slate-500">
                              {f.owner_name ?? "—"}
                            </td>
                          </tr>
                        );
                      })}
                    </Table>
                  ) : null}
                  {!loading &&
                  kind === "hypertables" &&
                  hypertables.length === 0 ? (
                    <div className="p-4">
                      <EmptyState
                        title="Sem hypertables"
                        description="Nenhum resultado para o filtro atual."
                      />
                    </div>
                  ) : null}
                  {!loading &&
                  kind === "hypertables" &&
                  hypertables.length > 0 ? (
                    <Table
                      dense
                      sortKey={sort?.key}
                      sortDir={sort?.dir}
                      onSort={handleSort}
                      headers={[
                        { id: "schema", label: "Schema", sortable: true },
                        { id: "name", label: "Hypertable", sortable: true },
                        { id: "chunks", label: "Chunks", sortable: true },
                        { id: "dims", label: "Dims", sortable: true },
                        {
                          id: "compression",
                          label: "Compressão",
                          sortable: true,
                        },
                        { id: "size", label: "Tamanho", sortable: true },
                      ]}
                    >
                      {sortedHypertables.map((h) => {
                        const key = `${h.database_name}.${h.schema_name}.${h.hypertable_name}`;
                        return (
                          <tr
                            key={h.id}
                            className={rowClass(selectedKey === key)}
                            onClick={() => selectRow(key)}
                          >
                            <td className="px-3 py-1.5 text-xs text-slate-400">
                              {h.schema_name}
                            </td>
                            <td className="px-3 py-1.5 text-xs font-medium text-slate-100">
                              {h.hypertable_name}
                            </td>
                            <td className="px-3 py-1.5 text-xs tabular-nums text-slate-400">
                              {formatNumber(h.num_chunks)}
                            </td>
                            <td className="px-3 py-1.5 text-xs tabular-nums text-slate-400">
                              {h.num_dimensions}
                            </td>
                            <td className="px-3 py-1.5 text-xs text-slate-400">
                              {boolLabel(h.compression_enabled)}
                            </td>
                            <td className="px-3 py-1.5 text-xs tabular-nums text-slate-400">
                              {formatBytes(h.total_size_bytes)}
                            </td>
                          </tr>
                        );
                      })}
                    </Table>
                  ) : null}
                </div>

                {!loading && objectCount > 0 ? (
                  <div className="flex flex-wrap items-center gap-2 border-t border-slate-800 px-3 py-2">
                    <Button
                      variant="secondary"
                      className="!px-3 !py-1.5 text-xs"
                      disabled={offset === 0}
                      onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                    >
                      Anterior
                    </Button>
                    <Button
                      variant="secondary"
                      className="!px-3 !py-1.5 text-xs"
                      disabled={!page?.has_more}
                      onClick={() => setOffset(offset + PAGE_SIZE)}
                    >
                      Próxima
                    </Button>
                    <span className="text-[11px] text-slate-500">
                      Página {pageNum} de {totalPages}
                    </span>
                  </div>
                ) : null}
              </section>
            </>
          )}
        </div>
      )}

      <Sheet
        open={sheetOpen && Boolean(detail)}
        onOpenChange={(open) => {
          setSheetOpen(open);
          if (!open) {
            setSelectedKey(null);
          }
        }}
        title={sheetTitle}
        description={sheetDescription}
        wide
      >
        {detail && kind === "tables" ? (
          <TableDetail t={detail as TableSnapshot} />
        ) : null}
        {detail && kind === "indexes" ? (
          <IndexDetail i={detail as IndexSnapshot} />
        ) : null}
        {detail && (kind === "views" || kind === "caggs") ? (
          <ViewDetail v={detail as ViewSnapshot} kind={kind} />
        ) : null}
        {detail && kind === "functions" ? (
          <FunctionDetail f={detail as FunctionSnapshot} />
        ) : null}
        {detail && kind === "hypertables" ? (
          <HypertableDetail h={detail as HypertableSnapshot} />
        ) : null}
      </Sheet>
    </>
  );
}

import { useCallback, useEffect, useMemo, useState } from "react";
import { PageHeader } from "../components/PageHeader";
import {
  Button,
  EmptyState,
  ErrorBanner,
  Input,
  Skeleton,
  Table,
} from "../components/ui";
import { useApp } from "../context/AppContext";
import { formatError } from "../lib/errors";
import { formatBytes, matchesSearch } from "../lib/format";
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
  return `cursor-pointer border-t border-slate-800/80 ${selected ? "bg-slate-800/70" : "hover:bg-slate-900/50"}`;
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

  const objectCount = page?.total;

  return (
    <>
      <PageHeader
        eyebrow="INVENTÁRIO"
        title="Explorer de inventário"
        description={
          <>
            Ambiente → database → schema → objeto. Use o seletor global na
            sidebar. Paginação server-side ({PAGE_SIZE}/página).
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
        <div className="mt-6 flex min-h-0 flex-col gap-3">
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
            <div className="grid min-h-0 grid-cols-1 gap-3 xl:grid-cols-[11rem_9rem_minmax(0,1fr)_minmax(16rem,20rem)]">
              <aside className="flex max-h-[min(70vh,36rem)] flex-col rounded-lg border border-slate-800 bg-slate-950/60">
                <div className="border-b border-slate-800 px-2.5 py-2 text-[10px] font-semibold uppercase tracking-wider text-slate-500">
                  Database
                </div>
                <div className="min-h-0 flex-1 overflow-y-auto p-1.5">
                  <button
                    type="button"
                    onClick={() => {
                      setSelectedDb(null);
                      setSelectedSchema(null);
                      setOffset(0);
                    }}
                    className={`mb-0.5 block w-full rounded px-2 py-1.5 text-left text-xs ${
                      selectedDb === null
                        ? "bg-slate-800 text-white"
                        : "text-slate-400 hover:bg-slate-900"
                    }`}
                  >
                    (todos)
                  </button>
                  {databases.map((db) => (
                    <button
                      key={db.id}
                      type="button"
                      onClick={() => {
                        setSelectedDb(db.database_name);
                        setSelectedSchema(null);
                        setOffset(0);
                      }}
                      className={`mb-0.5 block w-full truncate rounded px-2 py-1.5 text-left text-xs ${
                        selectedDb === db.database_name
                          ? "bg-slate-800 text-white"
                          : "text-slate-400 hover:bg-slate-900"
                      }`}
                      title={db.database_name}
                    >
                      {db.database_name}
                    </button>
                  ))}
                </div>
              </aside>

              <aside className="flex max-h-[min(70vh,36rem)] flex-col rounded-lg border border-slate-800 bg-slate-950/60">
                <div className="border-b border-slate-800 px-2.5 py-2 text-[10px] font-semibold uppercase tracking-wider text-slate-500">
                  Schema
                </div>
                <div className="min-h-0 flex-1 overflow-y-auto p-1.5">
                  <button
                    type="button"
                    onClick={() => {
                      setSelectedSchema(null);
                      setOffset(0);
                    }}
                    className={`mb-0.5 block w-full rounded px-2 py-1.5 text-left text-xs ${
                      selectedSchema === null
                        ? "bg-slate-800 text-white"
                        : "text-slate-400 hover:bg-slate-900"
                    }`}
                  >
                    (todos)
                  </button>
                  {schemasForDb.map((sc) => (
                    <button
                      key={sc.id}
                      type="button"
                      onClick={() => {
                        setSelectedSchema(sc.schema_name);
                        setOffset(0);
                      }}
                      className={`mb-0.5 block w-full truncate rounded px-2 py-1.5 text-left text-xs ${
                        selectedSchema === sc.schema_name
                          ? "bg-slate-800 text-white"
                          : "text-slate-400 hover:bg-slate-900"
                      }`}
                      title={sc.schema_name}
                    >
                      {sc.schema_name}
                    </button>
                  ))}
                </div>
              </aside>

              <section className="flex max-h-[min(70vh,36rem)] min-w-0 flex-col rounded-lg border border-slate-800 bg-slate-950/40">
                <div className="flex flex-wrap items-center gap-2 border-b border-slate-800 px-3 py-2">
                  {KINDS.map((k) => (
                    <button
                      key={k}
                      type="button"
                      onClick={() => {
                        setKind(k);
                        setOffset(0);
                        setSelectedKey(null);
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
                    {objectCount != null ? (
                      <span className="text-[11px] text-slate-500">
                        {objectCount} objeto(s)
                      </span>
                    ) : null}
                  </div>
                </div>
                <div className="border-b border-slate-800 px-3 py-2">
                  <Input
                    label=""
                    placeholder="Buscar nome…"
                    value={q}
                    onChange={(e) => {
                      setQ(e.target.value);
                      setOffset(0);
                    }}
                  />
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
                      <Skeleton className="h-32 w-full" />
                    </div>
                  ) : kind === "tables" ? (
                    tables.length === 0 ? (
                      <div className="p-4">
                        <EmptyState
                          title="Sem tabelas"
                          description="Nenhum resultado para o filtro atual. Tente (todos) em database ou rode uma nova auditoria."
                        />
                      </div>
                    ) : (
                      <Table headers={["Schema", "Nome", "Cols", "Size"]}>
                        {tables.map((t) => {
                          const key = `${t.database_name}.${t.schema_name}.${t.table_name}`;
                          return (
                            <tr
                              key={t.id}
                              className={rowClass(selectedKey === key)}
                              onClick={() => setSelectedKey(key)}
                            >
                              <td className="px-3 py-1.5 text-xs text-slate-400">
                                {t.schema_name}
                              </td>
                              <td className="px-3 py-1.5 text-xs font-medium text-slate-100">
                                {t.table_name}
                              </td>
                              <td className="px-3 py-1.5 text-xs text-slate-400">
                                {t.column_count}
                              </td>
                              <td className="px-3 py-1.5 text-xs text-slate-400">
                                {formatBytes(t.total_size_bytes)}
                              </td>
                            </tr>
                          );
                        })}
                      </Table>
                    )
                  ) : kind === "indexes" ? (
                    indexes.length === 0 ? (
                      <div className="p-4">
                        <EmptyState
                          title="Sem índices"
                          description="Nenhum resultado para o filtro atual."
                        />
                      </div>
                    ) : (
                      <Table headers={["Schema", "Index", "Table", "Scan"]}>
                        {indexes.map((i) => {
                          const key = `${i.database_name}.${i.schema_name}.${i.index_name}`;
                          return (
                            <tr
                              key={i.id}
                              className={rowClass(selectedKey === key)}
                              onClick={() => setSelectedKey(key)}
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
                                {i.idx_scan}
                              </td>
                            </tr>
                          );
                        })}
                      </Table>
                    )
                  ) : kind === "views" || kind === "caggs" ? (
                    views.length === 0 ? (
                      <div className="p-4">
                        <EmptyState
                          title="Sem itens"
                          description="Nenhum resultado para o filtro atual."
                        />
                      </div>
                    ) : (
                      <Table headers={["Schema", "Nome", "Kind"]}>
                        {views.map((v) => {
                          const key = `${v.database_name}.${v.schema_name}.${v.view_name}`;
                          return (
                            <tr
                              key={v.id}
                              className={rowClass(selectedKey === key)}
                              onClick={() => setSelectedKey(key)}
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
                            </tr>
                          );
                        })}
                      </Table>
                    )
                  ) : kind === "functions" ? (
                    functions.length === 0 ? (
                      <div className="p-4">
                        <EmptyState
                          title="Sem funções"
                          description="Nenhum resultado para o filtro atual."
                        />
                      </div>
                    ) : (
                      <Table headers={["Schema", "Nome", "Lang"]}>
                        {functions.map((f) => {
                          const key = `${f.database_name}.${f.schema_name}.${f.function_name}`;
                          return (
                            <tr
                              key={f.id}
                              className={rowClass(selectedKey === key)}
                              onClick={() => setSelectedKey(key)}
                            >
                              <td className="px-3 py-1.5 text-xs text-slate-400">
                                {f.schema_name}
                              </td>
                              <td className="px-3 py-1.5 text-xs font-medium text-slate-100">
                                {f.function_name}
                              </td>
                              <td className="px-3 py-1.5 text-xs text-slate-400">
                                {f.language}
                              </td>
                            </tr>
                          );
                        })}
                      </Table>
                    )
                  ) : hypertables.length === 0 ? (
                    <div className="p-4">
                      <EmptyState
                        title="Sem hypertables"
                        description="Nenhum resultado para o filtro atual."
                      />
                    </div>
                  ) : (
                    <Table headers={["Schema", "Nome", "Chunks", "Size"]}>
                      {hypertables.map((h) => {
                        const key = `${h.database_name}.${h.schema_name}.${h.hypertable_name}`;
                        return (
                          <tr
                            key={h.id}
                            className={rowClass(selectedKey === key)}
                            onClick={() => setSelectedKey(key)}
                          >
                            <td className="px-3 py-1.5 text-xs text-slate-400">
                              {h.schema_name}
                            </td>
                            <td className="px-3 py-1.5 text-xs font-medium text-slate-100">
                              {h.hypertable_name}
                            </td>
                            <td className="px-3 py-1.5 text-xs text-slate-400">
                              {h.num_chunks}
                            </td>
                            <td className="px-3 py-1.5 text-xs text-slate-400">
                              {formatBytes(h.total_size_bytes)}
                            </td>
                          </tr>
                        );
                      })}
                    </Table>
                  )}
                </div>
                {(page?.has_more || offset > 0) && !loading ? (
                  <div className="flex gap-2 border-t border-slate-800 px-3 py-2">
                    <Button
                      variant="secondary"
                      disabled={offset === 0}
                      onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                    >
                      Anterior
                    </Button>
                    <Button
                      variant="secondary"
                      disabled={!page?.has_more}
                      onClick={() => setOffset(offset + PAGE_SIZE)}
                    >
                      Próxima
                    </Button>
                    <span className="self-center text-[11px] text-slate-500">
                      offset {offset}
                    </span>
                  </div>
                ) : null}
              </section>

              <aside className="flex max-h-[min(70vh,36rem)] flex-col rounded-lg border border-slate-800 bg-slate-950/60">
                <div className="border-b border-slate-800 px-2.5 py-2 text-[10px] font-semibold uppercase tracking-wider text-slate-500">
                  Detalhe
                </div>
                <div className="min-h-0 flex-1 overflow-auto p-3">
                  {detail ? (
                    <div>
                      <p className="mb-2 break-all font-mono text-[11px] text-emerald-300/90">
                        {selectedKey}
                      </p>
                      <pre className="whitespace-pre-wrap break-all rounded bg-slate-950 p-2 text-[11px] leading-relaxed text-slate-300">
                        {JSON.stringify(detail, null, 2)}
                      </pre>
                    </div>
                  ) : (
                    <p className="text-xs text-slate-500">
                      Selecione um objeto na lista para ver o snapshot.
                    </p>
                  )}
                </div>
              </aside>
            </div>
          )}
        </div>
      )}
    </>
  );
}

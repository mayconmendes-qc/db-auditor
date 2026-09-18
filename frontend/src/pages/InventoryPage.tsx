import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  ErrorBanner,
  Input,
  Skeleton,
  Table,
} from "../components/ui";
import { formatError } from "../lib/errors";
import { formatBytes, matchesSearch } from "../lib/format";
import { api } from "../services/api";
import type {
  DatabaseSnapshot,
  Environment,
  FunctionSnapshot,
  HypertableSnapshot,
  IndexSnapshot,
  InventoryObjectKind,
  PageMeta,
  SchemaSnapshot,
  TableSnapshot,
  ViewSnapshot,
} from "../types";

const PAGE_SIZE = 25;

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

export function InventoryPage() {
  const [envs, setEnvs] = useState<Environment[] | null>(null);
  const [envId, setEnvId] = useState<string | null>(null);
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
    let cancelled = false;
    api
      .environments()
      .then((res) => {
        if (!cancelled) {
          setEnvs(res.items);
          if (res.items.length > 0) {
            setEnvId(res.items[0].id);
          }
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(formatError(err, "Falha ao carregar ambientes"));
          setEnvs([]);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!envId) {
      return;
    }
    let cancelled = false;
    setDatabases(null);
    setSchemas(null);
    setSelectedDb(null);
    setSelectedSchema(null);
    setOffset(0);
    Promise.all([api.databases(envId), api.schemas(envId)])
      .then(([dbRes, scRes]) => {
        if (!cancelled) {
          setDatabases(dbRes.items);
          setSchemas(scRes.items);
          if (dbRes.items.length > 0) {
            setSelectedDb(dbRes.items[0].database_name);
          }
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
    if (!schemas || !selectedDb) {
      return [];
    }
    return schemas.filter((s) => s.database_name === selectedDb);
  }, [schemas, selectedDb]);

  const loadObjects = useCallback(() => {
    if (!envId) {
      return;
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
        .catch((e: unknown) =>
          fail(formatError(e, "Falha ao listar tables")),
        )
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
        .catch((e: unknown) =>
          fail(formatError(e, "Falha ao listar indexes")),
        )
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
        .catch((e: unknown) =>
          fail(formatError(e, "Falha ao listar views")),
        )
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
        .catch((e: unknown) =>
          fail(formatError(e, "Falha ao listar CAGGs")),
        )
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

  return (
    <>
      <p className="text-xs font-bold tracking-[0.12em] text-emerald-300">
        INVENTÁRIO
      </p>
      <h1 className="mt-2 text-3xl font-semibold text-slate-50 md:text-4xl">
        Explorer de inventário
      </h1>
      <p className="mt-3 max-w-2xl text-slate-400">
        Navegação ambiente → database → schema → objeto com paginação
        server-side.
      </p>

      <div className="mt-8 space-y-6">
        {error ? (
          <ErrorBanner
            message={error}
            onRetry={() => {
              setError(null);
              setEnvs(null);
              api
                .environments()
                .then((res) => {
                  setEnvs(res.items);
                  if (res.items.length > 0) {
                    setEnvId(res.items[0].id);
                  }
                })
                .catch((err: unknown) => {
                  setError(formatError(err, "Falha ao carregar ambientes"));
                  setEnvs([]);
                });
            }}
          />
        ) : envs === null ? (
          <Skeleton className="h-16 w-full" />
        ) : envs.length === 0 ? (
          <EmptyState
            title="Nenhum ambiente"
            description="Configure ambientes via .env e rode discovery, ou use o seed demo (make reset-volume && make up)."
          />
        ) : (
          <div className="flex flex-wrap gap-2">
            {envs.map((e) => (
              <button
                key={e.id}
                type="button"
                onClick={() => setEnvId(e.id)}
                className={`rounded-md px-3 py-1.5 text-sm ${
                  envId === e.id
                    ? "bg-emerald-700 text-white"
                    : "bg-slate-800 text-slate-300"
                }`}
              >
                {e.name}
              </button>
            ))}
          </div>
        )}

        {databases === null ? (
          <Skeleton className="h-24 w-full" />
        ) : (
          <div className="grid gap-4 lg:grid-cols-[12rem_10rem_1fr]">
            <div>
              <h3 className="mb-2 text-xs font-semibold uppercase text-slate-400">
                Database
              </h3>
              {databases.map((db) => (
                <button
                  key={db.id}
                  type="button"
                  onClick={() => {
                    setSelectedDb(db.database_name);
                    setSelectedSchema(null);
                    setOffset(0);
                  }}
                  className={`mb-1 block w-full rounded px-2 py-1.5 text-left text-sm ${
                    selectedDb === db.database_name
                      ? "bg-slate-800 text-white"
                      : "text-slate-400 hover:bg-slate-900"
                  }`}
                >
                  {db.database_name}
                </button>
              ))}
            </div>
            <div>
              <h3 className="mb-2 text-xs font-semibold uppercase text-slate-400">
                Schema
              </h3>
              <button
                type="button"
                onClick={() => {
                  setSelectedSchema(null);
                  setOffset(0);
                }}
                className={`mb-1 block w-full rounded px-2 py-1.5 text-left text-sm ${
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
                  className={`mb-1 block w-full rounded px-2 py-1.5 text-left text-sm ${
                    selectedSchema === sc.schema_name
                      ? "bg-slate-800 text-white"
                      : "text-slate-400 hover:bg-slate-900"
                  }`}
                >
                  {sc.schema_name}
                </button>
              ))}
            </div>
            <div className="space-y-4">
              <div className="flex flex-wrap gap-2">
                {KINDS.map((k) => (
                  <button
                    key={k}
                    type="button"
                    onClick={() => {
                      setKind(k);
                      setOffset(0);
                      setSelectedKey(null);
                    }}
                    className={`rounded-md px-3 py-1 text-sm ${
                      kind === k
                        ? "bg-emerald-800 text-white"
                        : "bg-slate-800 text-slate-300"
                    }`}
                  >
                    {KIND_LABELS[k]}
                  </button>
                ))}
              </div>
              <Input
                label="Busca"
                placeholder="nome…"
                value={q}
                onChange={(e) => {
                  setQ(e.target.value);
                  setOffset(0);
                }}
              />
              {page ? (
                <p className="text-sm text-slate-400">{page.total} objeto(s)</p>
              ) : null}
              {listError ? (
                <ErrorBanner
                  message={listError}
                  onRetry={() => loadObjects()}
                />
              ) : null}
              {loading ? (
                <Skeleton className="h-40 w-full" />
              ) : kind === "tables" ? (
                tables.length === 0 ? (
                  <EmptyState title="Sem tabelas" description="Nenhum resultado para o filtro atual." />
                ) : (
                  <Table headers={["Schema", "Nome", "Cols", "Size"]}>
                    {tables.map((t) => {
                      const key = `${t.database_name}.${t.schema_name}.${t.table_name}`;
                      return (
                        <tr
                          key={t.id}
                          className={`cursor-pointer border-t border-slate-800 ${
                            selectedKey === key ? "bg-slate-900/80" : ""
                          }`}
                          onClick={() => setSelectedKey(key)}
                        >
                          <td className="px-4 py-3 text-slate-300">
                            {t.schema_name}
                          </td>
                          <td className="px-4 py-3 text-slate-100">
                            {t.table_name}
                          </td>
                          <td className="px-4 py-3 text-slate-300">
                            {t.column_count}
                          </td>
                          <td className="px-4 py-3 text-slate-300">
                            {formatBytes(t.total_size_bytes)}
                          </td>
                        </tr>
                      );
                    })}
                  </Table>
                )
              ) : kind === "indexes" ? (
                indexes.length === 0 ? (
                  <EmptyState title="Sem índices" description="Nenhum resultado para o filtro atual." />
                ) : (
                  <Table headers={["Schema", "Index", "Table", "Scan"]}>
                    {indexes.map((i) => {
                      const key = `${i.database_name}.${i.schema_name}.${i.index_name}`;
                      return (
                        <tr
                          key={i.id}
                          className={`cursor-pointer border-t border-slate-800 ${
                            selectedKey === key ? "bg-slate-900/80" : ""
                          }`}
                          onClick={() => setSelectedKey(key)}
                        >
                          <td className="px-4 py-3 text-slate-300">
                            {i.schema_name}
                          </td>
                          <td className="px-4 py-3 text-slate-100">
                            {i.index_name}
                          </td>
                          <td className="px-4 py-3 text-slate-300">
                            {i.table_name}
                          </td>
                          <td className="px-4 py-3 text-slate-300">
                            {i.idx_scan}
                          </td>
                        </tr>
                      );
                    })}
                  </Table>
                )
              ) : kind === "views" || kind === "caggs" ? (
                views.length === 0 ? (
                  <EmptyState title="Sem itens" description="Nenhum resultado para o filtro atual." />
                ) : (
                  <Table headers={["Schema", "Nome", "Kind"]}>
                    {views.map((v) => {
                      const key = `${v.database_name}.${v.schema_name}.${v.view_name}`;
                      return (
                        <tr
                          key={v.id}
                          className={`cursor-pointer border-t border-slate-800 ${
                            selectedKey === key ? "bg-slate-900/80" : ""
                          }`}
                          onClick={() => setSelectedKey(key)}
                        >
                          <td className="px-4 py-3 text-slate-300">
                            {v.schema_name}
                          </td>
                          <td className="px-4 py-3 text-slate-100">
                            {v.view_name}
                          </td>
                          <td className="px-4 py-3 text-slate-300">
                            {v.relkind}
                          </td>
                        </tr>
                      );
                    })}
                  </Table>
                )
              ) : kind === "functions" ? (
                functions.length === 0 ? (
                  <EmptyState title="Sem funções" description="Nenhum resultado para o filtro atual." />
                ) : (
                  <Table headers={["Schema", "Nome", "SECURITY"]}>
                    {functions.map((f) => {
                      const key = `${f.database_name}.${f.schema_name}.${f.function_name}`;
                      return (
                        <tr
                          key={f.id}
                          className={`cursor-pointer border-t border-slate-800 ${
                            selectedKey === key ? "bg-slate-900/80" : ""
                          }`}
                          onClick={() => setSelectedKey(key)}
                        >
                          <td className="px-4 py-3 text-slate-300">
                            {f.schema_name}
                          </td>
                          <td className="px-4 py-3 text-slate-100">
                            {f.function_name}
                          </td>
                          <td className="px-4 py-3 text-slate-300">
                            {f.security_definer ? "DEFINER" : "INVOKER"}
                          </td>
                        </tr>
                      );
                    })}
                  </Table>
                )
              ) : hypertables.length === 0 ? (
                <EmptyState title="Sem hypertables" description="Nenhum resultado para o filtro atual." />
              ) : (
                <Table headers={["Schema", "Nome", "Chunks", "Size"]}>
                  {hypertables.map((h) => {
                    const key = `${h.database_name}.${h.schema_name}.${h.hypertable_name}`;
                    return (
                      <tr
                        key={h.id}
                        className={`cursor-pointer border-t border-slate-800 ${
                          selectedKey === key ? "bg-slate-900/80" : ""
                        }`}
                        onClick={() => setSelectedKey(key)}
                      >
                        <td className="px-4 py-3 text-slate-300">
                          {h.schema_name}
                        </td>
                        <td className="px-4 py-3 text-slate-100">
                          {h.hypertable_name}
                        </td>
                        <td className="px-4 py-3 text-slate-300">
                          {h.num_chunks}
                        </td>
                        <td className="px-4 py-3 text-slate-300">
                          {formatBytes(h.total_size_bytes)}
                        </td>
                      </tr>
                    );
                  })}
                </Table>
              )}
              {page && page.has_more ? (
                <div className="flex gap-2">
                  <Button
                    disabled={offset === 0}
                    onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                  >
                    Anterior
                  </Button>
                  <Button
                    onClick={() => setOffset(offset + PAGE_SIZE)}
                  >
                    Próxima
                  </Button>
                </div>
              ) : null}
              {detail ? (
                <Card title="Detalhe" subtitle={selectedKey ?? ""}>
                  <pre className="mt-2 max-h-56 overflow-auto rounded bg-slate-950 p-3 text-xs text-slate-300">
                    {JSON.stringify(detail, null, 2)}
                  </pre>
                </Card>
              ) : null}
            </div>
          </div>
        )}
      </div>
    </>
  );
}

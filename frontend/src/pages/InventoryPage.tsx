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
import { nextSort, type SortState, sortBy } from "../lib/sort";
import { api } from "../services/api";
import type {
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
  const [snapshotStatus, setSnapshotStatus] =
    useState<SnapshotCompleteness | null>(null);

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

  const envName =
    environments.find((e) => e.id === envId)?.name ??
    (envId ? envId.slice(0, 8) : null);

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
              {snapshotStatus.failed_databases > 0 ||
              snapshotStatus.failed_collectors > 0 ? (
                <div className="mt-1 text-xs">
                  {snapshotStatus.failed_databases > 0
                    ? `${snapshotStatus.failed_databases} database(s) com falha na cobertura. `
                    : ""}
                  {snapshotStatus.failed_collectors > 0
                    ? `${snapshotStatus.failed_collectors} collector(s) com falha.`
                    : ""}
                </div>
              ) : null}
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
                  <ErrorBanner message={listError} onRetry={() => loadObjects()} />
                </div>
              ) : null}
              {loading ? (
                <Skeleton className="mt-3 h-40 w-full" />
              ) : (
                <p className="mt-3 text-xs text-slate-500">
                  {page
                    ? `${page.total.toLocaleString("pt-BR")} objeto(s) · ${KIND_LABELS[kind]}`
                    : "Carregando…"}
                </p>
              )}
            </Card>
          )}

          <Sheet
            open={sheetOpen}
            onOpenChange={setSheetOpen}
            title={selectedKey ?? "Detalhe"}
          >
            <p className="text-sm text-slate-400">Detalhe do objeto selecionado.</p>
          </Sheet>
        </div>
      )}
    </>
  );
}

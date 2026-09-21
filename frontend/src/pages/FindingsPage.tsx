import { useCallback, useEffect, useMemo, useState } from "react";
import { PageHeader } from "../components/PageHeader";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  ErrorBanner,
  Select,
  Skeleton,
  Table,
} from "../components/ui";
import { useApp } from "../context/AppContext";
import { formatError } from "../lib/errors";
import { downloadCSV, downloadJSON } from "../lib/export";
import { labels } from "../lib/labels";
import { nextSort, type SortState, sortBy } from "../lib/sort";
import { api } from "../services/api";
import type { Finding } from "../types";

function severityTone(
  severity: string,
): "success" | "warning" | "danger" | "neutral" {
  if (severity === "critical" || severity === "high") {
    return "danger";
  }
  if (severity === "medium") {
    return "warning";
  }
  return "neutral";
}

function statusTone(
  status: string,
): "success" | "warning" | "danger" | "neutral" {
  if (status === "open") {
    return "danger";
  }
  if (status === "acknowledged") {
    return "warning";
  }
  if (status === "resolved" || status === "suppressed") {
    return "success";
  }
  return "neutral";
}

const SEVERITY_OPTIONS = [
  { value: "", label: "Todas as severidades" },
  { value: "critical", label: "Crítica" },
  { value: "high", label: "Alta" },
  { value: "medium", label: "Média" },
  { value: "low", label: "Baixa" },
  { value: "info", label: "Informativo" },
];

const STATUS_OPTIONS = [
  { value: "", label: "Todos os status" },
  { value: "open", label: "Aberto" },
  { value: "acknowledged", label: "Reconhecido" },
  { value: "resolved", label: "Resolvido" },
  { value: "suppressed", label: "Suprimido" },
];

const demoFacts = {
  environment_id: "00000000-0000-0000-0000-000000000001",
  tables: [
    {
      database: "app",
      schema: "public",
      name: "events",
      size_bytes: 3_000_000_000,
    },
    {
      database: "app",
      schema: "public",
      name: "orders",
      size_bytes: 50_000_000,
    },
  ],
  indexes: [
    {
      database: "app",
      schema: "public",
      table_name: "orders",
      index_name: "idx_orders_legacy",
      idx_scan: 0,
      size_bytes: 12_000_000,
    },
  ],
  hypertables: [
    {
      database: "app",
      schema: "public",
      name: "metrics",
      num_chunks: 620,
      size_bytes: 8_000_000_000,
    },
  ],
  chunks: [
    {
      database: "app",
      schema: "public",
      hypertable_name: "metrics",
      chunk_name: "_hyper_1_1",
      size_bytes: 100_000_000,
    },
    {
      database: "app",
      schema: "public",
      hypertable_name: "metrics",
      chunk_name: "_hyper_1_2",
      size_bytes: 100_000_000,
    },
    {
      database: "app",
      schema: "public",
      hypertable_name: "metrics",
      chunk_name: "_hyper_1_3",
      size_bytes: 900_000_000,
    },
  ],
  caggs: [
    {
      database: "app",
      schema: "public",
      view_name: "metrics_1h",
      has_refresh_policy: false,
      materialization_schema: "_timescaledb_internal",
      materialization_hypertable: "_materialized_hypertable_1",
    },
    {
      database: "app",
      schema: "public",
      view_name: "metrics_1d",
      has_refresh_policy: true,
    },
  ],
  policies: [
    {
      database: "app",
      job_id: 101,
      policy_type: "retention",
      hypertable_schema: "public",
      hypertable_name: "metrics",
      last_run_status: "failed",
      schedule_interval: "1 day",
      proc_name: "policy_retention",
    },
  ],
  jobs: [
    {
      database: "app",
      job_id: 202,
      proc_name: "custom_cleanup",
      last_run_status: "failed",
      total_failures: 4,
      scheduled: true,
    },
  ],
  activity: [
    {
      database: "app",
      schema: "public",
      name: "legacy_events",
      object_type: "table",
      days_since_dml: 180,
      n_live_tup: 1000,
      n_tup_ins: 0,
      n_tup_upd: 0,
      n_tup_del: 0,
    },
  ],
};

function FilterChip({
  label,
  onClear,
}: {
  label: string;
  onClear: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClear}
      className="inline-flex items-center gap-1 rounded-full border border-slate-600 bg-slate-800/80 px-2.5 py-0.5 text-[11px] text-slate-200 hover:border-slate-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400/50"
    >
      {label}
      <span className="text-slate-400" aria-hidden>
        ×
      </span>
    </button>
  );
}

export function FindingsPage() {
  const { environmentId, setSection } = useApp();
  const [items, setItems] = useState<Finding[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [bulkBusy, setBulkBusy] = useState(false);
  const [selected, setSelected] = useState<Finding | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());
  const [severityFilter, setSeverityFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("open");
  const [sort, setSort] = useState<SortState | null>(null);

  const load = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      const res = await api.findings({
        severity: severityFilter || undefined,
        status: statusFilter || undefined,
        environment_id: environmentId || undefined,
      });
      setItems(res.items);
      setSelectedIds(new Set());
    } catch (err: unknown) {
      setError(formatError(err, "Falha ao listar findings"));
      setItems([]);
    } finally {
      setBusy(false);
    }
  }, [severityFilter, statusFilter, environmentId]);

  useEffect(() => {
    void load();
  }, [load]);

  const runAnalyze = async () => {
    setBusy(true);
    setError(null);
    try {
      await api.analyzeFindings(demoFacts);
      await load();
    } catch (err: unknown) {
      setError(formatError(err, "Falha no analyze"));
    } finally {
      setBusy(false);
    }
  };

  const triage = async (id: string, status: string) => {
    try {
      const updated = await api.updateFindingStatus(id, status);
      setItems((prev) => prev.map((f) => (f.id === id ? updated : f)));
      setSelected((cur) => (cur?.id === id ? updated : cur));
    } catch (err: unknown) {
      setError(formatError(err, "Falha na triagem"));
    }
  };

  const bulkTriage = async (status: string) => {
    const ids = Array.from(selectedIds);
    if (ids.length === 0) {
      return;
    }
    setBulkBusy(true);
    setError(null);
    let failed = 0;
    const updatedMap = new Map<string, Finding>();
    await Promise.all(
      ids.map(async (id) => {
        try {
          const updated = await api.updateFindingStatus(id, status);
          updatedMap.set(id, updated);
        } catch {
          failed += 1;
        }
      }),
    );
    setItems((prev) =>
      prev.map((f) => {
        const next = updatedMap.get(f.id);
        return next ?? f;
      }),
    );
    setSelected((cur) => {
      if (!cur) {
        return cur;
      }
      return updatedMap.get(cur.id) ?? cur;
    });
    setSelectedIds(new Set());
    setBulkBusy(false);
    if (failed > 0) {
      const ok = ids.length - failed;
      setError(`Triagem em lote parcial: ${ok} ok, ${failed} falha(s).`);
    }
  };

  const toggleOne = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const allSelected =
    items.length > 0 && items.every((f) => selectedIds.has(f.id));

  const toggleAll = () => {
    if (allSelected) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(items.map((f) => f.id)));
    }
  };

  const sortedItems = useMemo(
    () =>
      sortBy(items, sort, {
        type: (f) => f.finding_type,
        severity: (f) => f.severity,
        status: (f) => f.status,
        title: (f) => f.title,
        object: (f) => f.object_key ?? "",
        last_seen: (f) => f.last_seen_at ?? "",
      }),
    [items, sort],
  );

  const exportRows = () => {
    const stamp = new Date().toISOString().slice(0, 19).replace(/[:T]/g, "-");
    const rows = sortedItems.map((f) => ({
      id: f.id,
      finding_type: f.finding_type,
      severity: f.severity,
      status: f.status,
      title: f.title,
      summary: f.summary,
      object_key: f.object_key ?? "",
      environment_id: f.environment_id ?? "",
      last_seen_at: f.last_seen_at ?? "",
    }));
    downloadCSV(
      `findings-${stamp}.csv`,
      [
        "id",
        "finding_type",
        "severity",
        "status",
        "title",
        "summary",
        "object_key",
        "environment_id",
        "last_seen_at",
      ],
      rows,
    );
  };

  const exportJson = () => {
    const stamp = new Date().toISOString().slice(0, 19).replace(/[:T]/g, "-");
    downloadJSON(`findings-${stamp}.json`, {
      exported_at: new Date().toISOString(),
      filters: {
        environment_id: environmentId || null,
        severity: severityFilter || null,
        status: statusFilter || null,
      },
      total: sortedItems.length,
      items: sortedItems,
    });
  };

  const health = useMemo(() => {
    const open = items.filter((f) => f.status === "open");
    const byPrefix = (prefix: string) =>
      open.filter((f) => f.finding_type.startsWith(prefix)).length;
    return {
      open: open.length,
      cagg: byPrefix("cagg."),
      policy: byPrefix("policy.") + byPrefix("job."),
      inactivity: byPrefix("inactivity."),
    };
  }, [items]);

  const activeChips: Array<{ key: string; label: string; clear: () => void }> =
    [];
  if (severityFilter) {
    activeChips.push({
      key: "sev",
      label: `Severidade: ${labels.severity(severityFilter)}`,
      clear: () => setSeverityFilter(""),
    });
  }
  if (statusFilter) {
    activeChips.push({
      key: "status",
      label: `Status: ${labels.findingStatus(statusFilter)}`,
      clear: () => setStatusFilter(""),
    });
  }

  const selectionCount = selectedIds.size;

  return (
    <>
      <PageHeader
        eyebrow="Findings"
        title="Findings"
        description="Diagnósticos de storage, índices, chunks, CAGGs, policies/jobs e inatividade (POSSIBLY_INACTIVE — sem exclusão automática)."
        actions={
          <div className="flex flex-wrap gap-2">
            <Button
              variant="secondary"
              onClick={exportRows}
              disabled={busy || items.length === 0}
            >
              Exportar CSV
            </Button>
            <Button
              variant="secondary"
              onClick={exportJson}
              disabled={busy || items.length === 0}
            >
              Exportar JSON
            </Button>
            <Button onClick={() => void runAnalyze()} disabled={busy}>
              {busy ? "Analisando…" : "Rodar analyzers (demo)"}
            </Button>
          </div>
        }
      />

      <div className="mt-8 space-y-6">
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <Card title={String(health.open)} subtitle="Abertos" />
          <Card title={String(health.cagg)} subtitle="CAGG" />
          <Card title={String(health.policy)} subtitle="Policies/Jobs" />
          <Card title={String(health.inactivity)} subtitle="Inatividade" />
        </div>

        <div className="grid gap-3 sm:grid-cols-2">
          <Select
            label="Severidade"
            options={SEVERITY_OPTIONS}
            value={severityFilter}
            onChange={(e) => setSeverityFilter(e.target.value)}
          />
          <Select
            label="Status"
            options={STATUS_OPTIONS}
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
          />
        </div>

        {activeChips.length > 0 ? (
          <div className="flex flex-wrap gap-2">
            {activeChips.map((c) => (
              <FilterChip key={c.key} label={c.label} onClear={c.clear} />
            ))}
          </div>
        ) : null}

        <div className="flex flex-wrap items-center gap-2">
          <Button onClick={() => void load()} disabled={busy || bulkBusy}>
            Atualizar
          </Button>
          {selectionCount > 0 ? (
            <>
              <span className="text-xs text-slate-400">
                {selectionCount} selecionado(s)
              </span>
              <Button
                disabled={bulkBusy}
                onClick={() => void bulkTriage("acknowledged")}
              >
                Reconhecer
              </Button>
              <Button
                disabled={bulkBusy}
                onClick={() => void bulkTriage("resolved")}
              >
                Resolver
              </Button>
              <Button
                disabled={bulkBusy}
                onClick={() => void bulkTriage("suppressed")}
              >
                Suprimir
              </Button>
              <Button
                variant="secondary"
                disabled={bulkBusy}
                onClick={() => void bulkTriage("open")}
              >
                Reabrir
              </Button>
            </>
          ) : null}
          {bulkBusy ? (
            <span className="text-xs text-slate-400">Aplicando triagem…</span>
          ) : null}
        </div>

        {error ? (
          <ErrorBanner message={error} onRetry={() => void load()} />
        ) : null}
        {busy ? <Skeleton className="h-40 w-full" /> : null}

        {!busy && items.length === 0 ? (
          <EmptyState
            title="Sem findings"
            description="Nenhum resultado após filtros. Rode os analyzers demo ou limpe os filtros."
            action={
              <div className="flex flex-wrap justify-center gap-2">
                <Button onClick={() => void runAnalyze()} disabled={busy}>
                  Rodar analyzers (demo)
                </Button>
                <Button
                  variant="secondary"
                  onClick={() => setSection("Execuções")}
                >
                  Ir para Execuções
                </Button>
              </div>
            }
          />
        ) : null}

        {!busy && sortedItems.length > 0 ? (
          <Table
            dense
            headers={[
              { id: "sel", label: "Sel." },
              { id: "type", label: "Tipo", sortable: true },
              { id: "severity", label: "Severidade", sortable: true },
              { id: "status", label: "Status", sortable: true },
              { id: "title", label: "Título", sortable: true },
              { id: "object", label: "Objeto", sortable: true },
              { id: "last_seen", label: "Última vez", sortable: true },
            ]}
            sortKey={sort?.key}
            sortDir={sort?.dir}
            onSort={(id) => {
              if (id === "sel") {
                return;
              }
              setSort((prev) => nextSort(prev, id));
            }}
          >
            <tr className="border-t border-slate-800 bg-slate-900/40">
              <td className="px-3 py-1.5">
                <input
                  type="checkbox"
                  checked={allSelected}
                  onChange={toggleAll}
                  aria-label="Selecionar todos"
                  className="rounded border-slate-600 bg-slate-900 text-emerald-500 focus:ring-emerald-400/50"
                />
              </td>
              <td className="px-3 py-1.5 text-xs text-slate-500" colSpan={6}>
                Selecionar todos na página ({sortedItems.length})
              </td>
            </tr>
            {sortedItems.map((f) => (
              <tr
                key={f.id}
                className="cursor-pointer border-t border-slate-800 hover:bg-slate-900/50"
                onClick={() => setSelected(f)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    setSelected(f);
                  }
                }}
              >
                <td className="px-3 py-1.5">
                  <input
                    type="checkbox"
                    checked={selectedIds.has(f.id)}
                    onClick={(e) => {
                      e.stopPropagation();
                    }}
                    onChange={() => {
                      toggleOne(f.id);
                    }}
                    aria-label={`Selecionar ${f.title}`}
                    className="rounded border-slate-600 bg-slate-900 text-emerald-500 focus:ring-emerald-400/50"
                  />
                </td>
                <td className="px-3 py-1.5 font-mono text-xs text-slate-300">
                  {f.finding_type}
                </td>
                <td className="px-3 py-1.5">
                  <Badge tone={severityTone(f.severity)}>
                    {labels.severity(f.severity)}
                  </Badge>
                </td>
                <td className="px-3 py-1.5">
                  <Badge tone={statusTone(f.status)}>
                    {labels.findingStatus(f.status)}
                  </Badge>
                </td>
                <td className="px-3 py-1.5 text-slate-100">{f.title}</td>
                <td className="px-3 py-1.5 font-mono text-xs text-slate-400">
                  {f.object_key || "—"}
                </td>
                <td className="px-3 py-1.5 text-xs text-slate-400">
                  {f.last_seen_at
                    ? new Date(f.last_seen_at).toLocaleString()
                    : "—"}
                </td>
              </tr>
            ))}
          </Table>
        ) : null}

        {selected ? (
          <Card
            title={`Detalhe · ${labels.severity(selected.severity)}`}
            subtitle={selected.title}
          >
            <ul className="mt-3 space-y-1 text-sm text-slate-300">
              <li>Tipo: {selected.finding_type}</li>
              <li>Status: {labels.findingStatus(selected.status)}</li>
              <li>Objeto: {selected.object_key || "—"}</li>
              <li>Resumo: {selected.summary}</li>
            </ul>
            {selected.finding_type.startsWith("inactivity.") ? (
              <p className="mt-3 text-xs text-amber-300">
                Classificação POSSIBLY_INACTIVE — o auditor nunca recomenda
                DROP, TRUNCATE ou exclusão automática.
              </p>
            ) : null}
            {selected.evidence ? (
              <pre className="mt-3 overflow-auto rounded bg-slate-950 p-3 text-xs text-slate-400">
                {JSON.stringify(selected.evidence, null, 2)}
              </pre>
            ) : null}
            <div className="mt-4 flex flex-wrap gap-2">
              <Button onClick={() => void triage(selected.id, "acknowledged")}>
                Reconhecer
              </Button>
              <Button onClick={() => void triage(selected.id, "resolved")}>
                Resolver
              </Button>
              <Button onClick={() => void triage(selected.id, "suppressed")}>
                Suprimir
              </Button>
              <Button
                variant="secondary"
                onClick={() => void triage(selected.id, "open")}
              >
                Reabrir
              </Button>
            </div>
          </Card>
        ) : null}
      </div>
    </>
  );
}

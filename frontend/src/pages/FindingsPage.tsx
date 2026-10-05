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
import {
  type PageSize,
  PaginationControls,
} from "../components/ui/PaginationControls";
import { useApp } from "../context/AppContext";
import { formatError } from "../lib/errors";
import { downloadCSV, downloadJSON } from "../lib/export";
import { findingGuidance } from "../lib/findingGuidance";
import { labels } from "../lib/labels";
import { fetchAllPages } from "../lib/pagination";
import { nextSort, type SortState, sortBy } from "../lib/sort";
import { api } from "../services/api";
import type { Finding, FindingEvent } from "../types";

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
  const {
    environmentId,
    setSection,
    search,
    setSearch,
    findingId,
    openFinding,
  } = useApp();
  const [items, setItems] = useState<Finding[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [pageSize, setPageSize] = useState<PageSize>(20);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [bulkBusy, setBulkBusy] = useState(false);
  const [selected, setSelected] = useState<Finding | null>(null);
  const [timeline, setTimeline] = useState<FindingEvent[]>([]);
  const [suppressionReason, setSuppressionReason] = useState("");
  const [suppressedUntil, setSuppressedUntil] = useState("");

  useEffect(() => {
    if (!selected) {
      setTimeline([]);
      return;
    }
    let active = true;
    void api
      .findingTimeline(selected.id)
      .then((result) => {
        if (active) setTimeline(result.items);
      })
      .catch(() => {
        if (active) setTimeline([]);
      });
    return () => {
      active = false;
    };
  }, [selected?.id]);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());
  const [severityFilter, setSeverityFilter] = useState(search.severity || "");
  const [statusFilter, setStatusFilter] = useState(search.status ?? "open");
  const [mine, setMine] = useState(search.mine === "1");
  const [overdueOnly, setOverdueOnly] = useState(search.overdue === "1");
  const [assignee, setAssignee] = useState("");
  const [dueAt, setDueAt] = useState("");
  const [sort, setSort] = useState<SortState | null>(null);

  const load = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      const filters = {
        severity: severityFilter || undefined,
        status: statusFilter || undefined,
        environment_id: environmentId || undefined,
        assignee: mine ? "me" : undefined,
        overdue: overdueOnly ? "1" : undefined,
      };
      if (pageSize === "all") {
        const rows = await fetchAllPages((pageOffset, limit) =>
          api.findingsPage({ ...filters, offset: pageOffset, limit }),
        );
        setItems(rows);
        setTotal(rows.length);
      } else {
        const res = await api.findingsPage({
          ...filters,
          offset,
          limit: pageSize,
        });
        setItems(res.items);
        setTotal(res.page.total);
      }
      setSelectedIds(new Set());
    } catch (err: unknown) {
      setError(formatError(err, "Falha ao listar findings"));
      setItems([]);
    } finally {
      setBusy(false);
    }
  }, [
    severityFilter,
    statusFilter,
    environmentId,
    offset,
    pageSize,
    mine,
    overdueOnly,
  ]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    setSearch({
      severity: severityFilter || null,
      status: statusFilter || null,
      mine: mine ? "1" : null,
      overdue: overdueOnly ? "1" : null,
    });
  }, [severityFilter, statusFilter, mine, overdueOnly, setSearch]);

  useEffect(() => {
    const nextSeverity = search.severity || "";
    const nextStatus = search.status ?? "open";
    const nextMine = search.mine === "1";
    const nextOverdue = search.overdue === "1";
    setSeverityFilter((current) =>
      current === nextSeverity ? current : nextSeverity,
    );
    setStatusFilter((current) =>
      current === nextStatus ? current : nextStatus,
    );
    setMine((current) => (current === nextMine ? current : nextMine));
    setOverdueOnly((current) =>
      current === nextOverdue ? current : nextOverdue,
    );
  }, [search.severity, search.status, search.mine, search.overdue]);

  useEffect(() => {
    if (!findingId) return;
    let active = true;
    void api
      .finding(findingId)
      .then((item) => {
        if (active) setSelected(item);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, [findingId]);

  useEffect(() => {
    setAssignee(selected?.assignee || "");
    setDueAt(selected?.due_at ? selected.due_at.slice(0, 16) : "");
  }, [selected]);

  const saveWorkflow = async () => {
    if (!selected) return;
    try {
      const due = dueAt ? new Date(dueAt).toISOString() : undefined;
      const updated = await api.updateFindingWorkflow(
        selected.id,
        assignee.trim(),
        due,
      );
      setItems((prev) => prev.map((f) => (f.id === updated.id ? updated : f)));
      setSelected(updated);
    } catch (err: unknown) {
      setError(formatError(err, "Falha ao salvar responsável ou prazo"));
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

  const suppressSelected = async () => {
    if (!selected || !suppressionReason.trim() || !suppressedUntil) return;
    try {
      const updated = await api.suppressFinding(
        selected.id,
        suppressionReason.trim(),
        new Date(suppressedUntil).toISOString(),
      );
      setItems((prev) => prev.map((f) => (f.id === updated.id ? updated : f)));
      setSelected(updated);
      setSuppressionReason("");
      setSuppressedUntil("");
      setTimeline((await api.findingTimeline(updated.id)).items);
    } catch (err: unknown) {
      setError(formatError(err, "Falha ao suprimir finding"));
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
      clear: () => {
        setSeverityFilter("");
        setOffset(0);
      },
    });
  }
  if (statusFilter) {
    activeChips.push({
      key: "status",
      label: `Status: ${labels.findingStatus(statusFilter)}`,
      clear: () => {
        setStatusFilter("");
        setOffset(0);
      },
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

        <div className="flex flex-wrap gap-2">
          <Button
            variant={mine ? "primary" : "secondary"}
            onClick={() => {
              setMine((value) => !value);
              setOffset(0);
            }}
          >
            Meus
          </Button>
          <Button
            variant={overdueOnly ? "primary" : "secondary"}
            onClick={() => {
              setOverdueOnly((value) => !value);
              setOffset(0);
            }}
          >
            Atrasados
          </Button>
          <Button
            variant="secondary"
            onClick={() => {
              setSeverityFilter("critical");
              setStatusFilter("open");
              setOffset(0);
            }}
          >
            Críticos
          </Button>
          <Button
            variant="secondary"
            onClick={() => {
              setStatusFilter("suppressed");
              setSeverityFilter("");
              setOffset(0);
            }}
          >
            Suprimidos
          </Button>
        </div>

        <div className="grid gap-3 sm:grid-cols-2">
          <Select
            label="Severidade"
            options={SEVERITY_OPTIONS}
            value={severityFilter}
            onChange={(e) => {
              setSeverityFilter(e.target.value);
              setOffset(0);
            }}
          />
          <Select
            label="Status"
            options={STATUS_OPTIONS}
            value={statusFilter}
            onChange={(e) => {
              setStatusFilter(e.target.value);
              setOffset(0);
            }}
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
          {selectionCount > 0 && api.hasRole("auditor") ? (
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
            description="Nenhum resultado após filtros. Execute uma auditoria ou limpe os filtros."
            action={
              <div className="flex flex-wrap justify-center gap-2">
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
          <>
            <Table
              dense
              pagination={false}
              virtualize
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
                    disabled={!api.hasRole("auditor")}
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
                  onClick={() => {
                    setSelected(f);
                    openFinding(f.id);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      setSelected(f);
                      openFinding(f.id);
                    }
                  }}
                >
                  <td className="px-3 py-1.5">
                    <input
                      type="checkbox"
                      disabled={!api.hasRole("auditor")}
                      checked={selectedIds.has(f.id)}
                      onClick={(e) => {
                        e.stopPropagation();
                      }}
                      onChange={() => {
                        toggleOne(f.id);
                      }}
                      aria-label={`Selecionar achado em ${f.object_key || "objeto não informado"}`}
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
                  <td className="px-3 py-1.5 text-slate-100">
                    {findingGuidance(f.finding_type).meaning}
                  </td>
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
            <PaginationControls
              total={total}
              offset={pageSize === "all" ? 0 : offset}
              size={pageSize}
              onSizeChange={(next) => {
                setPageSize(next);
                setOffset(0);
              }}
              onOffsetChange={setOffset}
              label="Achados"
            />
          </>
        ) : null}

        {selected ? (
          <Card
            title={`Detalhe · ${labels.severity(selected.severity)}`}
            subtitle={findingGuidance(selected.finding_type).meaning}
          >
            <ul className="mt-3 space-y-1 text-sm text-slate-300">
              <li>Status: {labels.findingStatus(selected.status)}</li>
              <li>
                Primeira observação:{" "}
                {new Date(selected.first_seen_at).toLocaleString()}
              </li>
              <li>
                Última observação:{" "}
                {new Date(selected.last_seen_at).toLocaleString()}
              </li>
              <li>Recorrências: {selected.recurrence_count ?? 0}</li>
              {selected.resolved_at ? (
                <li>
                  Resolução: {new Date(selected.resolved_at).toLocaleString()}
                </li>
              ) : null}
              {selected.superseded_by ? (
                <li>Substituído por: {selected.superseded_by}</li>
              ) : null}
              {selected.suppression_reason ? (
                <li>
                  Supressão: {selected.suppression_reason} (até{" "}
                  {selected.suppressed_until
                    ? new Date(selected.suppressed_until).toLocaleString()
                    : "—"}
                  )
                </li>
              ) : null}
              <li>Objeto: {selected.object_key || "—"}</li>
              <li>
                Confiança:{" "}
                {selected.confidence != null
                  ? `${Math.round(selected.confidence * 100)}%`
                  : "não informada"}
              </li>
              <li>
                Próximo passo: {findingGuidance(selected.finding_type).next}
              </li>
            </ul>
            <details className="mt-3 text-xs text-slate-400">
              <summary className="cursor-pointer">
                Detalhes técnicos da regra
              </summary>
              <p className="mt-2">Título original: {selected.title}</p>
              <p>
                Regra: {selected.rule_id || selected.finding_type}{" "}
                {selected.rule_version ? `(v${selected.rule_version})` : ""}
              </p>
              <p>Categoria: {selected.category || "—"}</p>
              <p>Impacto: {selected.impact || "—"}</p>
              <p>Risco/ressalva: {selected.risk || "revisão humana necessária"}</p>
              <p>Resumo original: {selected.summary}</p>
              <p>Recomendação original: {selected.recommendation || "—"}</p>
              <p>Validação original: {selected.validation || "—"}</p>
            </details>
            {selected.references?.length ? (
              <div className="mt-3 text-xs text-slate-400">
                Referências:{" "}
                {selected.references.map((url) => (
                  <a
                    key={url}
                    className="mr-2 underline"
                    href={url}
                    target="_blank"
                    rel="noreferrer"
                  >
                    PostgreSQL
                  </a>
                ))}
              </div>
            ) : null}
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
            {api.hasRole("auditor") ? (
              <div className="mt-4 flex flex-wrap gap-2">
                <Button
                  onClick={() => void triage(selected.id, "acknowledged")}
                >
                  Reconhecer
                </Button>
                <Button onClick={() => void triage(selected.id, "resolved")}>
                  Resolver
                </Button>
                <Button
                  variant="secondary"
                  onClick={() => void triage(selected.id, "open")}
                >
                  Reabrir
                </Button>
              </div>
            ) : null}
            {api.hasRole("auditor") ? (
              <div className="mt-4 grid gap-2 sm:grid-cols-3">
                <input
                  className="rounded border border-slate-600 bg-slate-900 px-3 py-2 text-sm text-slate-100"
                  aria-label="Responsável"
                  placeholder="Responsável"
                  value={assignee}
                  onChange={(event) => setAssignee(event.target.value)}
                />
                <input
                  className="rounded border border-slate-600 bg-slate-900 px-3 py-2 text-sm text-slate-100"
                  aria-label="Prazo"
                  type="datetime-local"
                  value={dueAt}
                  onChange={(event) => setDueAt(event.target.value)}
                />
                <div className="flex gap-2">
                  <Button
                    disabled={!assignee.trim() && !dueAt}
                    onClick={() => void saveWorkflow()}
                  >
                    Salvar prazo
                  </Button>
                  {api.currentUser() ? (
                    <Button
                      variant="secondary"
                      onClick={() => setAssignee(api.currentUser())}
                    >
                      Eu
                    </Button>
                  ) : null}
                </div>
              </div>
            ) : null}
            {api.hasRole("auditor") ? (
              <div className="mt-4 grid gap-2 sm:grid-cols-3">
                <input
                  className="rounded border border-slate-600 bg-slate-900 px-3 py-2 text-sm text-slate-100"
                  aria-label="Motivo da supressão"
                  placeholder="Motivo da supressão"
                  value={suppressionReason}
                  onChange={(event) => setSuppressionReason(event.target.value)}
                />
                <input
                  className="rounded border border-slate-600 bg-slate-900 px-3 py-2 text-sm text-slate-100"
                  aria-label="Validade da supressão"
                  type="datetime-local"
                  value={suppressedUntil}
                  onChange={(event) => setSuppressedUntil(event.target.value)}
                />
                <Button
                  disabled={!suppressionReason.trim() || !suppressedUntil}
                  onClick={() => void suppressSelected()}
                >
                  Suprimir com validade
                </Button>
              </div>
            ) : null}
            <h3 className="mt-5 text-sm font-semibold text-slate-200">
              Linha do tempo
            </h3>
            <ul className="mt-2 space-y-1 text-xs text-slate-400">
              {timeline.map((event) => (
                <li key={event.id}>
                  {new Date(event.recorded_at).toLocaleString()} ·{" "}
                  {event.event_type}
                  {event.reason ? ` — ${event.reason}` : ""}
                </li>
              ))}
            </ul>
          </Card>
        ) : null}
      </div>
    </>
  );
}

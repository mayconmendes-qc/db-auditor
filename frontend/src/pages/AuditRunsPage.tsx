import { useCallback, useEffect, useMemo, useState } from "react";
import { PageHeader } from "../components/PageHeader";
import {
  Badge,
  Button,
  Card,
  ConfirmDialog,
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
import { labels } from "../lib/labels";
import { fetchAllPages } from "../lib/pagination";
import { api } from "../services/api";
import type {
  AnalysisRun,
  AuditBaseline,
  AuditRun,
  AuditRunCoverage,
  BaselineComparison,
  CollectorRun,
  ScopeScore,
} from "../types";

const POLL_MS = 2500;

const STATUS_OPTIONS = [
  { value: "", label: "Todos os status" },
  { value: "running", label: "Em execução" },
  { value: "success", label: "Sucesso" },
  { value: "partial_success", label: "Sucesso parcial" },
  { value: "failed", label: "Falhou" },
  { value: "cancelled", label: "Cancelado" },
  { value: "skipped", label: "Ignorado" },
];

const PROFILE_OPTIONS = [
  { value: "", label: "Todos os perfis" },
  { value: "manual", label: "Manual" },
  { value: "fast", label: "Rápido" },
  { value: "daily", label: "Diário" },
  { value: "weekly", label: "Semanal" },
  { value: "monthly", label: "Mensal" },
];

const SCHEDULE_PROFILES = [
  { value: "fast", label: "Rápido (15 min)" },
  { value: "daily", label: "Diário" },
  { value: "weekly", label: "Semanal" },
  { value: "monthly", label: "Mensal" },
];

function nextRunLabel(value?: string): string {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "—";
  }
  return date.toLocaleString("pt-BR");
}

function statusTone(
  status: string,
): "success" | "warning" | "danger" | "neutral" {
  if (status === "success") {
    return "success";
  }
  if (status === "partial_success" || status === "running") {
    return "warning";
  }
  if (status === "failed") {
    return "danger";
  }
  return "neutral";
}

function envLabel(run: AuditRun): string {
  if (run.environment_name?.trim()) {
    return run.environment_name;
  }
  return `${run.environment_id.slice(0, 8)}…`;
}

function isTerminal(status: string): boolean {
  return (
    status === "success" ||
    status === "partial_success" ||
    status === "failed" ||
    status === "cancelled" ||
    status === "skipped"
  );
}

function durationLabel(start: string, end?: string | null): string {
  const a = new Date(start).getTime();
  const b = end ? new Date(end).getTime() : Date.now();
  if (Number.isNaN(a) || Number.isNaN(b) || b < a) {
    return "—";
  }
  const sec = Math.round((b - a) / 1000);
  if (sec < 60) {
    return `${sec}s`;
  }
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return `${m}m ${s}s`;
}

function CollectorProgress({
  collectors,
  runStatus,
}: {
  collectors: CollectorRun[];
  runStatus: string;
}) {
  const total = collectors.length;
  const done = collectors.filter((c) => isTerminal(c.status)).length;
  const running = collectors.filter((c) => c.status === "running").length;
  const failed = collectors.filter((c) => c.status === "failed").length;
  const pct = total > 0 ? Math.round((done / total) * 100) : 0;
  const live = runStatus === "running" || running > 0;

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-slate-300">
        <span>
          {done}/{total || "…"} collectors
          {running > 0 ? ` · ${running} em execução` : ""}
          {failed > 0 ? ` · ${failed} falha(s)` : ""}
        </span>
        <span className="font-mono text-slate-400">
          {total > 0 ? `${pct}%` : live ? "…" : "—"}
          {live ? " · ao vivo" : ""}
        </span>
      </div>
      <div
        className="h-2 overflow-hidden rounded-full bg-slate-800"
        role="progressbar"
        aria-valuenow={pct}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        <div
          className={`h-full rounded-full transition-all duration-500 ${
            failed > 0
              ? "bg-rose-500/80"
              : live
                ? "bg-amber-400/90"
                : "bg-emerald-500/80"
          }`}
          style={{ width: `${total > 0 ? pct : live ? 8 : 0}%` }}
        />
      </div>
    </div>
  );
}

export function AuditRunsPage() {
  const { environments, environmentId } = useApp();
  const [runs, setRuns] = useState<AuditRun[] | null>(null);
  const [runTotal, setRunTotal] = useState(0);
  const [runOffset, setRunOffset] = useState(0);
  const [runPageSize, setRunPageSize] = useState<PageSize>(20);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState("");
  const [profileFilter, setProfileFilter] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [collectors, setCollectors] = useState<CollectorRun[] | null>(null);
  const [coverage, setCoverage] = useState<AuditRunCoverage[] | null>(null);
  const [analysis, setAnalysis] = useState<AnalysisRun | null>(null);
  const [baseline, setBaseline] = useState<AuditBaseline | null>(null);
  const [baselineToken, setBaselineToken] = useState("");
  const [comparison, setComparison] = useState<BaselineComparison | null>(null);
  const [scopeScore, setScopeScore] = useState<ScopeScore | null>(null);
  const [triggerEnv, setTriggerEnv] = useState("");
  const [triggerMsg, setTriggerMsg] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [polling, setPolling] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [schedules, setSchedules] = useState<
    { profile: string; enabled: boolean; next_run_at?: string; last_status?: string }[]
  >([]);
  const [scheduleMsg, setScheduleMsg] = useState<string | null>(null);

  useEffect(() => {
    if (environmentId) {
      setTriggerEnv(environmentId);
    } else if (environments.length > 0 && !triggerEnv) {
      setTriggerEnv(environments[0].id);
    }
  }, [environmentId, environments, triggerEnv]);

  useEffect(() => {
    if (!triggerEnv) {
      setSchedules([]);
      return;
    }
    let cancelled = false;
    void api
      .schedules(triggerEnv)
      .then((result) => {
        if (!cancelled) {
          setSchedules(result.items);
        }
      })
      .catch((cause: unknown) => {
        if (!cancelled) {
          setScheduleMsg(formatError(cause));
        }
      });
    return () => {
      cancelled = true;
    };
  }, [triggerEnv]);

  const loadRuns = useCallback(() => {
    setError(null);
    const filters = {
      status: statusFilter || undefined,
      profile: profileFilter || undefined,
      environment_id: environmentId || undefined,
    };
    const request =
      runPageSize === "all"
        ? fetchAllPages((offset, limit) =>
            api.auditRunsPage({ ...filters, offset, limit }),
          ).then((items) => ({ items, total: items.length }))
        : api
            .auditRunsPage({
              ...filters,
              offset: runOffset,
              limit: runPageSize,
            })
            .then((result) => ({
              items: result.items,
              total: result.page.total,
            }));
    return request
      .then((res) => {
        setRuns(res.items);
        setRunTotal(res.total);
        return res.items;
      })
      .catch((err: unknown) => {
        setError(
          formatError(
            err,
            "Falha ao listar execuções. Verifique a API e tente novamente.",
          ),
        );
        setRuns([]);
        return [] as AuditRun[];
      });
  }, [statusFilter, profileFilter, environmentId, runOffset, runPageSize]);

  useEffect(() => {
    setRuns(null);
    void loadRuns();
  }, [loadRuns]);

  const loadCollectors = useCallback(async (runId: string) => {
    try {
      const res = await api.auditRunCollectors(runId);
      setCollectors(res.items);
      return res.items;
    } catch {
      setCollectors([]);
      return [] as CollectorRun[];
    }
  }, []);

  const loadRunDiagnostics = useCallback(async (runId: string) => {
    const [coverageResult, analysisResult] = await Promise.allSettled([
      api.auditRunCoverage(runId),
      api.auditRunAnalysis(runId),
    ]);
    setCoverage(
      coverageResult.status === "fulfilled" ? coverageResult.value.items : [],
    );
    setAnalysis(
      analysisResult.status === "fulfilled" ? analysisResult.value : null,
    );
  }, []);

  useEffect(() => {
    if (!selectedId) {
      setCollectors(null);
      setCoverage(null);
      setAnalysis(null);
      return;
    }
    setCollectors(null);
    setCoverage(null);
    void loadCollectors(selectedId);
    void loadRunDiagnostics(selectedId);
  }, [selectedId, loadCollectors, loadRunDiagnostics]);

  const selected = useMemo(
    () => runs?.find((r) => r.id === selectedId) ?? null,
    [runs, selectedId],
  );

  useEffect(() => {
    if (!selected || selected.status === "running") {
      setBaseline(null);
      setComparison(null);
      setScopeScore(null);
      return;
    }
    let active = true;
    void Promise.allSettled([
      api.auditBaseline(selected.environment_id),
      api.baselineComparisons(selected.environment_id, selected.id),
      api.scopeScore(selected.environment_id, selected.id),
    ]).then(([base, comparisons, score]) => {
      if (!active) return;
      setBaseline(base.status === "fulfilled" ? base.value : null);
      setComparison(
        comparisons.status === "fulfilled"
          ? (comparisons.value.items[0] ?? null)
          : null,
      );
      setScopeScore(score.status === "fulfilled" ? score.value : null);
    });
    return () => {
      active = false;
    };
  }, [selected]);

  const chooseBaseline = async () => {
    if (
      !selected ||
      !window.confirm(
        `Usar a execução ${selected.id} como baseline aprovado para todo o ambiente?`,
      )
    )
      return;
    try {
      api.setReportToken(baselineToken);
      setBaseline(
        await api.selectAuditBaseline(selected.environment_id, selected.id),
      );
      setError(null);
    } catch (err: unknown) {
      setError(formatError(err, "Não foi possível aprovar o baseline"));
    }
  };

  useEffect(() => {
    const listRunning = runs?.some((r) => r.status === "running") ?? false;
    const selectedRunning = selected?.status === "running";
    if ((!listRunning && !selectedRunning) || runPageSize === "all") {
      setPolling(false);
      return;
    }
    setPolling(true);
    const id = window.setInterval(() => {
      void loadRuns().then((items) => {
        const still = items.find((r) => r.id === selectedId);
        if (still && selectedId) {
          void loadCollectors(selectedId);
          void loadRunDiagnostics(selectedId);
        }
      });
    }, POLL_MS);
    return () => window.clearInterval(id);
  }, [
    runs,
    selected,
    selectedId,
    loadRuns,
    loadCollectors,
    loadRunDiagnostics,
    runPageSize,
  ]);

  const triggerEnvName =
    environments.find((e) => e.id === triggerEnv)?.name ??
    (triggerEnv ? `${triggerEnv.slice(0, 8)}…` : "—");

  const executeTrigger = async () => {
    if (!triggerEnv) {
      return;
    }
    setBusy(true);
    setTriggerMsg(null);
    try {
      const res = await api.triggerAuditRun(triggerEnv, "manual");
      setTriggerMsg(
        `Execução iniciada (${res.audit_run_id.slice(0, 8)}…) — ${labels.runStatus(res.status)}`,
      );
      setSelectedId(res.audit_run_id);
      await loadRuns();
      await loadCollectors(res.audit_run_id);
    } catch (err: unknown) {
      setTriggerMsg(
        formatError(
          err,
          "Não foi possível iniciar a execução. Verifique o ambiente e a API.",
        ),
      );
    } finally {
      setBusy(false);
      setConfirmOpen(false);
    }
  };

  const onTriggerClick = () => {
    if (!triggerEnv) {
      return;
    }
    setConfirmOpen(true);
  };

  return (
    <>
      <PageHeader
        eyebrow="Execuções"
        title="Execuções de auditoria"
        description="Dispare coletas, acompanhe o progresso por collector e revise erros."
        actions={
          polling ? (
            <span className="inline-flex items-center gap-2 rounded-md border border-amber-500/30 bg-amber-500/10 px-3 py-1.5 text-xs text-amber-200">
              <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-amber-400" />
              Atualizando…
            </span>
          ) : null
        }
      />

      <ConfirmDialog
        open={confirmOpen}
        title="Disparar auditoria manual"
        description={
          <>
            Será iniciada uma coleta somente leitura no ambiente{" "}
            <strong className="text-slate-100">{triggerEnvName}</strong>.
            Nenhuma alteração é aplicada nos bancos auditados.
          </>
        }
        confirmLabel="Executar agora"
        cancelLabel="Cancelar"
        busy={busy}
        onCancel={() => {
          if (!busy) {
            setConfirmOpen(false);
          }
        }}
        onConfirm={() => {
          void executeTrigger();
        }}
      />

      <div className="mt-8 space-y-6">
        {api.hasRole("operator") ? (
          <Card title="Disparo manual">
            <div className="mt-3 flex flex-col gap-3 sm:flex-row sm:items-end">
              <div className="min-w-0 flex-1">
                <Select
                  label="Ambiente"
                  options={environments.map((e) => ({
                    value: e.id,
                    label: e.name,
                  }))}
                  value={triggerEnv}
                  onChange={(e) => setTriggerEnv(e.target.value)}
                />
              </div>
              <Button onClick={onTriggerClick} disabled={busy || !triggerEnv}>
                {busy ? "Executando…" : "Executar agora"}
              </Button>
            </div>
            {triggerMsg ? (
              <p className="mt-3 text-sm text-slate-300">{triggerMsg}</p>
            ) : null}
          </Card>
        ) : null}

        {triggerEnv ? (
          <Card title="Agenda">
            <p className="mt-1 text-sm text-slate-400">
              A agenda fica no snapshot store. Reiniciar a API não desliga o perfil.
            </p>
            <ul className="mt-4 space-y-3">
              {SCHEDULE_PROFILES.map((profile) => {
                const row = schedules.find((item) => item.profile === profile.value);
                return (
                  <li
                    key={profile.value}
                    className="flex flex-wrap items-center justify-between gap-3 text-sm"
                  >
                    <label className="flex items-center gap-2 text-slate-100">
                      <input
                        type="checkbox"
                        checked={Boolean(row?.enabled)}
                        disabled={!api.hasRole("operator") || busy}
                        onChange={(event) => {
                          const enabled = event.target.checked;
                          setBusy(true);
                          setScheduleMsg(null);
                          void api
                            .saveSchedule(triggerEnv, profile.value, enabled)
                            .then((saved) => {
                              setSchedules((current) => {
                                const next = current.filter(
                                  (item) => item.profile !== profile.value,
                                );
                                next.push(saved);
                                return next;
                              });
                            })
                            .catch((cause: unknown) => setScheduleMsg(formatError(cause)))
                            .finally(() => setBusy(false));
                        }}
                      />
                      {profile.label}
                    </label>
                    <span className="text-slate-400">
                      Próxima: {nextRunLabel(row?.next_run_at)}
                      {row?.last_status ? ` · última ${row.last_status}` : ""}
                    </span>
                  </li>
                );
              })}
            </ul>
            {scheduleMsg ? (
              <p className="mt-3 text-sm text-rose-300">{scheduleMsg}</p>
            ) : null}
          </Card>
        ) : null}

        <div className="grid w-full gap-3 sm:grid-cols-2">
          <Select
            label="Status"
            options={STATUS_OPTIONS}
            value={statusFilter}
            onChange={(e) => {
              setStatusFilter(e.target.value);
              setRunOffset(0);
            }}
          />
          <Select
            label="Perfil"
            options={PROFILE_OPTIONS}
            value={profileFilter}
            onChange={(e) => {
              setProfileFilter(e.target.value);
              setRunOffset(0);
            }}
          />
        </div>

        {error ? (
          <ErrorBanner message={error} onRetry={() => void loadRuns()} />
        ) : null}

        {runs === null ? (
          <Skeleton className="h-40 w-full" />
        ) : runs.length === 0 ? (
          <EmptyState
            title="Nenhuma execução"
            description="Dispare uma auditoria manual acima ou aguarde o scheduler. Ambientes demo vêm do seed local."
            action={
              <Button onClick={onTriggerClick} disabled={busy || !triggerEnv}>
                Executar agora
              </Button>
            }
          />
        ) : (
          <>
            <Table
              dense
              pagination={false}
              headers={["Perfil", "Status", "Duração", "Ambiente"]}
            >
              {runs.map((r) => (
                <tr
                  key={r.id}
                  className={`cursor-pointer border-t border-slate-800 hover:bg-slate-900/50 ${
                    selectedId === r.id ? "bg-slate-900/80" : ""
                  }`}
                  onClick={() => setSelectedId(r.id)}
                >
                  <td className="px-3 py-1.5 text-slate-100">{r.profile}</td>
                  <td className="px-3 py-1.5">
                    <Badge tone={statusTone(r.status)}>
                      {labels.runStatus(r.status)}
                    </Badge>
                  </td>
                  <td className="px-3 py-1.5 text-xs text-slate-400">
                    {durationLabel(r.started_at, r.finished_at)}
                    <span className="mt-0.5 block font-mono text-[10px] text-slate-500">
                      {new Date(r.started_at).toLocaleString()}
                    </span>
                  </td>
                  <td className="px-3 py-1.5 text-slate-200">{envLabel(r)}</td>
                </tr>
              ))}
            </Table>
            <PaginationControls
              total={runTotal}
              offset={runPageSize === "all" ? 0 : runOffset}
              size={runPageSize}
              onSizeChange={(next) => {
                setRunPageSize(next);
                setRunOffset(0);
              }}
              onOffsetChange={setRunOffset}
              label="Execuções"
            />
          </>
        )}

        {selected ? (
          <section className="space-y-4">
            <h2 className="text-lg font-semibold text-slate-100">Detalhe</h2>
            <Card
              title={`${selected.profile} · ${labels.runStatus(selected.status)}`}
              subtitle={`${envLabel(selected)} · ${selected.id.slice(0, 8)}…`}
            >
              {selected.status === "running" ? (
                <Button
                  variant="secondary"
                  onClick={() =>
                    void api
                      .cancelAuditRun(selected.id)
                      .then(() => loadRuns())
                      .catch((cause: unknown) =>
                        setError(
                          formatError(
                            cause,
                            "Não foi possível cancelar a execução",
                          ),
                        ),
                      )
                  }
                >
                  Cancelar execução
                </Button>
              ) : null}
              <ul className="mt-1 space-y-1 text-sm text-slate-300">
                <li>
                  Início: {new Date(selected.started_at).toLocaleString()}
                </li>
                <li>
                  Fim:{" "}
                  {selected.finished_at
                    ? new Date(selected.finished_at).toLocaleString()
                    : "—"}
                </li>
                <li>
                  Duração:{" "}
                  {durationLabel(selected.started_at, selected.finished_at)}
                </li>
                <li>Avisos: {selected.warnings.length}</li>
                <li>Erros: {selected.errors.length}</li>
              </ul>
              {selected.errors.length > 0 ? (
                <ul className="mt-3 list-inside list-disc text-xs text-rose-300">
                  {selected.errors.slice(0, 8).map((e) => (
                    <li key={e}>{e}</li>
                  ))}
                </ul>
              ) : null}
            </Card>

            <div className="grid gap-3 sm:grid-cols-2">
              <Card
                title="Baseline aprovado"
                subtitle={
                  baseline
                    ? `Execução ${baseline.audit_run_id.slice(0, 8)}…`
                    : "Ainda não definido"
                }
              >
                <p className="mt-2 text-xs text-slate-400">
                  A seleção é registrada no histórico e exige cobertura
                  completa.
                </p>
                {!api.hasSession() && (
                  <input
                    aria-label="Token para aprovar baseline"
                    type="password"
                    autoComplete="off"
                    placeholder="Token de operação protegida"
                    value={baselineToken}
                    onChange={(event) => setBaselineToken(event.target.value)}
                    className="mt-2 w-full rounded border border-slate-600 bg-slate-900 px-3 py-2 text-sm text-slate-100"
                  />
                )}
                {selected.status === "success" ? (
                  <div className="mt-3">
                    <Button
                      disabled={!api.hasRole("operator") && !baselineToken}
                      onClick={() => void chooseBaseline()}
                    >
                      Aprovar esta execução
                    </Button>
                  </div>
                ) : null}
                {comparison ? (
                  <p className="mt-2 text-sm text-slate-300">
                    Comparação {comparison.status}: +{comparison.added_tables} /
                    −{comparison.removed_tables} tabelas;{" "}
                    {comparison.changed_tables} alteradas.
                  </p>
                ) : null}
              </Card>
              <Card
                title="Score por escopo"
                subtitle={scopeScore?.version ?? "Aguardando"}
              >
                <p className="mt-2 text-sm text-slate-300">
                  {scopeScore?.score == null
                    ? "Indisponível por cobertura insuficiente"
                    : `${scopeScore.score}/100`}{" "}
                  · confiança {Math.round((scopeScore?.confidence ?? 0) * 100)}%
                </p>
                {scopeScore?.categories.map((category) => (
                  <p key={category.category} className="text-xs text-slate-400">
                    {category.category}: {category.score}/100 (
                    {category.findings} findings)
                  </p>
                ))}
              </Card>
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              <Card
                title="Análise automática"
                subtitle={
                  analysis ? labels.runStatus(analysis.status) : "Aguardando"
                }
              >
                <p className="mt-2 text-sm text-slate-300">
                  {analysis
                    ? `${analysis.findings_saved}/${analysis.findings_produced} findings persistidos`
                    : "A análise será executada após a coleta."}
                </p>
                {analysis?.error ? (
                  <p className="mt-2 text-xs text-rose-300">{analysis.error}</p>
                ) : null}
              </Card>
              <Card
                title="Cobertura"
                subtitle={
                  coverage === null
                    ? "Carregando…"
                    : `${coverage.filter((item) => item.database_name).length} combinações collector/database`
                }
              >
                <p className="mt-2 text-sm text-slate-300">
                  {coverage
                    ? `${coverage.filter((item) => item.status === "failed").length} falha(s) registradas`
                    : "—"}
                </p>
              </Card>
            </div>

            {coverage?.some((item) => item.database_name) ? (
              <Card title="Progresso por database">
                <Table
                  dense
                  headers={["Database", "Collector", "Estado", "Ação"]}
                >
                  {coverage
                    .filter((item) => item.database_name)
                    .map((item) => (
                      <tr key={`${item.collector_name}:${item.database_name}`}>
                        <td className="px-3 py-2">{item.database_name}</td>
                        <td className="px-3 py-2">{item.collector_name}</td>
                        <td className="px-3 py-2">{item.status}</td>
                        <td className="px-3 py-2">
                          {selected.status === "running" &&
                          api.hasRole("operator") &&
                          item.database_name &&
                          item.status === "attempted" ? (
                            <button
                              type="button"
                              onClick={() =>
                                void api
                                  .cancelAuditDatabase(
                                    selected.id,
                                    item.database_name ?? "",
                                  )
                                  .then(() => loadRunDiagnostics(selected.id))
                                  .catch((cause: unknown) =>
                                    setError(
                                      formatError(
                                        cause,
                                        "Falha ao cancelar database",
                                      ),
                                    ),
                                  )
                              }
                              className="text-rose-300 hover:underline"
                            >
                              Cancelar database
                            </button>
                          ) : (
                            "—"
                          )}
                        </td>
                      </tr>
                    ))}
                </Table>
              </Card>
            ) : null}

            {coverage?.some((item) => item.status === "failed") ? (
              <Card title="Falhas de cobertura">
                <ul className="mt-2 space-y-1 text-xs text-rose-300">
                  {coverage
                    .filter((item) => item.status === "failed")
                    .slice(0, 12)
                    .map((item) => (
                      <li
                        key={`${item.collector_name}:${item.database_name ?? "all"}`}
                      >
                        {item.collector_name}
                        {item.database_name ? ` · ${item.database_name}` : ""}
                        {item.error ? ` — ${item.error}` : ""}
                      </li>
                    ))}
                </ul>
              </Card>
            ) : null}

            <div className="space-y-3">
              <div className="flex items-center justify-between gap-2">
                <h3 className="text-sm font-medium text-slate-200">
                  Collectors
                </h3>
                <Button
                  variant="ghost"
                  onClick={() => void loadCollectors(selected.id)}
                >
                  Atualizar
                </Button>
              </div>

              {collectors === null ? (
                <Skeleton className="h-24 w-full" />
              ) : collectors.length === 0 ? (
                <EmptyState
                  title="Sem collectors ainda"
                  description={
                    selected.status === "running"
                      ? "A execução ainda está iniciando os collectors…"
                      : "Nenhum registro de collector para esta execução."
                  }
                />
              ) : (
                <>
                  <CollectorProgress
                    collectors={collectors}
                    runStatus={selected.status}
                  />
                  <Table dense headers={["Nome", "Status", "Rows", "Duração"]}>
                    {collectors.map((c) => (
                      <tr key={c.id} className="border-t border-slate-800">
                        <td className="px-3 py-1.5 text-xs text-slate-100">
                          {c.collector_name}
                          {c.error ? (
                            <span className="mt-0.5 block truncate text-[10px] text-rose-300">
                              {c.error}
                            </span>
                          ) : c.warning ? (
                            <span className="mt-0.5 block truncate text-[10px] text-amber-300">
                              {c.warning}
                            </span>
                          ) : null}
                        </td>
                        <td className="px-3 py-1.5">
                          <Badge tone={statusTone(c.status)}>
                            {labels.runStatus(c.status)}
                          </Badge>
                        </td>
                        <td className="px-3 py-1.5 font-mono text-xs text-slate-300">
                          {c.rows_collected}
                        </td>
                        <td className="px-3 py-1.5 font-mono text-xs text-slate-400">
                          {durationLabel(c.started_at, c.finished_at)}
                        </td>
                      </tr>
                    ))}
                  </Table>
                </>
              )}
            </div>
          </section>
        ) : null}
      </div>
    </>
  );
}

import { useCallback, useEffect, useMemo, useState } from "react";
import { PageHeader } from "../components/PageHeader";
import {
  Badge,
  Button,
  Card,
  ConfirmDialog,
  EmptyState,
  ErrorBanner,
  Input,
  Skeleton,
  Table,
} from "../components/ui";
import { useApp } from "../context/AppContext";
import { formatError } from "../lib/errors";
import { labels } from "../lib/labels";
import { api } from "../services/api";
import type { AuditRun, CollectorRun } from "../types";

const POLL_MS = 2500;

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
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState("");
  const [profileFilter, setProfileFilter] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [collectors, setCollectors] = useState<CollectorRun[] | null>(null);
  const [triggerEnv, setTriggerEnv] = useState("");
  const [triggerMsg, setTriggerMsg] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [polling, setPolling] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);

  useEffect(() => {
    if (environmentId) {
      setTriggerEnv(environmentId);
    } else if (environments.length > 0 && !triggerEnv) {
      setTriggerEnv(environments[0].id);
    }
  }, [environmentId, environments, triggerEnv]);

  const loadRuns = useCallback(() => {
    setError(null);
    return api
      .auditRuns({
        status: statusFilter || undefined,
        profile: profileFilter || undefined,
        environment_id: environmentId || undefined,
      })
      .then((res) => {
        setRuns(res.items);
        return res.items;
      })
      .catch((err: unknown) => {
        setError(formatError(err, "Falha ao listar execuções"));
        setRuns([]);
        return [] as AuditRun[];
      });
  }, [statusFilter, profileFilter, environmentId]);

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

  useEffect(() => {
    if (!selectedId) {
      setCollectors(null);
      return;
    }
    setCollectors(null);
    void loadCollectors(selectedId);
  }, [selectedId, loadCollectors]);

  const selected = useMemo(
    () => runs?.find((r) => r.id === selectedId) ?? null,
    [runs, selectedId],
  );

  useEffect(() => {
    const listRunning = runs?.some((r) => r.status === "running") ?? false;
    const selectedRunning = selected?.status === "running";
    if (!listRunning && !selectedRunning) {
      setPolling(false);
      return;
    }
    setPolling(true);
    const id = window.setInterval(() => {
      void loadRuns().then((items) => {
        const still = items.find((r) => r.id === selectedId);
        if (still && selectedId) {
          void loadCollectors(selectedId);
        }
      });
    }, POLL_MS);
    return () => window.clearInterval(id);
  }, [runs, selected, selectedId, loadRuns, loadCollectors]);

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
        `Run ${res.audit_run_id.slice(0, 8)}… → ${labels.runStatus(res.status)}`,
      );
      setSelectedId(res.audit_run_id);
      await loadRuns();
      await loadCollectors(res.audit_run_id);
    } catch (err: unknown) {
      setTriggerMsg(formatError(err, "Falha no disparo"));
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
        eyebrow="EXECUÇÕES"
        title="Execuções de auditoria"
        description={
          <>
            Dispare coletas, acompanhe o progresso por collector e revise erros.
            Configure{" "}
            <code className="rounded bg-slate-800 px-1 text-slate-200">
              {"AUDITOR_TARGET_DSN_<uuid>"}
            </code>{" "}
            no .env para dados reais. O filtro de ambiente da sidebar aplica-se
            à lista.
          </>
        }
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

      <div className="mt-8 space-y-8">
        <Card title="Disparo manual">
          <div className="mt-3 flex flex-col gap-3 sm:flex-row sm:items-end">
            <div className="w-full sm:max-w-md">
              <label className="mb-1 block text-xs text-slate-400">
                Ambiente
                <select
                  className="mt-1 w-full rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400/60"
                  value={triggerEnv}
                  onChange={(e) => setTriggerEnv(e.target.value)}
                >
                  {environments.map((e) => (
                    <option key={e.id} value={e.id}>
                      {e.name}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <Button onClick={onTriggerClick} disabled={busy || !triggerEnv}>
              {busy ? "Disparando…" : "Executar agora"}
            </Button>
          </div>
          {triggerMsg ? (
            <p className="mt-3 text-sm text-slate-300">{triggerMsg}</p>
          ) : null}
        </Card>

        <div className="grid gap-3 sm:grid-cols-2">
          <Input
            label="Filtrar status"
            placeholder="running, success, failed…"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
          />
          <Input
            label="Filtrar profile"
            placeholder="manual, daily…"
            value={profileFilter}
            onChange={(e) => setProfileFilter(e.target.value)}
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
            description="Dispare uma auditoria manual ou aguarde o scheduler. Ambientes demo vêm do seed local."
            action={
              <Button onClick={onTriggerClick} disabled={busy || !triggerEnv}>
                Executar agora
              </Button>
            }
          />
        ) : (
          <Table dense headers={["Profile", "Status", "Duração", "Ambiente"]}>
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
                  <span className="mt-0.5 block text-[10px] text-slate-500">
                    {new Date(r.started_at).toLocaleString()}
                  </span>
                </td>
                <td className="px-3 py-1.5 text-slate-200">{envLabel(r)}</td>
              </tr>
            ))}
          </Table>
        )}

        {selected ? (
          <section className="space-y-4">
            <h2 className="text-lg font-semibold text-slate-100">Detalhe</h2>
            <Card
              title={`${selected.profile} · ${labels.runStatus(selected.status)}`}
              subtitle={`${envLabel(selected)} · ${selected.id.slice(0, 8)}…`}
            >
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
                      ? "A run ainda está iniciando os collectors…"
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
                        <td className="px-3 py-1.5 text-xs text-slate-300">
                          {c.rows_collected}
                        </td>
                        <td className="px-3 py-1.5 text-xs text-slate-400">
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

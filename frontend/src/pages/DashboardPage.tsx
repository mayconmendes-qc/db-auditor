import { type ReactNode, useEffect, useState } from "react";
import { PageHeader } from "../components/PageHeader";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  ErrorBanner,
  Skeleton,
  StoragePieChart,
} from "../components/ui";
import { useApp } from "../context/AppContext";
import { formatError } from "../lib/errors";
import { labels } from "../lib/labels";
import { api } from "../services/api";
import type {
  ConnectionStatus,
  DashboardKPIs,
  FindingsTrendResponse,
  JobHealthResponse,
  StorageGrowthResponse,
} from "../types";

function formatBytes(n: number): string {
  if (n <= 0) {
    return "0 B";
  }
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let v = n;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

function MiniBar({
  label,
  value,
  max,
  tone = "emerald",
  valueLabel,
}: {
  label: string;
  value: number;
  max: number;
  tone?: "emerald" | "amber" | "rose" | "sky";
  valueLabel?: string;
}) {
  const pct = max > 0 ? Math.min(100, Math.round((value / max) * 100)) : 0;
  const colors = {
    emerald: "bg-emerald-500/80",
    amber: "bg-amber-500/80",
    rose: "bg-rose-500/80",
    sky: "bg-sky-500/80",
  };
  return (
    <div className="space-y-1">
      <div className="flex justify-between gap-2 text-xs">
        <span className="truncate text-slate-300" title={label}>
          {label}
        </span>
        <span className="shrink-0 font-mono text-slate-100">
          {valueLabel ?? value}
          {max > 0 ? <span className="ml-1 text-slate-400">{pct}%</span> : null}
        </span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-slate-800">
        <div
          className={`h-full rounded-full ${colors[tone]}`}
          style={{ width: `${pct}%` }}
          role="presentation"
        />
      </div>
    </div>
  );
}

function KpiGroup({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="space-y-2">
      <p className="text-xs font-medium text-slate-400">{title}</p>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">{children}</div>
    </div>
  );
}

export function DashboardPage() {
  const { environmentId, setEnvironmentId, setSection, selectedEnvironment } =
    useApp();
  const [kpis, setKpis] = useState<DashboardKPIs | null>(null);
  const [storage, setStorage] = useState<StorageGrowthResponse | null>(null);
  const [trends, setTrends] = useState<FindingsTrendResponse | null>(null);
  const [jobs, setJobs] = useState<JobHealthResponse | null>(null);
  const [connections, setConnections] = useState<ConnectionStatus[] | null>(
    null,
  );
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    setEnvironmentId(null);
  }, [setEnvironmentId]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      setError(null);
      try {
        const params = environmentId
          ? { environment_id: environmentId }
          : undefined;
        const [k, s, t, j, c] = await Promise.all([
          api.analyticsKpis(params),
          api.analyticsStorage(params),
          api.analyticsFindingsTrends(params),
          api.analyticsJobHealth(params),
          api.connectionStatus(),
        ]);
        if (!cancelled) {
          setKpis(k);
          setStorage(s);
          setTrends(t);
          setJobs(j);
          setConnections(c.items);
        }
      } catch (e) {
        if (!cancelled) {
          setError(
            formatError(
              e,
              "Falha ao carregar o dashboard. Verifique a API e tente novamente.",
            ),
          );
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [environmentId, reloadKey]);

  const consumersMax = storage?.top_consumers?.length
    ? Math.max(1, ...storage.top_consumers.map((p) => p.size_bytes))
    : 1;
  const severityMax = trends
    ? Math.max(1, ...trends.by_severity.map((b) => b.count))
    : 1;
  const statusMax = trends
    ? Math.max(1, ...trends.by_status.map((b) => b.count))
    : 1;
  const typeMax = trends?.by_type?.length
    ? Math.max(1, ...trends.by_type.map((b) => b.count))
    : 1;

  const envName =
    selectedEnvironment?.name ??
    (environmentId ? `${environmentId.slice(0, 8)}…` : null);

  return (
    <>
      <PageHeader
        eyebrow="Dashboard"
        title="Auditoria com evidências, sem mudanças automáticas"
        description="Conexões, KPIs, storage, findings e saúde de jobs. Use o filtro de ambiente na barra lateral quando precisar focar um alvo."
      />

      {environmentId && envName ? (
        <div className="mt-4 flex flex-wrap items-center gap-2 rounded-lg border border-slate-700 bg-slate-900/60 px-3 py-2 text-sm text-slate-200">
          <span>
            Mostrando: <strong className="text-slate-50">{envName}</strong>
          </span>
          <Button
            type="button"
            variant="ghost"
            onClick={() => setEnvironmentId(null)}
          >
            Ver todos
          </Button>
        </div>
      ) : null}

      {error ? (
        <div className="mt-8">
          <ErrorBanner
            message={error}
            onRetry={() => setReloadKey((k) => k + 1)}
          />
        </div>
      ) : null}

      {loading ? (
        <div className="mt-8 space-y-6">
          <Skeleton className="h-12 w-full" />
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-24 w-full" />
            ))}
          </div>
        </div>
      ) : null}

      {!loading && connections ? (
        <section className="mt-8" aria-labelledby="conn-heading">
          <h2 id="conn-heading" className="text-sm font-medium text-slate-300">
            Conexões
          </h2>
          {connections.length === 0 ? (
            <div className="mt-3">
              <EmptyState
                title="Nenhum ambiente configurado"
                description="Configure AUDITOR_TARGET_DSN_* no backend e reinicie a API. Depois valide em Status."
                action={
                  <Button type="button" onClick={() => setSection("Status")}>
                    Ir para Status
                  </Button>
                }
              />
            </div>
          ) : (
            <div className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {connections.map((c) => {
                const ok = c.dsn_configured && c.reachable;
                const statusLabel = ok
                  ? "Conectado"
                  : c.dsn_configured
                    ? "Indisponível"
                    : "DSN ausente";
                return (
                  <Card
                    key={c.environment_id}
                    subtitle="Ambiente auditado"
                    title={c.environment_name}
                  >
                    <div className="mt-1 flex flex-wrap items-center gap-2">
                      <Badge tone={ok ? "success" : "danger"}>
                        {statusLabel}
                      </Badge>
                      {c.server_version ? (
                        <span className="font-mono text-xs text-slate-400">
                          PG {c.server_version}
                          {c.latency_ms != null ? ` · ${c.latency_ms} ms` : ""}
                        </span>
                      ) : null}
                    </div>
                    {c.error ? (
                      <p className="mt-2 truncate text-xs text-rose-300">
                        {c.error}
                      </p>
                    ) : null}
                  </Card>
                );
              })}
            </div>
          )}
        </section>
      ) : null}

      {!loading && kpis ? (
        <div className="mt-8 space-y-6">
          <KpiGroup title="Risco">
            <Card
              subtitle="Findings abertos"
              title={String(kpis.open_findings)}
              onClick={() => setSection("Findings")}
            />
            <Card
              subtitle="Critical / High"
              title={`${kpis.critical_findings} / ${kpis.high_findings}`}
              onClick={() => setSection("Findings")}
            />
            <Card
              subtitle="Runs ok / falha"
              title={`${kpis.successful_runs_recent} / ${kpis.failed_runs_recent}`}
              onClick={() => setSection("Execuções")}
            />
          </KpiGroup>
          <KpiGroup title="Capacidade">
            <Card
              subtitle="Storage total"
              title={formatBytes(kpis.total_storage_bytes)}
              onClick={() => setSection("Inventário")}
            />
            <Card subtitle="Hypertables" title={String(kpis.hypertables)} />
            <Card subtitle="Policies" title={String(kpis.policies)} />
          </KpiGroup>
          <KpiGroup title="Operação">
            <Card
              subtitle="Ambientes"
              title={String(kpis.environments)}
              onClick={() => setSection("Ambientes")}
            />
            <Card
              subtitle="Jobs agendados"
              title={String(kpis.jobs_scheduled)}
            />
          </KpiGroup>
        </div>
      ) : null}

      {!loading ? (
        <div className="mt-8 grid gap-4 lg:grid-cols-2 lg:items-stretch">
          {storage ? (
            <Card title="Storage por ambiente" className="min-h-[22rem]">
              <div className="mt-2 flex flex-1 flex-col">
                <StoragePieChart items={storage.by_environment} />
              </div>
            </Card>
          ) : null}

          {trends ? (
            <Card
              title={`Findings (total ${trends.total})`}
              className="min-h-[22rem]"
            >
              <div className="mt-2 grid flex-1 gap-4 sm:grid-cols-2">
                <div>
                  <p className="mb-2 text-xs font-medium text-slate-400">
                    Severidade
                  </p>
                  <ul className="space-y-2.5">
                    {trends.by_severity.map((b) => (
                      <li key={b.key}>
                        <MiniBar
                          label={labels.severity(b.key)}
                          value={b.count}
                          max={severityMax}
                          tone={
                            b.key === "critical" || b.key === "high"
                              ? "rose"
                              : b.key === "medium"
                                ? "amber"
                                : "emerald"
                          }
                        />
                      </li>
                    ))}
                  </ul>
                </div>
                <div>
                  <p className="mb-2 text-xs font-medium text-slate-400">
                    Status
                  </p>
                  <ul className="space-y-2.5">
                    {trends.by_status.map((b) => (
                      <li key={b.key}>
                        <MiniBar
                          label={labels.findingStatus(b.key)}
                          value={b.count}
                          max={statusMax}
                          tone="amber"
                        />
                      </li>
                    ))}
                  </ul>
                </div>
              </div>
            </Card>
          ) : null}

          {storage?.top_consumers && storage.top_consumers.length > 0 ? (
            <Card
              title="Maiores consumidores (tabelas)"
              className="min-h-[22rem]"
            >
              <ul className="mt-1 min-h-0 flex-1 space-y-2.5 overflow-y-auto pr-1">
                {storage.top_consumers.slice(0, 12).map((p) => (
                  <li key={`${p.object_kind}-${p.label}`}>
                    <MiniBar
                      label={p.label}
                      value={p.size_bytes}
                      max={consumersMax}
                      tone="sky"
                      valueLabel={formatBytes(p.size_bytes)}
                    />
                  </li>
                ))}
              </ul>
            </Card>
          ) : null}

          {trends?.by_type && trends.by_type.length > 0 ? (
            <Card title="Findings por tipo" className="min-h-[22rem]">
              <ul className="mt-1 min-h-0 flex-1 space-y-2.5 overflow-y-auto pr-1">
                {trends.by_type.slice(0, 12).map((b) => (
                  <li key={b.key}>
                    <MiniBar
                      label={b.key}
                      value={b.count}
                      max={typeMax}
                      tone="emerald"
                    />
                  </li>
                ))}
              </ul>
            </Card>
          ) : null}
        </div>
      ) : null}

      {!loading && jobs ? (
        <section className="mt-8" aria-labelledby="jobs-heading">
          <h2 id="jobs-heading" className="text-sm font-medium text-slate-300">
            Saúde de jobs / policies
          </h2>
          {jobs.items.length === 0 ? (
            <p className="mt-2 text-sm text-slate-400">
              Nenhum ambiente com dados de jobs. Dispare uma coleta em
              Execuções.
            </p>
          ) : (
            <div className="mt-3 overflow-x-auto rounded-md border border-slate-800">
              <table className="w-full min-w-[28rem] text-left text-sm">
                <thead className="sticky top-0 bg-slate-900/95 text-xs text-slate-400">
                  <tr>
                    <th className="px-3 py-2 font-medium">Ambiente</th>
                    <th className="px-3 py-2 font-medium">Jobs</th>
                    <th className="px-3 py-2 font-medium">Agendados</th>
                    <th className="px-3 py-2 font-medium">Policies</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800 text-slate-300">
                  {jobs.items.map((item) => (
                    <tr key={item.environment_id}>
                      <td className="px-3 py-2">
                        {item.environment_name ||
                          item.environment_id.slice(0, 8)}
                      </td>
                      <td className="px-3 py-2 font-mono text-xs">
                        {item.jobs_total}
                      </td>
                      <td className="px-3 py-2 font-mono text-xs">
                        {item.jobs_scheduled}
                      </td>
                      <td className="px-3 py-2 font-mono text-xs">
                        {item.policies_total}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      ) : null}
    </>
  );
}

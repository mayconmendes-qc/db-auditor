import { useEffect, useState } from "react";
import { Card } from "../components/ui";
import { api } from "../services/api";
import type {
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

/** Executive dashboard (Sprint 11). */
export function DashboardPage() {
  const [kpis, setKpis] = useState<DashboardKPIs | null>(null);
  const [storage, setStorage] = useState<StorageGrowthResponse | null>(null);
  const [trends, setTrends] = useState<FindingsTrendResponse | null>(null);
  const [jobs, setJobs] = useState<JobHealthResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [envFilter, setEnvFilter] = useState("");

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      setError(null);
      try {
        const params = envFilter ? { environment_id: envFilter } : undefined;
        const [k, s, t, j] = await Promise.all([
          api.analyticsKpis(params),
          api.analyticsStorage(params),
          api.analyticsFindingsTrends(params),
          api.analyticsJobHealth(params),
        ]);
        if (!cancelled) {
          setKpis(k);
          setStorage(s);
          setTrends(t);
          setJobs(j);
        }
      } catch (e) {
        if (!cancelled) {
          setError(
            e instanceof Error ? e.message : "Falha ao carregar dashboard",
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
  }, [envFilter]);

  return (
    <>
      <p className="text-xs font-bold tracking-[0.12em] text-emerald-300">
        DASHBOARD
      </p>
      <h1 className="mt-2 text-3xl font-semibold text-slate-50 md:text-4xl">
        Visão executiva
      </h1>
      <p className="mt-3 max-w-2xl text-sm leading-relaxed text-slate-400">
        KPIs, storage, findings e saúde de jobs — apenas via API analítica.
      </p>

      <div className="mt-6 flex flex-wrap items-end gap-3">
        <label className="grid gap-1 text-xs text-slate-400">
          Filtro environment_id (opcional)
          <input
            className="rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-100"
            value={envFilter}
            onChange={(e) => setEnvFilter(e.target.value.trim())}
            placeholder="uuid do ambiente"
            aria-label="Filtro de ambiente"
          />
        </label>
      </div>

      {loading && (
        <p className="mt-8 text-sm text-slate-400">Carregando KPIs…</p>
      )}
      {error && (
        <div
          className="mt-8 rounded-md border border-rose-800/60 bg-rose-950/40 px-4 py-3 text-sm text-rose-200"
          role="alert"
        >
          {error}
        </div>
      )}

      {kpis && (
        <div className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Card subtitle="Ambientes" title={String(kpis.environments)} />
          <Card
            subtitle="Findings abertos"
            title={String(kpis.open_findings)}
          />
          <Card
            subtitle="Critical / High"
            title={`${kpis.critical_findings} / ${kpis.high_findings}`}
          />
          <Card
            subtitle="Storage total"
            title={formatBytes(kpis.total_storage_bytes)}
          />
          <Card subtitle="Hypertables" title={String(kpis.hypertables)} />
          <Card subtitle="Jobs agendados" title={String(kpis.jobs_scheduled)} />
          <Card subtitle="Policies" title={String(kpis.policies)} />
          <Card
            subtitle="Runs ok / falha"
            title={`${kpis.successful_runs_recent} / ${kpis.failed_runs_recent}`}
          />
        </div>
      )}

      {storage && (
        <section className="mt-10" aria-labelledby="storage-heading">
          <h2
            id="storage-heading"
            className="text-sm font-semibold text-slate-200"
          >
            Storage por ambiente
          </h2>
          {storage.by_environment.length === 0 ? (
            <p className="mt-2 text-sm text-slate-500">Sem dados de storage.</p>
          ) : (
            <ul className="mt-3 space-y-2">
              {storage.by_environment.map((p) => (
                <li
                  key={p.label}
                  className="flex justify-between rounded-md border border-slate-800 px-3 py-2 text-sm"
                >
                  <span className="text-slate-300">{p.label}</span>
                  <span className="font-mono text-slate-100">
                    {formatBytes(p.size_bytes)}
                  </span>
                </li>
              ))}
            </ul>
          )}
          {storage.top_consumers.length > 0 && (
            <>
              <h3 className="mt-6 text-xs font-semibold uppercase tracking-wide text-slate-500">
                Top consumers (tabelas)
              </h3>
              <ul className="mt-2 space-y-1 text-sm text-slate-400">
                {storage.top_consumers.slice(0, 8).map((p) => (
                  <li key={p.label} className="flex justify-between gap-4">
                    <span className="truncate font-mono text-xs">
                      {p.label}
                    </span>
                    <span>{formatBytes(p.size_bytes)}</span>
                  </li>
                ))}
              </ul>
            </>
          )}
        </section>
      )}

      {trends && (
        <section className="mt-10" aria-labelledby="findings-heading">
          <h2
            id="findings-heading"
            className="text-sm font-semibold text-slate-200"
          >
            Findings por severidade / status (total {trends.total})
          </h2>
          <div className="mt-3 grid gap-4 sm:grid-cols-2">
            <div className="rounded-md border border-slate-800 p-3">
              <p className="text-xs uppercase text-slate-500">Severidade</p>
              <ul className="mt-2 space-y-1 text-sm">
                {trends.by_severity.map((b) => (
                  <li
                    key={b.key}
                    className="flex justify-between text-slate-300"
                  >
                    <span>{b.key || "(vazio)"}</span>
                    <span>{b.count}</span>
                  </li>
                ))}
              </ul>
            </div>
            <div className="rounded-md border border-slate-800 p-3">
              <p className="text-xs uppercase text-slate-500">Status</p>
              <ul className="mt-2 space-y-1 text-sm">
                {trends.by_status.map((b) => (
                  <li
                    key={b.key}
                    className="flex justify-between text-slate-300"
                  >
                    <span>{b.key || "(vazio)"}</span>
                    <span>{b.count}</span>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </section>
      )}

      {jobs && (
        <section className="mt-10" aria-labelledby="jobs-heading">
          <h2
            id="jobs-heading"
            className="text-sm font-semibold text-slate-200"
          >
            Saúde de jobs / policies
          </h2>
          {jobs.items.length === 0 ? (
            <p className="mt-2 text-sm text-slate-500">Nenhum ambiente.</p>
          ) : (
            <div className="mt-3 overflow-x-auto">
              <table className="w-full min-w-[28rem] text-left text-sm">
                <thead className="text-xs uppercase text-slate-500">
                  <tr>
                    <th className="pb-2 pr-3 font-medium">Ambiente</th>
                    <th className="pb-2 pr-3 font-medium">Jobs</th>
                    <th className="pb-2 pr-3 font-medium">Agendados</th>
                    <th className="pb-2 font-medium">Policies</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800 text-slate-300">
                  {jobs.items.map((item) => (
                    <tr key={item.environment_id}>
                      <td className="py-2 pr-3">
                        {item.environment_name ||
                          item.environment_id.slice(0, 8)}
                      </td>
                      <td className="py-2 pr-3">{item.jobs_total}</td>
                      <td className="py-2 pr-3">{item.jobs_scheduled}</td>
                      <td className="py-2">{item.policies_total}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      )}
    </>
  );
}

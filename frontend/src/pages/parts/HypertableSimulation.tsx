import { useEffect, useMemo, useState } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Button, ErrorBanner, Input, Skeleton } from "../../components/ui";
import { formatError, isAbortError } from "../../lib/errors";
import { formatBytes } from "../../lib/format";
import { api } from "../../services/api";
import type {
  HypertableSimulationResponse,
  UnscannedChunksResponse,
} from "../../types";

export function HypertableSimulation({
  environment,
  run,
  database,
  schema,
  name,
  canSave,
}: {
  environment: string;
  run: string;
  database: string;
  schema: string;
  name: string;
  canSave: boolean;
}) {
  const [retentionDays, setRetentionDays] = useState("90");
  const [compressAfter, setCompressAfter] = useState("7");
  const [data, setData] = useState<HypertableSimulationResponse | null>(null);
  const [unscanned, setUnscanned] = useState<UnscannedChunksResponse | null>(
    null,
  );
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saveMsg, setSaveMsg] = useState<string | null>(null);

  const scope = useMemo(
    () => [environment, run, database, schema, name].join("|"),
    [environment, run, database, schema, name],
  );

  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    void (async () => {
      try {
        const sim = await api.hypertableSimulation(
          environment,
          run,
          database,
          schema,
          name,
          {
            retention_days: Number(retentionDays) || undefined,
            compress_after_days: Number(compressAfter) || undefined,
          },
          controller.signal,
        );
        if (cancelled) return;
        setData(sim);
        if (sim.applicable) {
          const u = await api.hypertableUnscannedChunks(
            environment,
            run,
            database,
            schema,
            name,
            controller.signal,
          );
          if (!cancelled) setUnscanned(u);
        } else {
          setUnscanned(null);
        }
      } catch (err) {
        if (!cancelled && !isAbortError(err)) setError(formatError(err));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [scope, retentionDays, compressAfter]);

  const chartData = (data?.scenarios ?? []).map((s) => ({
    name: s.label,
    atual: (data?.current_bytes ?? 0) / 1e9,
    apos: s.bytes_after / 1e9,
    d30: s.projection_30_gb,
    d90: s.projection_90_gb,
    d180: s.projection_180_gb,
  }));

  const save = async () => {
    if (!canSave || !data?.applicable) return;
    setSaving(true);
    setSaveMsg(null);
    try {
      await api.saveTimescaleScenario(environment, {
        database_name: database,
        schema_name: schema,
        hypertable_name: name,
        label: `ret-${retentionDays}+comp-${compressAfter}`,
        retention_days: Number(retentionDays) || 0,
        compress_after_days: Number(compressAfter) || 0,
        audit_run_id: run,
      });
      setSaveMsg(
        "Cenário salvo. Achados de conflito com CAGG, se houver, foram registrados na execução de referência.",
      );
    } catch (err) {
      setError(formatError(err));
    } finally {
      setSaving(false);
    }
  };

  if (loading && !data) {
    return <Skeleton className="h-40 w-full" />;
  }
  if (error) {
    return <ErrorBanner message={error} />;
  }
  if (!data?.applicable) {
    return (
      <p role="status" className="text-sm text-slate-300">
        {data?.reason ?? "não aplicável"} — esta simulação exige TimescaleDB
        observado na execução.
      </p>
    );
  }

  return (
    <div className="space-y-4">
      <p className="text-sm text-slate-300">
        <strong>Hipótese:</strong> {data.gain.hypothesis}{" "}
        <strong>Ganho previsto:</strong> {data.gain.predicted}{" "}
        <strong>Ganho medido:</strong> {data.gain.measured}
      </p>
      {data.partial ? (
        <p role="status" className="text-sm text-amber-200">
          Resultado parcial: inventário de chunks truncado.
        </p>
      ) : null}
      {!data.retention_applicable ? (
        <p role="status" className="text-sm text-slate-300">
          Retenção por idade não aplicável (dimensão inteira sem função de
          &quot;agora&quot;).
        </p>
      ) : null}
      <div className="flex flex-wrap items-end gap-3">
        <Input
          label="Retenção (dias)"
          type="number"
          min={1}
          value={retentionDays}
          onChange={(e) => setRetentionDays(e.target.value)}
        />
        <Input
          label="Comprimir após (dias)"
          type="number"
          min={1}
          value={compressAfter}
          onChange={(e) => setCompressAfter(e.target.value)}
        />
        {canSave ? (
          <Button type="button" onClick={() => void save()} disabled={saving}>
            Salvar cenário
          </Button>
        ) : null}
      </div>
      {saveMsg ? (
        <p role="status" className="text-sm text-emerald-200">
          {saveMsg}
        </p>
      ) : null}
      <p className="text-sm text-slate-300">
        Atual: {formatBytes(data.current_bytes)}; comprimido{" "}
        {data.compressed_pct.toFixed(1)}%; taxa observada{" "}
        {data.observed_ratio.toFixed(2)}×
        {data.ratio_confidence_low
          ? " (sem taxa observada, confiança baixa)"
          : ""}
        {data.currency
          ? `; preço ${data.currency}${data.price_effective_from ? ` desde ${new Date(data.price_effective_from).toLocaleDateString("pt-BR")}` : ""}`
          : ""}
      </p>
      <div className="overflow-x-auto">
        <table className="min-w-full text-left text-sm text-slate-200">
          <thead>
            <tr className="border-b border-slate-700 text-slate-400">
              <th className="py-2 pr-3">Cenário</th>
              <th className="py-2 pr-3">Após</th>
              <th className="py-2 pr-3">Economia</th>
              <th className="py-2 pr-3">Moeda</th>
              <th className="py-2 pr-3">Riscos</th>
            </tr>
          </thead>
          <tbody>
            {data.scenarios.map((s) => (
              <tr key={s.label} className="border-b border-slate-800">
                <td className="py-2 pr-3">{s.label}</td>
                <td className="py-2 pr-3">{formatBytes(s.bytes_after)}</td>
                <td className="py-2 pr-3">{s.gb_saved.toFixed(2)} GB</td>
                <td className="py-2 pr-3">
                  {s.money_saved != null && data.currency
                    ? `${data.currency} ${s.money_saved.toFixed(2)}/mês`
                    : "—"}
                </td>
                <td className="py-2 pr-3">
                  {s.risks.length ? s.risks.join(", ") : "—"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {chartData.length ? (
        <div
          className="h-64 w-full"
          role="img"
          aria-label="Projeção de tamanho em GB por cenário (atual, após, 30, 90 e 180 dias)"
        >
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
              <XAxis dataKey="name" stroke="#94a3b8" />
              <YAxis stroke="#94a3b8" unit=" GB" />
              <Tooltip />
              <Legend />
              <Bar dataKey="atual" name="Atual" fill="#64748b" />
              <Bar dataKey="apos" name="Após cenário" fill="#38bdf8" />
              <Bar dataKey="d30" name="Proj. 30d" fill="#22c55e" />
              <Bar dataKey="d90" name="Proj. 90d" fill="#eab308" />
              <Bar dataKey="d180" name="Proj. 180d" fill="#f97316" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      ) : null}
      {data.age_buckets?.length ? (
        <ul className="text-sm text-slate-300">
          {data.age_buckets.map((b) => (
            <li key={b.label}>
              Faixa {b.label}: {b.count} chunks, {formatBytes(b.bytes)}
            </li>
          ))}
        </ul>
      ) : null}
      {unscanned ? (
        <div className="rounded border border-slate-700 bg-slate-900/50 p-3 text-sm text-slate-300">
          <p>
            Chunks possivelmente sem consulta (
            <strong>{unscanned.label}</strong>
            ):
          </p>
          {!unscanned.comparable ? (
            <p>
              Janela não comparável: {unscanned.reason || "sem coleta anterior"}
              .
            </p>
          ) : unscanned.items.length ? (
            <ul className="mt-1 list-disc pl-5">
              {unscanned.items.slice(0, 20).map((c) => (
                <li key={c}>{c}</li>
              ))}
            </ul>
          ) : (
            <p>Nenhum candidato na janela comparável.</p>
          )}
        </div>
      ) : null}
    </div>
  );
}

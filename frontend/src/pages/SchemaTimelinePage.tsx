import { useCallback, useEffect, useState } from "react";
import {
  Badge,
  Button,
  EmptyState,
  ErrorBanner,
  Skeleton,
  Table,
} from "../components/ui";
import { useApp } from "../context/AppContext";
import { formatError } from "../lib/errors";
import { api } from "../services/api";
import type { SchemaTimelinePage as TimelinePage } from "../types";

function correlationTone(
  correlation: string,
): "success" | "warning" | "danger" | "neutral" {
  if (correlation === "none") return "danger";
  if (correlation === "not_verified") return "warning";
  if (correlation === "deployment" || correlation === "migration_registry") {
    return "success";
  }
  return "neutral";
}

export function SchemaTimelinePage() {
  const { environmentId, environments, setEnvironmentId } = useApp();
  const [env, setEnv] = useState(environmentId ?? environments[0]?.id ?? "");
  const [page, setPage] = useState<TimelinePage | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(() => {
    if (!env) return;
    setBusy(true);
    setError(null);
    void api
      .schemaTimeline(env, { limit: 100 })
      .then((res) => setPage(res))
      .catch((err: unknown) => {
        setError(formatError(err, "Falha ao carregar a linha do tempo"));
        setPage(null);
      })
      .finally(() => setBusy(false));
  }, [env]);

  useEffect(() => {
    load();
  }, [load]);

  const exportUrl = (format: "csv" | "md") =>
    `/api/v1/environments/${env}/schema-timeline/export?format=${format}`;

  return (
    <>
      <p className="text-xs font-bold tracking-[0.12em] text-emerald-300">
        LINHA DO TEMPO
      </p>
      <h1 className="mt-2 text-3xl font-semibold text-slate-50 md:text-4xl">
        Linha do tempo de schema
      </h1>
      <p className="mt-3 max-w-3xl text-sm leading-relaxed text-slate-400">
        Eventos gravados na ingestão entre coletas completas. Sem recalcular o
        diff na leitura. Mudanças sem implantação aparecem como fora do
        pipeline.
      </p>

      <div className="mt-8 space-y-6">
        <div className="flex flex-wrap items-end gap-3">
          <label className="block text-sm text-slate-300">
            Ambiente
            <select
              className="mt-1 block w-64 rounded border border-slate-700 bg-slate-900 px-3 py-2"
              value={env}
              onChange={(e) => {
                setEnv(e.target.value);
                setEnvironmentId(e.target.value);
              }}
            >
              {environments.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name}
                  {item.is_production ? " · produção" : ""}
                </option>
              ))}
            </select>
          </label>
          <Button onClick={load} disabled={busy}>
            Atualizar
          </Button>
          <a
            className="text-sm text-emerald-300 underline"
            href={exportUrl("csv")}
          >
            Exportar CSV
          </a>
          <a
            className="text-sm text-emerald-300 underline"
            href={exportUrl("md")}
          >
            Exportar Markdown
          </a>
        </div>

        {error ? <ErrorBanner message={error} onRetry={load} /> : null}
        {busy ? <Skeleton className="h-40 w-full" /> : null}

        {page && !busy && page.gaps.length > 0 ? (
          <div className="rounded border border-amber-900/50 bg-amber-950/30 p-4 text-sm text-amber-100">
            <p className="font-medium">Lacunas</p>
            <ul className="mt-2 space-y-1">
              {page.gaps.map((g) => (
                <li key={g.id}>
                  {g.gap_from} → {g.gap_to}: {g.reason}
                </li>
              ))}
            </ul>
          </div>
        ) : null}

        {page && !busy ? (
          page.events.length === 0 ? (
            <EmptyState
              title="Sem eventos"
              description="Nenhuma mudança registrada na janela (padrão 90 dias)."
            />
          ) : (
            <Table
              headers={[
                "Quando",
                "Objeto",
                "Mudança",
                "Correlação",
                "Fingerprints",
              ]}
            >
              {page.events.map((e) => (
                <tr key={e.id} className="border-t border-slate-800">
                  <td className="px-4 py-3 text-slate-300 text-xs">
                    {e.detected_at}
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-slate-100">
                    {e.object_type} · {e.object_key}
                  </td>
                  <td className="px-4 py-3 text-slate-200">{e.change_kind}</td>
                  <td className="px-4 py-3">
                    <Badge tone={correlationTone(e.correlation)}>
                      {e.correlation === "none"
                        ? "fora do pipeline"
                        : e.correlation}
                    </Badge>
                    {e.registry_version ? (
                      <span className="ml-2 text-xs text-slate-400">
                        mig {e.registry_version}
                      </span>
                    ) : null}
                  </td>
                  <td className="px-4 py-3 font-mono text-[11px] text-slate-400">
                    {(e.prior_fingerprint || "∅").slice(0, 12)} →{" "}
                    {(e.current_fingerprint || "∅").slice(0, 12)}
                  </td>
                </tr>
              ))}
            </Table>
          )
        ) : null}
      </div>
    </>
  );
}

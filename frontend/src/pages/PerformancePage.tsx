import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  ErrorBanner,
  Skeleton,
  Table,
} from "../components/ui";
import { formatError } from "../lib/errors";
import { labels } from "../lib/labels";
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

export function PerformancePage() {
  const [items, setItems] = useState<Finding[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [selected, setSelected] = useState<Finding | null>(null);

  const load = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      const res = await api.findings({ status: "open", limit: 200 });
      const perf = res.items.filter(
        (f) =>
          f.finding_type.startsWith("performance.") ||
          f.finding_type.startsWith("vacuum."),
      );
      setItems(perf);
    } catch (err: unknown) {
      setError(formatError(err, "Falha ao listar performance"));
      setItems([]);
    } finally {
      setBusy(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const summary = useMemo(() => {
    const by = (prefix: string) =>
      items.filter((f) => f.finding_type.startsWith(prefix)).length;
    return {
      vacuum: by("vacuum."),
      locks: by("performance.lock"),
      connections: by("performance.high_connections"),
      slow: by("performance.slow_query"),
    };
  }, [items]);

  return (
    <>
      <p className="text-xs font-bold tracking-[0.12em] text-emerald-300">
        PERFORMANCE
      </p>
      <h1 className="mt-2 text-3xl font-semibold text-slate-50 md:text-4xl">
        Performance
      </h1>
      <p className="mt-3 max-w-3xl text-sm leading-relaxed text-slate-400">
        Vacuum / dead tuples, lock waits, pressão de conexões e fingerprints de
        queries lentas. SQL sensível nunca é exposto — apenas fingerprints
        normalizados.
      </p>

      <div className="mt-8 space-y-6">
        <div className="grid gap-3 sm:grid-cols-4">
          <Card title="Vacuum" subtitle={String(summary.vacuum)} />
          <Card title="Lock waits" subtitle={String(summary.locks)} />
          <Card title="Conexões" subtitle={String(summary.connections)} />
          <Card title="Slow queries" subtitle={String(summary.slow)} />
        </div>

        <div className="flex flex-wrap gap-2">
          <Button onClick={() => void load()} disabled={busy}>
            Atualizar
          </Button>
        </div>

        {error ? (
          <ErrorBanner message={error} onRetry={() => void load()} />
        ) : null}
        {busy ? <Skeleton className="h-40 w-full" /> : null}

        {!busy && items.length === 0 ? (
          <EmptyState
            title="Sem findings de performance"
            description="Execute uma auditoria para analisar vacuum, locks e queries."
          />
        ) : null}

        {!busy && items.length > 0 ? (
          <Table
            headers={["Tipo", "Severidade", "Título", "Objeto", "Última vez"]}
          >
            {items.map((f) => (
              <tr
                key={f.id}
                className="border-t border-slate-800 cursor-pointer"
                onClick={() => setSelected(f)}
              >
                <td className="px-4 py-3 text-slate-300 font-mono text-xs">
                  {f.finding_type}
                </td>
                <td className="px-4 py-3">
                  <Badge tone={severityTone(f.severity)}>
                    {labels.severity(f.severity)}
                  </Badge>
                </td>
                <td className="px-4 py-3 text-slate-100">{f.title}</td>
                <td className="px-4 py-3 text-slate-400 font-mono text-xs">
                  {f.object_key || "—"}
                </td>
                <td className="px-4 py-3 text-slate-400 text-xs">
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
              <li>Objeto: {selected.object_key || "—"}</li>
              <li>Resumo: {selected.summary}</li>
            </ul>
            <p className="mt-3 text-xs text-amber-300">
              Evidências sanitizadas. O auditor nunca executa VACUUM, TERMINATE
              ou alterações automáticas.
            </p>
            {selected.evidence ? (
              <pre className="mt-3 overflow-auto rounded bg-slate-900 p-3 text-xs text-slate-400">
                {JSON.stringify(selected.evidence, null, 2)}
              </pre>
            ) : null}
          </Card>
        ) : null}
      </div>
    </>
  );
}

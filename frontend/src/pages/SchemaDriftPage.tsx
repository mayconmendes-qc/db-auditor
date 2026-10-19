import { useEffect, useState } from "react";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  ErrorBanner,
  Input,
  Skeleton,
  Table,
} from "../components/ui";
import { useApp } from "../context/AppContext";
import { formatError } from "../lib/errors";
import { api } from "../services/api";
import type { AuditRun, ObjectDiff, SchemaDiffResult } from "../types";

function statusTone(
  status: string,
): "success" | "warning" | "danger" | "neutral" {
  if (status === "MATCH") return "success";
  if (status === "DRIFT" || status === "UNKNOWN") return "warning";
  if (status === "ONLY_SOURCE" || status === "ONLY_TARGET") return "danger";
  return "neutral";
}

export function SchemaDriftPage() {
  const { environmentId, environments, setSection } = useApp();
  const [sourceEnv, setSourceEnv] = useState(environmentId ?? "");
  const [targetEnv, setTargetEnv] = useState(environmentId ?? "");
  const [sourceRun, setSourceRun] = useState("");
  const [targetRun, setTargetRun] = useState("");
  const [sourceRuns, setSourceRuns] = useState<AuditRun[]>([]);
  const [targetRuns, setTargetRuns] = useState<AuditRun[]>([]);
  const [statusFilter, setStatusFilter] = useState("");
  const [typeFilter, setTypeFilter] = useState("");
  const [result, setResult] = useState<SchemaDiffResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [selected, setSelected] = useState<ObjectDiff | null>(null);

  useEffect(() => {
    if (environmentId && !sourceEnv) setSourceEnv(environmentId);
  }, [environmentId, sourceEnv]);

  useEffect(() => {
    if (!sourceEnv) return;
    void api
      .auditRuns({ environment_id: sourceEnv, status: "success" })
      .then((res) => {
        setSourceRuns(res.items);
        if (!sourceRun && res.items[0]) setSourceRun(res.items[0].id);
      })
      .catch(() => setSourceRuns([]));
  }, [sourceEnv, sourceRun]);

  useEffect(() => {
    if (!targetEnv) return;
    void api
      .auditRuns({ environment_id: targetEnv, status: "success" })
      .then((res) => {
        setTargetRuns(res.items);
        if (!targetRun && res.items[0]) setTargetRun(res.items[0].id);
      })
      .catch(() => setTargetRuns([]));
  }, [targetEnv, targetRun]);

  const runCompare = async () => {
    if (!sourceEnv) return;
    setBusy(true);
    setError(null);
    setSelected(null);
    try {
      const res = await api.schemaDiff(sourceEnv, {
        source_run: sourceRun || undefined,
        target_environment_id: targetEnv || sourceEnv,
        target_run: targetRun || undefined,
        object_type: typeFilter || undefined,
        status: statusFilter || undefined,
        limit: 100,
      });
      setResult(res);
    } catch (err: unknown) {
      const msg = formatError(err, "Falha na comparação");
      if (String(msg).includes("403") || String(msg).includes("autoriz")) {
        setError("Acesso não autorizado aos dois ambientes.");
      } else {
        setError(msg);
      }
      setResult(null);
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <p className="text-xs font-bold tracking-[0.12em] text-emerald-300">
        DESVIO DE SCHEMA
      </p>
      <h1 className="mt-2 text-3xl font-semibold text-slate-50 md:text-4xl">
        Desvio de schema
      </h1>
      <p className="mt-3 max-w-3xl text-sm leading-relaxed text-slate-400">
        Compare snapshots reais entre ambientes ou execuções. Definições SQL
        aparecem só como fingerprint.
      </p>

      <div className="mt-8 space-y-6">
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block text-sm text-slate-300">
            Ambiente origem
            <select
              className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-3 py-2"
              value={sourceEnv}
              onChange={(e) => {
                setSourceEnv(e.target.value);
                setSourceRun("");
              }}
            >
              {environments.map((e) => (
                <option key={e.id} value={e.id}>
                  {e.name}
                </option>
              ))}
            </select>
          </label>
          <label className="block text-sm text-slate-300">
            Ambiente destino
            <select
              className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-3 py-2"
              value={targetEnv}
              onChange={(e) => {
                setTargetEnv(e.target.value);
                setTargetRun("");
              }}
            >
              {environments.map((e) => (
                <option key={e.id} value={e.id}>
                  {e.name}
                </option>
              ))}
            </select>
          </label>
          <label className="block text-sm text-slate-300">
            Execução origem
            <select
              className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-3 py-2"
              value={sourceRun}
              onChange={(e) => setSourceRun(e.target.value)}
            >
              <option value="">Última completa</option>
              {sourceRuns.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.started_at} · {r.status}
                </option>
              ))}
            </select>
          </label>
          <label className="block text-sm text-slate-300">
            Execução destino
            <select
              className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-3 py-2"
              value={targetRun}
              onChange={(e) => setTargetRun(e.target.value)}
            >
              <option value="">Última completa</option>
              {targetRuns.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.started_at} · {r.status}
                </option>
              ))}
            </select>
          </label>
          <Input
            label="Filtrar status"
            placeholder="MATCH,DRIFT,ONLY_SOURCE"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
          />
          <Input
            label="Filtrar object_type"
            placeholder="table, index, function…"
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
          />
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            onClick={() => void runCompare()}
            disabled={busy || !sourceEnv}
          >
            {busy ? "Comparando…" : "Comparar"}
          </Button>
          <Button variant="ghost" onClick={() => setSection("Linha do tempo")}>
            Abrir linha do tempo
          </Button>
        </div>

        {error ? (
          <ErrorBanner message={error} onRetry={() => void runCompare()} />
        ) : null}

        {result?.partial ? (
          <p className="text-sm text-amber-300" role="status">
            Diff marcado como parcial: uma das execuções não está completa.
            Achados de drift não são gerados neste caso.
          </p>
        ) : null}

        {result ? (
          <div className="grid gap-3 sm:grid-cols-5">
            <Card title="MATCH" subtitle={String(result.summary.match)} />
            <Card title="DRIFT" subtitle={String(result.summary.drift)} />
            <Card
              title="ONLY_SOURCE"
              subtitle={String(result.summary.only_source)}
            />
            <Card
              title="ONLY_TARGET"
              subtitle={String(result.summary.only_target)}
            />
            <Card title="UNKNOWN" subtitle={String(result.summary.unknown)} />
          </div>
        ) : null}

        {busy ? <Skeleton className="h-40 w-full" /> : null}

        {result && !busy ? (
          result.objects.length === 0 ? (
            <EmptyState
              title="Sem diferenças"
              description="Nenhum objeto após filtros."
            />
          ) : (
            <Table headers={["Tipo", "Key", "Status", "Origem", "Destino"]}>
              {result.objects.map((o) => (
                <tr
                  key={`${o.object_type}-${o.object_key}-${o.status}`}
                  className="border-t border-slate-800 cursor-pointer"
                  onClick={() => setSelected(o)}
                >
                  <td className="px-4 py-3 text-slate-100">{o.object_type}</td>
                  <td className="px-4 py-3 text-slate-300 font-mono text-xs">
                    {o.object_key}
                  </td>
                  <td className="px-4 py-3">
                    <Badge tone={statusTone(o.status)}>{o.status}</Badge>
                  </td>
                  <td className="px-4 py-3 text-slate-300">
                    {o.source_name ?? "—"}
                  </td>
                  <td className="px-4 py-3 text-slate-300">
                    {o.target_name ?? "—"}
                  </td>
                </tr>
              ))}
            </Table>
          )
        ) : null}

        {selected ? (
          <Card
            title={`Detalhe · ${selected.status}`}
            subtitle={selected.object_key}
          >
            <ul className="mt-3 space-y-1 text-sm text-slate-300">
              <li>Tipo: {selected.object_type}</li>
              <li>Source FP: {selected.source_fingerprint ?? "—"}</li>
              <li>Target FP: {selected.target_fingerprint ?? "—"}</li>
            </ul>
            {selected.field_diffs && selected.field_diffs.length > 0 ? (
              <div className="mt-4">
                <h3 className="text-sm font-medium text-slate-200">
                  Field diffs
                </h3>
                <ul className="mt-2 space-y-1 text-xs font-mono text-slate-400">
                  {selected.field_diffs.map((f) => (
                    <li key={f.field}>
                      {f.field}: {f.source ?? "∅"} → {f.target ?? "∅"}
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
            <p className="mt-4 text-xs text-slate-400">
              Use Inventário com a execução e o objeto selecionados para
              detalhar a estrutura.
            </p>
          </Card>
        ) : null}
      </div>
    </>
  );
}

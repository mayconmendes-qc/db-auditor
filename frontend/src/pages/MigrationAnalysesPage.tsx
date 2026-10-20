import { useEffect, useState } from "react";
import {
  Badge,
  EmptyState,
  ErrorBanner,
  Skeleton,
  Table,
} from "../components/ui";
import { formatError } from "../lib/errors";
import { api } from "../services/api";
import type {
  Environment,
  MigrationAnalysis,
  MigrationAnalysisItem,
} from "../types";

function severityTone(s: string): "neutral" | "success" | "warning" | "danger" {
  switch (s) {
    case "critical":
    case "high":
      return "danger";
    case "medium":
      return "warning";
    case "low":
    case "info":
      return "success";
    default:
      return "neutral";
  }
}

export function MigrationAnalysesPage() {
  const [envs, setEnvs] = useState<Environment[] | null>(null);
  const [envId, setEnvId] = useState("");
  const [items, setItems] = useState<MigrationAnalysis[] | null>(null);
  const [detail, setDetail] = useState<{
    analysis: MigrationAnalysis;
    items: MigrationAnalysisItem[];
  } | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    api
      .environments()
      .then((res) => {
        if (cancelled) return;
        setEnvs(res.items);
        if (res.items.length && !envId) setEnvId(res.items[0].id);
      })
      .catch((err: unknown) =>
        setError(formatError(err, "Não foi possível listar ambientes")),
      );
    return () => {
      cancelled = true;
    };
  }, [envId]);

  useEffect(() => {
    if (!envId) return;
    let cancelled = false;
    setItems(null);
    setDetail(null);
    api
      .listMigrationAnalyses(envId)
      .then((res) => {
        if (!cancelled) setItems(res.items);
      })
      .catch((err: unknown) =>
        setError(formatError(err, "Não foi possível listar análises")),
      );
    return () => {
      cancelled = true;
    };
  }, [envId]);

  return (
    <>
      <p className="text-xs font-medium text-slate-400">Análises</p>
      <h1 className="mt-2 text-3xl font-semibold text-slate-50 md:text-4xl">
        Análises de migration
      </h1>
      <p className="mt-3 max-w-3xl text-sm leading-relaxed text-slate-400">
        Histórico enviado pela CLI (`--submit`). Sem texto SQL nem literais — só
        metadados de risco (MR-*).
      </p>
      {error ? <ErrorBanner message={error} /> : null}
      <div className="mt-4 max-w-md">
        <label className="block text-sm text-slate-300" htmlFor="mr-env">
          Ambiente
        </label>
        <select
          id="mr-env"
          className="mt-1 w-full rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-slate-100"
          value={envId}
          onChange={(e) => setEnvId(e.target.value)}
        >
          {(envs ?? []).map((e) => (
            <option key={e.id} value={e.id}>
              {e.name}
            </option>
          ))}
        </select>
      </div>
      <div className="mt-6 space-y-6">
        {items === null ? (
          <Skeleton className="h-40 w-full" />
        ) : items.length === 0 ? (
          <EmptyState
            title="Nenhuma análise"
            description="Execute auditor migration-risk --submit no CI ou localmente."
          />
        ) : (
          <Table
            headers={[
              { id: "when", label: "Data" },
              { id: "repo", label: "Repositório" },
              { id: "pr", label: "PR" },
              { id: "sev", label: "Maior severidade" },
              { id: "mode", label: "Contexto" },
            ]}
          >
            {items.map((a) => (
              <tr
                key={a.id}
                className="cursor-pointer border-t border-slate-800 hover:bg-slate-900/60"
                onClick={() => {
                  void api
                    .getMigrationAnalysis(envId, a.id)
                    .then(setDetail)
                    .catch((err: unknown) =>
                      setError(formatError(err, "Falha ao abrir detalhe")),
                    );
                }}
              >
                <td className="px-4 py-3 text-slate-300">
                  {new Date(a.created_at).toLocaleString()}
                </td>
                <td className="px-4 py-3 text-slate-100">
                  {a.repository || "—"}
                </td>
                <td className="px-4 py-3 text-slate-300">{a.pr || "—"}</td>
                <td className="px-4 py-3">
                  <Badge tone={severityTone(a.highest_severity)}>
                    {a.highest_severity}
                  </Badge>
                </td>
                <td className="px-4 py-3 text-slate-300">{a.context_mode}</td>
              </tr>
            ))}
          </Table>
        )}
        {detail ? (
          <section className="space-y-3">
            <h2 className="text-xl font-semibold text-slate-100">
              Detalhe — {detail.analysis.repository || detail.analysis.id}
            </h2>
            <Table
              headers={[
                { id: "loc", label: "Arquivo:linha" },
                { id: "code", label: "Código" },
                { id: "op", label: "Operação" },
                { id: "obj", label: "Objeto" },
                { id: "lock", label: "Lock" },
                { id: "sev", label: "Severidade" },
              ]}
            >
              {detail.items.map((it, i) => (
                <tr
                  key={`${it.file}:${it.line}:${i}`}
                  className="border-t border-slate-800"
                >
                  <td className="px-4 py-3 text-slate-300">
                    {it.file}:{it.line}
                  </td>
                  <td className="px-4 py-3 text-slate-100">{it.code}</td>
                  <td className="px-4 py-3 text-slate-300">{it.operation}</td>
                  <td className="px-4 py-3 text-slate-300">{it.object}</td>
                  <td className="px-4 py-3 text-slate-300">{it.lock}</td>
                  <td className="px-4 py-3">
                    <Badge
                      tone={severityTone(it.suppressed ? "info" : it.severity)}
                    >
                      {it.suppressed ? "suprimida" : it.severity}
                    </Badge>
                  </td>
                </tr>
              ))}
            </Table>
          </section>
        ) : null}
      </div>
    </>
  );
}

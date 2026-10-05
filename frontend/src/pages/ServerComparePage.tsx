import { useEffect, useState } from "react";
import { PageHeader } from "../components/PageHeader";
import { Button, Card, EmptyState, ErrorBanner } from "../components/ui";
import { useApp } from "../context/AppContext";
import { formatError } from "../lib/errors";
import { api } from "../services/api";
import type { ServerCompareReport } from "../types";

export function ServerComparePage() {
  const { environments, search, setSearch } = useApp();
  const [left, setLeft] = useState(search.left || "");
  const [right, setRight] = useState(search.right || "");
  const [report, setReport] = useState<ServerCompareReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = async (nextLeft = left, nextRight = right) => {
    if (!nextLeft || !nextRight || nextLeft === nextRight) return;
    setBusy(true);
    setError(null);
    try {
      setReport(await api.serverCompare(nextLeft, nextRight));
      setSearch({ left: nextLeft, right: nextRight });
    } catch (err: unknown) {
      setReport(null);
      setError(formatError(err, "Não foi possível comparar os snapshots"));
    } finally {
      setBusy(false);
    }
  };

  useEffect(() => {
    if (search.left && search.right) {
      setLeft(search.left);
      setRight(search.right);
      void load(search.left, search.right);
    }
    // Reload only when the shared URL changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search.left, search.right]);

  return (
    <>
      <PageHeader
        eyebrow="Snapshots"
        title="Comparar servidores"
        description="Lado a lado a partir do último snapshot de cada ambiente. Nenhuma sessão é aberta no banco auditado. shared_buffers diferente é informativo."
      />
      <div className="mt-6 space-y-4">
        <Card>
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="text-sm text-slate-300">
              Esquerda
              <select
                className="mt-1 w-full rounded border border-slate-600 bg-slate-900 px-3 py-2"
                value={left}
                onChange={(event) => setLeft(event.target.value)}
              >
                <option value="">Escolha</option>
                {environments.map((env) => (
                  <option key={env.id} value={env.id}>
                    {env.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="text-sm text-slate-300">
              Direita
              <select
                className="mt-1 w-full rounded border border-slate-600 bg-slate-900 px-3 py-2"
                value={right}
                onChange={(event) => setRight(event.target.value)}
              >
                <option value="">Escolha</option>
                {environments.map((env) => (
                  <option key={env.id} value={env.id}>
                    {env.name}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <Button
            className="mt-3"
            disabled={busy || !left || !right || left === right}
            onClick={() => void load()}
          >
            Comparar snapshots
          </Button>
        </Card>
        {error ? <ErrorBanner message={error} /> : null}
        {report && report.rows.length === 0 ? (
          <EmptyState title="Nenhuma diferença nos conjuntos fechados" />
        ) : null}
        {report && report.rows.length > 0 ? (
          <div className="overflow-auto rounded border border-slate-700">
            <table className="min-w-full text-left text-sm">
              <thead className="bg-slate-900 text-slate-400">
                <tr>
                  <th className="px-3 py-2">Tipo</th>
                  <th className="px-3 py-2">Nome</th>
                  <th className="px-3 py-2">Esquerda</th>
                  <th className="px-3 py-2">Direita</th>
                  <th className="px-3 py-2">Status</th>
                </tr>
              </thead>
              <tbody>
                {report.rows.map((row) => (
                  <tr
                    key={`${row.kind}:${row.name}:${row.status}`}
                    className="border-t border-slate-800"
                  >
                    <td className="px-3 py-2">{row.kind}</td>
                    <td className="px-3 py-2">{row.name}</td>
                    <td className="px-3 py-2">{row.left || "—"}</td>
                    <td className="px-3 py-2">{row.right || "—"}</td>
                    <td className="px-3 py-2">
                      {row.status}
                      {row.informational ? " · informativo" : ""}
                      {row.detail ? ` · ${row.detail}` : ""}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </div>
    </>
  );
}

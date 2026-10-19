import { useEffect, useState } from "react";
import { Button, EmptyState, Skeleton, Table } from "../../components/ui";
import { formatError } from "../../lib/errors";
import { api } from "../../services/api";
import type { CostRankingResponse, CostSeriesPoint } from "../../types";

export function DashboardCost({
  environmentId,
}: {
  environmentId: string | null;
}) {
  const [ranking, setRanking] = useState<CostRankingResponse | null>(null);
  const [series, setSeries] = useState<CostSeriesPoint[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!environmentId) {
      setRanking(null);
      setSeries(null);
      return;
    }
    let cancelled = false;
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    (async () => {
      try {
        const [r, s] = await Promise.all([
          api.costRanking(environmentId, {}, controller.signal),
          api.costSeries(environmentId, {}, controller.signal),
        ]);
        if (!cancelled) {
          setRanking(r);
          setSeries(s.items);
        }
      } catch (e) {
        if (!cancelled) {
          setError(
            formatError(e, "Não foi possível carregar o custo estimado"),
          );
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [environmentId]);

  if (!environmentId) {
    return (
      <EmptyState
        title="Selecione um ambiente"
        description="O ranking de custo é por ambiente."
      />
    );
  }
  if (loading) return <Skeleton className="h-40 w-full" />;
  if (error) return <p className="text-sm text-rose-300">{error}</p>;

  const items = ranking?.items ?? [];
  const reason = ranking?.reason;

  return (
    <section className="space-y-4" aria-labelledby="cost-heading">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2
            id="cost-heading"
            className="text-lg font-semibold text-slate-100"
          >
            Custo estimado
          </h2>
          <p className="text-sm text-slate-400">
            Valores rotulados estimado/mês. Não substituem a fatura real.
          </p>
        </div>
        <Button
          type="button"
          variant="secondary"
          onClick={() => {
            if (!environmentId) return;
            void api.costRankingExport(environmentId).then((blob) => {
              const url = URL.createObjectURL(blob);
              const a = document.createElement("a");
              a.href = url;
              a.download = "cost-ranking.csv";
              a.click();
              URL.revokeObjectURL(url);
            });
          }}
        >
          Exportar CSV
        </Button>
      </div>
      {!ranking?.priced ? (
        <p className="rounded-md border border-amber-700/50 bg-amber-950/30 px-3 py-2 text-sm text-amber-100">
          {reason || "não estimado — preço não configurado"}
        </p>
      ) : null}
      {items.length === 0 ? (
        <EmptyState
          title="Sem itens de custo"
          description={
            reason ||
            "Sem coleta completa ou sem desperdícios acima dos limiares."
          }
        />
      ) : (
        <Table
          dense
          headers={[
            { id: "type", label: "Tipo" },
            { id: "object", label: "Objeto" },
            { id: "value", label: "Valor/mês" },
            { id: "basis", label: "Base" },
            { id: "confidence", label: "Confiança" },
          ]}
        >
          {items.map((item) => (
            <tr
              key={`${item.type}-${item.object_key}-${item.fingerprint ?? ""}`}
            >
              <td className="px-3 py-2">{item.type}</td>
              <td className="px-3 py-2 font-mono text-xs">
                {item.object_key || item.fingerprint || "—"}
              </td>
              <td className="px-3 py-2">
                {item.priced && item.month_amount != null
                  ? `${item.currency ?? ""} ${item.month_amount.toFixed(2)} estimado/mês`
                  : item.reason || item.label || "não estimado"}
              </td>
              <td className="px-3 py-2 text-xs text-slate-400">
                {item.formula || "—"}
              </td>
              <td className="px-3 py-2">{item.confidence}</td>
            </tr>
          ))}
        </Table>
      )}
      <div>
        <h3 className="mb-2 text-sm font-medium text-slate-300">
          Série (coletas completas)
        </h3>
        {(series ?? []).length === 0 ? (
          <p className="text-sm text-slate-500">
            Sem pontos na janela de 90 dias.
          </p>
        ) : (
          <ul className="space-y-1 text-sm text-slate-300">
            {(series ?? []).map((p) => (
              <li key={p.audit_run_id}>
                {p.at}: armazenamento {p.storage_month.toFixed(2)} · índice{" "}
                {p.unused_index_month.toFixed(2)} · bloat{" "}
                {p.bloat_month.toFixed(2)} · consulta {p.query_month.toFixed(2)}{" "}
                {p.currency} {!p.priced ? "(não estimado)" : ""}
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}

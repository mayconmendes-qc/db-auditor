import { useEffect, useState } from "react";
import { Button, Input, Skeleton } from "../../components/ui";
import { formatError } from "../../lib/errors";
import { api } from "../../services/api";
import type {
  CostPricesResponse,
  CostPriceTemplate,
  CostPriceVersion,
} from "../../types";

export function CostPricesCard({ environmentId }: { environmentId: string }) {
  const canEdit = api.hasRole("operator");
  const [data, setData] = useState<CostPricesResponse | null>(null);
  const [templates, setTemplates] = useState<CostPriceTemplate[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [currency, setCurrency] = useState("BRL");
  const [storage, setStorage] = useState("0.50");
  const [model, setModel] = useState("self_hosted");
  const [saving, setSaving] = useState(false);

  const reload = () => {
    setData(null);
    void Promise.all([
      api.costPrices(environmentId),
      api.costTemplates(environmentId),
    ])
      .then(([prices, tpl]) => {
        setData(prices);
        setTemplates(tpl.items);
        if (prices.currency) setCurrency(prices.currency);
        else if (prices.current) setCurrency(prices.current.currency);
      })
      .catch((err: unknown) =>
        setError(formatError(err, "Não foi possível carregar preços")),
      );
  };

  useEffect(() => {
    reload();
  }, [environmentId]);

  const locked = Boolean(data?.currency_locked);

  return (
    <div className="mt-8 space-y-3 rounded-lg border border-slate-800 p-4">
      <h3 className="text-lg font-semibold text-slate-100">
        Tabela de preços (custo)
      </h3>
      <p className="text-sm text-slate-400">
        Uma moeda por ambiente (BRL ou USD), travada após a primeira versão.
        Valores são estimados/mês.
      </p>
      {error ? <p className="text-sm text-rose-300">{error}</p> : null}
      {!data ? (
        <Skeleton className="h-24 w-full" />
      ) : (
        <>
          {data.current ? (
            <p className="text-sm text-slate-300">
              Vigente v{data.current.version}: {data.current.currency}{" "}
              {data.current.storage_gb_month}/GB-mês · modelo{" "}
              {data.current.model_origin} · desde {data.current.effective_from}
            </p>
          ) : (
            <p className="text-sm text-amber-200">Preço não configurado.</p>
          )}
          {canEdit ? (
            <form
              className="flex flex-wrap items-end gap-2"
              onSubmit={(event) => {
                event.preventDefault();
                setSaving(true);
                setError(null);
                void api
                  .updateCostPrices(environmentId, {
                    currency,
                    storage_gb_month: Number(storage),
                    model_origin: model,
                    effective_from: new Date().toISOString(),
                  })
                  .then(() => reload())
                  .catch((err: unknown) =>
                    setError(
                      formatError(err, "Não foi possível gravar a versão"),
                    ),
                  )
                  .finally(() => setSaving(false));
              }}
            >
              <label className="text-xs text-slate-400">
                Moeda
                <select
                  className="mt-1 block rounded-md border border-slate-700 bg-slate-900 px-2 py-1 text-sm"
                  value={currency}
                  disabled={locked}
                  onChange={(e) => setCurrency(e.target.value)}
                >
                  <option value="BRL">BRL</option>
                  <option value="USD">USD</option>
                </select>
              </label>
              <Input
                label="Armazenamento / GB-mês"
                value={storage}
                onChange={(e) => setStorage(e.target.value)}
                className="mt-1"
              />
              <label className="text-xs text-slate-400">
                Modelo
                <select
                  className="mt-1 block rounded-md border border-slate-700 bg-slate-900 px-2 py-1 text-sm"
                  value={model}
                  onChange={(e) => setModel(e.target.value)}
                >
                  <option value="rds_postgresql">RDS PostgreSQL</option>
                  <option value="cloud_sql">Cloud SQL</option>
                  <option value="self_hosted">Self-hosted</option>
                  <option value="custom">Custom</option>
                </select>
              </label>
              <Button type="submit" disabled={saving}>
                Nova versão
              </Button>
            </form>
          ) : (
            <p className="text-sm text-slate-400">
              Somente operator pode criar novas versões de preço.
            </p>
          )}
          {canEdit && templates.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {templates.map((t) => (
                <Button
                  key={t.id}
                  type="button"
                  variant="secondary"
                  onClick={() => {
                    if (!locked) setCurrency(t.currency);
                    setStorage(String(t.storage_gb_month));
                    setModel(t.model_origin);
                  }}
                >
                  Usar {t.name}
                </Button>
              ))}
            </div>
          ) : null}
          {data.history.length > 1 ? (
            <details className="text-sm text-slate-400">
              <summary>Histórico ({data.history.length})</summary>
              <ul className="mt-2 space-y-1">
                {data.history.map((v: CostPriceVersion) => (
                  <li key={v.id}>
                    v{v.version} · {v.currency} {v.storage_gb_month}/GB-mês ·{" "}
                    {v.effective_from}
                  </li>
                ))}
              </ul>
            </details>
          ) : null}
        </>
      )}
    </div>
  );
}

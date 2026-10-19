import { useEffect, useState } from "react";
import { Button, Card, Select } from "../../components/ui";
import { formatError } from "../../lib/errors";
import { api } from "../../services/api";
import type { QualityContract, QualityContractTrendPoint } from "../../types";

export function QualityContractsPanel({
  environmentId,
  enabled,
  database,
  schema,
  table,
  onError,
}: {
  environmentId: string;
  enabled: boolean;
  database: string;
  schema: string;
  table: string;
  onError: (message: string) => void;
}) {
  const [items, setItems] = useState<QualityContract[]>([]);
  const [busy, setBusy] = useState(false);
  const [selected, setSelected] = useState<string>("");
  const [trend, setTrend] = useState<QualityContractTrendPoint[]>([]);
  const canWrite = api.hasRole("auditor");

  const refresh = async () => {
    try {
      const res = await api.qualityContracts(environmentId);
      setItems(res.items);
    } catch (err) {
      onError(formatError(err));
    }
  };

  useEffect(() => {
    void refresh();
  }, [environmentId]);

  const propose = async () => {
    if (!database || !schema || !table) {
      onError("Informe banco, esquema e tabela para gerar a proposta.");
      return;
    }
    setBusy(true);
    try {
      await api.proposeQualityContract(environmentId, {
        database,
        schema,
        table,
      });
      await refresh();
    } catch (err) {
      onError(formatError(err));
    } finally {
      setBusy(false);
    }
  };

  const approve = async (contractId: string, versionId: string) => {
    setBusy(true);
    try {
      await api.approveQualityContractVersion(
        environmentId,
        contractId,
        versionId,
      );
      await refresh();
    } catch (err) {
      onError(formatError(err));
    } finally {
      setBusy(false);
    }
  };

  const run = async (contractId: string) => {
    setBusy(true);
    try {
      await api.runQualityContract(environmentId, contractId);
      await refresh();
    } catch (err) {
      onError(formatError(err));
    } finally {
      setBusy(false);
    }
  };

  const loadTrend = async (contractId: string) => {
    setSelected(contractId);
    try {
      const res = await api.qualityContractTrend(environmentId, contractId);
      setTrend(res.items);
    } catch (err) {
      onError(formatError(err));
    }
  };

  return (
    <Card
      title="Qualidade — contratos"
      subtitle="Contratos versionados por tabela: proposta, aprovação, execução e tendência. Só contagens são guardadas."
    >
      {!enabled ? (
        <p className="mt-3 text-sm text-amber-300">
          Recurso desabilitado. Peça ao operator para definir
          AUDITOR_DATA_QUALITY_ENABLED=1.
        </p>
      ) : null}
      <div className="mt-3 flex flex-wrap gap-2">
        {canWrite ? (
          <Button disabled={busy || !enabled} onClick={() => void propose()}>
            Gerar proposta
          </Button>
        ) : (
          <p className="text-sm text-slate-400">
            Viewer: leitura e tendência; aprovação e execução exigem auditor.
          </p>
        )}
        <Button disabled={busy} onClick={() => void refresh()}>
          Atualizar
        </Button>
      </div>
      {items.length === 0 ? (
        <p className="mt-3 text-sm text-slate-400">Nenhum contrato ainda.</p>
      ) : (
        <div className="mt-4 space-y-3">
          {items.map((c) => {
            const v = c.latest_version;
            return (
              <div
                key={c.id}
                className="rounded border border-slate-700 p-3 text-sm"
              >
                <p className="font-medium">
                  {c.database}.{c.schema}.{c.table}
                </p>
                <p className="text-slate-400">
                  Versão {v?.version_number ?? "—"} · {v?.status ?? "—"} ·
                  amostra {v?.sample_limit ?? "—"} · checagens{" "}
                  {v?.document?.checks?.length ?? 0}
                </p>
                <p className="text-slate-400">
                  Última execução:{" "}
                  {c.last_run
                    ? `${c.last_run.status} (${c.last_run.mode}) · ${new Date(c.last_run.started_at).toLocaleString("pt-BR")}`
                    : "nenhuma"}
                </p>
                <div className="mt-2 flex flex-wrap gap-2">
                  {canWrite && v?.status === "proposed" ? (
                    <Button
                      disabled={busy || !enabled}
                      onClick={() => void approve(c.id, v.id)}
                    >
                      Aprovar
                    </Button>
                  ) : null}
                  {canWrite && v?.status === "approved" ? (
                    <Button
                      disabled={busy || !enabled}
                      onClick={() => void run(c.id)}
                    >
                      Executar
                    </Button>
                  ) : null}
                  <Button onClick={() => void loadTrend(c.id)}>Tendência</Button>
                </div>
              </div>
            );
          })}
        </div>
      )}
      {selected ? (
        <div className="mt-4">
          <Select
            label="Contrato na tendência"
            value={selected}
            onChange={(e) => void loadTrend(e.target.value)}
            options={items.map((c) => ({
              value: c.id,
              label: `${c.database}.${c.schema}.${c.table}`,
            }))}
          />
          <p className="mt-2 text-xs text-slate-400">
            Margem de erro amostral desconhecida. Lacunas aparecem quando a
            execução foi parcial ou a checagem não rodou.
          </p>
          <ul className="mt-2 max-h-48 space-y-1 overflow-auto text-xs text-slate-300">
            {trend.map((t) => (
              <li key={`${t.run_id}-${t.check_key}-${t.started_at}`}>
                {new Date(t.started_at).toLocaleString("pt-BR")} · {t.check_key}{" "}
                · {t.result}
                {t.observed_pct != null ? ` · ${t.observed_pct.toFixed(2)}%` : ""}
                {t.gap ? " · lacuna" : ""}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </Card>
  );
}

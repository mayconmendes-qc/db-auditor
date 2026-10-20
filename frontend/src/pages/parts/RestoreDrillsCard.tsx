import { useEffect, useState } from "react";
import { Button, Input, Skeleton } from "../../components/ui";
import { formatError } from "../../lib/errors";
import { api } from "../../services/api";
import type { RestoreDrill, RestoreDrillPolicy } from "../../types";

function formatRTO(seconds?: number | null): string {
  if (seconds == null) return "n/d";
  if (seconds < 60) return `${seconds}s`;
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return s ? `${m}m ${s}s` : `${m}m`;
}

export function RestoreDrillsCard({
  environmentId,
}: {
  environmentId: string;
}) {
  const canEdit = api.hasRole("operator");
  const [items, setItems] = useState<RestoreDrill[] | null>(null);
  const [total, setTotal] = useState(0);
  const [policy, setPolicy] = useState<RestoreDrillPolicy | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [rtoTarget, setRtoTarget] = useState("600");
  const [maxAge, setMaxAge] = useState("30");
  const [saving, setSaving] = useState(false);

  const reload = () => {
    setItems(null);
    setError(null);
    void Promise.all([
      api.restoreDrills(environmentId),
      api.restoreDrillPolicy(environmentId),
    ])
      .then(([drills, pol]) => {
        setItems(drills.items);
        setTotal(drills.total);
        setPolicy(pol);
        setRtoTarget(String(pol.rto_target_seconds));
        setMaxAge(String(pol.max_age_days));
      })
      .catch((err: unknown) =>
        setError(
          formatError(err, "Não foi possível carregar ensaios de restauração"),
        ),
      );
  };

  useEffect(() => {
    reload();
  }, [environmentId]);

  const lastSuccess = items?.find((d) => d.succeeded) ?? null;
  const last = items?.[0] ?? null;

  return (
    <div className="mt-8 space-y-3 rounded-lg border border-slate-800 p-4">
      <h3 className="text-lg font-semibold text-slate-100">
        Restauração verificada
      </h3>
      <p className="text-sm text-slate-400">
        Ensaios CLI{" "}
        <code className="text-slate-300">auditor restore-verify</code> do backup
        do banco auditado (pg_dump). Evidência{" "}
        <code className="text-slate-300">target_restore_drill</code>, separada
        da restauração do snapshot store.
      </p>
      {error ? <p className="text-sm text-rose-300">{error}</p> : null}
      {!items || !policy ? (
        <Skeleton className="h-24 w-full" />
      ) : (
        <>
          <div className="grid gap-2 text-sm text-slate-300 sm:grid-cols-3">
            <div>
              <div className="text-xs text-slate-500">Último ensaio</div>
              {last ? (
                <div>
                  {last.succeeded ? "sucesso" : "falha"} ·{" "}
                  {new Date(last.performed_at).toLocaleString()}
                </div>
              ) : (
                <div className="text-amber-200">nenhum</div>
              )}
            </div>
            <div>
              <div className="text-xs text-slate-500">Último sucesso / RTO</div>
              {lastSuccess ? (
                <div>
                  {formatRTO(lastSuccess.rto_seconds)} (alvo{" "}
                  {formatRTO(policy.rto_target_seconds)})
                </div>
              ) : (
                <div className="text-amber-200">sem sucesso</div>
              )}
            </div>
            <div>
              <div className="text-xs text-slate-500">Histórico</div>
              <div>{total} ensaio(s)</div>
            </div>
          </div>

          {items.length > 0 ? (
            <div className="overflow-x-auto">
              <table className="min-w-full text-left text-sm text-slate-300">
                <thead className="text-xs uppercase text-slate-500">
                  <tr>
                    <th className="px-2 py-1">Quando</th>
                    <th className="px-2 py-1">Estado</th>
                    <th className="px-2 py-1">RTO</th>
                    <th className="px-2 py-1">Divergências</th>
                  </tr>
                </thead>
                <tbody>
                  {items.slice(0, 10).map((d) => {
                    const divs = Array.isArray(d.divergences)
                      ? d.divergences.length
                      : 0;
                    return (
                      <tr key={d.id} className="border-t border-slate-800">
                        <td className="px-2 py-1">
                          {new Date(d.performed_at).toLocaleString()}
                        </td>
                        <td className="px-2 py-1">
                          {d.succeeded ? (
                            <span className="text-emerald-300">sucesso</span>
                          ) : (
                            <span className="text-rose-300">falha</span>
                          )}
                        </td>
                        <td className="px-2 py-1">
                          {formatRTO(d.rto_seconds)}
                        </td>
                        <td className="px-2 py-1">
                          {divs > 0 ? (
                            <a
                              className="text-sky-300 underline"
                              href={`#/findings?environment=${environmentId}&type=backup.restore_divergence`}
                            >
                              {divs}
                            </a>
                          ) : (
                            "0"
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          ) : (
            <p className="text-sm text-slate-400">
              Ainda não há ensaios. Rode{" "}
              <code className="text-slate-300">auditor restore-verify</code> no
              runner com --confirm-isolated-target e --submit.
            </p>
          )}

          {canEdit ? (
            <form
              className="flex flex-wrap items-end gap-2 border-t border-slate-800 pt-3"
              onSubmit={(event) => {
                event.preventDefault();
                setSaving(true);
                setError(null);
                void api
                  .updateRestoreDrillPolicy(environmentId, {
                    rto_target_seconds: Number(rtoTarget),
                    max_age_days: Number(maxAge),
                  })
                  .then(() => reload())
                  .catch((err: unknown) =>
                    setError(
                      formatError(err, "Não foi possível gravar a política"),
                    ),
                  )
                  .finally(() => setSaving(false));
              }}
            >
              <Input
                label="RTO-alvo (s)"
                value={rtoTarget}
                onChange={(e) => setRtoTarget(e.target.value)}
                className="mt-1"
              />
              <Input
                label="Prazo máx. entre ensaios (dias)"
                value={maxAge}
                onChange={(e) => setMaxAge(e.target.value)}
                className="mt-1"
              />
              <Button type="submit" disabled={saving}>
                {saving ? "Salvando…" : "Salvar política"}
              </Button>
            </form>
          ) : null}
        </>
      )}
    </div>
  );
}

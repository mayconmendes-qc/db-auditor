import { useEffect, useMemo, useState } from "react";
import { Badge, Button, Card, Input, Skeleton, Table } from "../components/ui";
import { api } from "../services/api";
import type { AuditRun, CollectorRun, Environment } from "../types";

function statusTone(
  status: string,
): "success" | "warning" | "danger" | "neutral" {
  if (status === "success") {
    return "success";
  }
  if (status === "partial_success" || status === "running") {
    return "warning";
  }
  if (status === "failed") {
    return "danger";
  }
  return "neutral";
}

export function AuditRunsPage() {
  const [envs, setEnvs] = useState<Environment[] | null>(null);
  const [runs, setRuns] = useState<AuditRun[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState("");
  const [profileFilter, setProfileFilter] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [collectors, setCollectors] = useState<CollectorRun[] | null>(null);
  const [triggerEnv, setTriggerEnv] = useState("");
  const [triggerMsg, setTriggerMsg] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const loadRuns = () => {
    setRuns(null);
    api
      .auditRuns({
        status: statusFilter || undefined,
        profile: profileFilter || undefined,
      })
      .then((res) => setRuns(res.items))
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : "Falha ao listar runs");
        setRuns([]);
      });
  };

  useEffect(() => {
    api
      .environments()
      .then((res) => {
        setEnvs(res.items);
        if (res.items.length > 0) {
          setTriggerEnv(res.items[0].id);
        }
      })
      .catch(() => setEnvs([]));
  }, []);

  useEffect(() => {
    loadRuns();
  }, [statusFilter, profileFilter]);

  useEffect(() => {
    if (!selectedId) {
      setCollectors(null);
      return;
    }
    setCollectors(null);
    api
      .auditRunCollectors(selectedId)
      .then((res) => setCollectors(res.items))
      .catch(() => setCollectors([]));
  }, [selectedId]);

  const selected = useMemo(
    () => runs?.find((r) => r.id === selectedId) ?? null,
    [runs, selectedId],
  );

  const onTrigger = async () => {
    if (!triggerEnv) {
      return;
    }
    if (!window.confirm("Disparar auditoria manual neste ambiente?")) {
      return;
    }
    setBusy(true);
    setTriggerMsg(null);
    try {
      const res = await api.triggerAuditRun(triggerEnv, "manual");
      setTriggerMsg(`Run ${res.audit_run_id} → ${res.status}`);
      loadRuns();
    } catch (err: unknown) {
      setTriggerMsg(err instanceof Error ? err.message : "Falha no disparo");
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <p className="text-xs font-bold tracking-[0.12em] text-emerald-300">
        SPRINT 3
      </p>
      <h1 className="mt-2 text-3xl font-semibold text-slate-50 md:text-4xl">
        Audit Runs
      </h1>
      <p className="mt-3 max-w-2xl text-slate-400">
        Execuções de auditoria, collectors e disparo manual via API.
      </p>

      <div className="mt-8 space-y-8">
        <Card title="Disparo manual">
          <div className="mt-3 flex flex-col gap-3 sm:flex-row sm:items-end">
            <div className="w-full sm:max-w-md">
              <label className="mb-1 block text-xs text-slate-400">
                Ambiente
              </label>
              <select
                className="w-full rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-100"
                value={triggerEnv}
                onChange={(e) => setTriggerEnv(e.target.value)}
              >
                {(envs ?? []).map((e) => (
                  <option key={e.id} value={e.id}>
                    {e.name}
                  </option>
                ))}
              </select>
            </div>
            <Button onClick={onTrigger} disabled={busy || !triggerEnv}>
              {busy ? "Executando…" : "Executar agora"}
            </Button>
          </div>
          {triggerMsg ? (
            <p className="mt-3 text-sm text-slate-300">{triggerMsg}</p>
          ) : null}
        </Card>

        <div className="grid gap-3 sm:grid-cols-2">
          <Input
            label="Filtrar status"
            placeholder="success, failed…"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
          />
          <Input
            label="Filtrar profile"
            placeholder="manual, daily…"
            value={profileFilter}
            onChange={(e) => setProfileFilter(e.target.value)}
          />
        </div>

        {error ? <Card title="Erro" subtitle={error} /> : null}

        {runs === null ? (
          <Skeleton className="h-40 w-full" />
        ) : runs.length === 0 ? (
          <Card
            title="Nenhum audit run"
            subtitle="Dispare uma auditoria manual ou aguarde o scheduler."
          />
        ) : (
          <Table headers={["Profile", "Status", "Início", "Ambiente"]}>
            {runs.map((r) => (
              <tr
                key={r.id}
                className={`border-t border-slate-800 cursor-pointer ${
                  selectedId === r.id ? "bg-slate-900/80" : ""
                }`}
                onClick={() => setSelectedId(r.id)}
              >
                <td className="px-4 py-3 text-slate-100">{r.profile}</td>
                <td className="px-4 py-3">
                  <Badge tone={statusTone(r.status)}>{r.status}</Badge>
                </td>
                <td className="px-4 py-3 text-slate-300">
                  {new Date(r.started_at).toLocaleString()}
                </td>
                <td className="px-4 py-3 text-slate-300 font-mono text-xs">
                  {r.environment_id.slice(0, 8)}…
                </td>
              </tr>
            ))}
          </Table>
        )}

        {selected ? (
          <section className="space-y-4">
            <h2 className="text-xl font-semibold text-slate-100">Detalhe</h2>
            <Card
              title={`${selected.profile} · ${selected.status}`}
              subtitle={`id ${selected.id}`}
            >
              <ul className="mt-3 space-y-1 text-sm text-slate-300">
                <li>
                  Início: {new Date(selected.started_at).toLocaleString()}
                </li>
                <li>
                  Fim:{" "}
                  {selected.finished_at
                    ? new Date(selected.finished_at).toLocaleString()
                    : "—"}
                </li>
                <li>Warnings: {selected.warnings.length}</li>
                <li>Errors: {selected.errors.length}</li>
              </ul>
            </Card>

            <h3 className="text-sm font-medium text-slate-300">Collectors</h3>
            {collectors === null ? (
              <Skeleton className="h-32 w-full" />
            ) : collectors.length === 0 ? (
              <Card title="Sem collectors" subtitle="Nenhum registro." />
            ) : (
              <Table headers={["Nome", "Status", "Rows"]}>
                {collectors.map((c) => (
                  <tr key={c.id} className="border-t border-slate-800">
                    <td className="px-4 py-3 text-slate-100">
                      {c.collector_name}
                    </td>
                    <td className="px-4 py-3">
                      <Badge tone={statusTone(c.status)}>{c.status}</Badge>
                    </td>
                    <td className="px-4 py-3 text-slate-300">
                      {c.rows_collected}
                    </td>
                  </tr>
                ))}
              </Table>
            )}
          </section>
        ) : null}
      </div>
    </>
  );
}

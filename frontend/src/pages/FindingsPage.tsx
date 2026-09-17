import { useCallback, useEffect, useState } from "react";
import { Badge, Button, Card, Input, Skeleton, Table } from "../components/ui";
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
  if (severity === "low" || severity === "info") {
    return "neutral";
  }
  return "neutral";
}

function statusTone(
  status: string,
): "success" | "warning" | "danger" | "neutral" {
  if (status === "open") {
    return "danger";
  }
  if (status === "acknowledged") {
    return "warning";
  }
  if (status === "resolved" || status === "suppressed") {
    return "success";
  }
  return "neutral";
}

/** Demo facts so analyze works without live inventory load. */
const demoFacts = {
  environment_id: "00000000-0000-0000-0000-000000000001",
  tables: [
    {
      database: "app",
      schema: "public",
      name: "events",
      size_bytes: 3_000_000_000,
    },
    {
      database: "app",
      schema: "public",
      name: "orders",
      size_bytes: 50_000_000,
    },
  ],
  indexes: [
    {
      database: "app",
      schema: "public",
      table_name: "orders",
      index_name: "idx_orders_legacy",
      idx_scan: 0,
      size_bytes: 12_000_000,
    },
  ],
  hypertables: [
    {
      database: "app",
      schema: "public",
      name: "metrics",
      num_chunks: 620,
      size_bytes: 8_000_000_000,
    },
  ],
  chunks: [
    {
      database: "app",
      schema: "public",
      hypertable_name: "metrics",
      chunk_name: "_hyper_1_1",
      size_bytes: 100_000_000,
    },
    {
      database: "app",
      schema: "public",
      hypertable_name: "metrics",
      chunk_name: "_hyper_1_2",
      size_bytes: 100_000_000,
    },
    {
      database: "app",
      schema: "public",
      hypertable_name: "metrics",
      chunk_name: "_hyper_1_3",
      size_bytes: 900_000_000,
    },
  ],
};

export function FindingsPage() {
  const [items, setItems] = useState<Finding[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [selected, setSelected] = useState<Finding | null>(null);
  const [typeFilter, setTypeFilter] = useState("");
  const [severityFilter, setSeverityFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("open");

  const load = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      const res = await api.findings({
        finding_type: typeFilter || undefined,
        severity: severityFilter || undefined,
        status: statusFilter || undefined,
      });
      setItems(res.items);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Falha ao listar findings");
      setItems([]);
    } finally {
      setBusy(false);
    }
  }, [typeFilter, severityFilter, statusFilter]);

  useEffect(() => {
    void load();
  }, [load]);

  const runAnalyze = async () => {
    setBusy(true);
    setError(null);
    try {
      await api.analyzeFindings(demoFacts);
      await load();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Falha no analyze");
    } finally {
      setBusy(false);
    }
  };

  const triage = async (id: string, status: string) => {
    try {
      const updated = await api.updateFindingStatus(id, status);
      setItems((prev) => prev.map((f) => (f.id === id ? updated : f)));
      setSelected(updated);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Falha na triagem");
    }
  };

  return (
    <>
      <p className="text-xs font-bold tracking-[0.12em] text-emerald-300">
        SPRINT 6
      </p>
      <h1 className="mt-2 text-3xl font-semibold text-slate-50 md:text-4xl">
        Findings
      </h1>
      <p className="mt-3 max-w-2xl text-slate-400">
        Diagnósticos gerados por analyzers (storage, índices, chunks) com
        triagem, evidências e deduplicação por first_seen/last_seen.
      </p>

      <div className="mt-8 space-y-6">
        <div className="grid gap-3 sm:grid-cols-3">
          <Input
            label="Finding type"
            placeholder="storage.top_consumer"
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
          />
          <Input
            label="Severity"
            placeholder="info, low, medium…"
            value={severityFilter}
            onChange={(e) => setSeverityFilter(e.target.value)}
          />
          <Input
            label="Status"
            placeholder="open, acknowledged…"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
          />
        </div>
        <div className="flex flex-wrap gap-2">
          <Button onClick={() => void load()} disabled={busy}>
            Atualizar
          </Button>
          <Button onClick={() => void runAnalyze()} disabled={busy}>
            {busy ? "Analisando…" : "Rodar analyzers (demo)"}
          </Button>
        </div>

        {error ? <Card title="Erro" subtitle={error} /> : null}
        {busy ? <Skeleton className="h-40 w-full" /> : null}

        {!busy && items.length === 0 ? (
          <Card
            title="Sem findings"
            subtitle="Nenhum resultado após filtros. Rode os analyzers demo."
          />
        ) : null}

        {!busy && items.length > 0 ? (
          <Table
            headers={[
              "Tipo",
              "Severidade",
              "Status",
              "Título",
              "Objeto",
              "Last seen",
            ]}
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
                  <Badge tone={severityTone(f.severity)}>{f.severity}</Badge>
                </td>
                <td className="px-4 py-3">
                  <Badge tone={statusTone(f.status)}>{f.status}</Badge>
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
            title={`Detalhe · ${selected.severity}`}
            subtitle={selected.title}
          >
            <ul className="mt-3 space-y-1 text-sm text-slate-300">
              <li>Tipo: {selected.finding_type}</li>
              <li>Status: {selected.status}</li>
              <li>Objeto: {selected.object_key || "—"}</li>
              <li>Resumo: {selected.summary}</li>
              <li>
                First seen:{" "}
                {selected.first_seen_at
                  ? new Date(selected.first_seen_at).toLocaleString()
                  : "—"}
              </li>
              <li>
                Last seen:{" "}
                {selected.last_seen_at
                  ? new Date(selected.last_seen_at).toLocaleString()
                  : "—"}
              </li>
            </ul>
            {selected.evidence ? (
              <pre className="mt-3 overflow-auto rounded bg-slate-900 p-3 text-xs text-slate-400">
                {JSON.stringify(selected.evidence, null, 2)}
              </pre>
            ) : null}
            <div className="mt-4 flex flex-wrap gap-2">
              <Button onClick={() => void triage(selected.id, "acknowledged")}>
                Acknowledge
              </Button>
              <Button onClick={() => void triage(selected.id, "resolved")}>
                Resolve
              </Button>
              <Button onClick={() => void triage(selected.id, "suppressed")}>
                Suppress
              </Button>
              <Button onClick={() => void triage(selected.id, "open")}>
                Reopen
              </Button>
            </div>
          </Card>
        ) : null}
      </div>
    </>
  );
}

import { useState } from "react";
import { Badge, Button, Card, Input, Skeleton, Table } from "../components/ui";
import { api } from "../services/api";
import type { CompareResult, ObjectDiff } from "../types";

function statusTone(
  status: string,
): "success" | "warning" | "danger" | "neutral" {
  if (status === "MATCH") {
    return "success";
  }
  if (status === "DRIFT" || status === "UNKNOWN") {
    return "warning";
  }
  if (status === "ONLY_SOURCE" || status === "ONLY_TARGET") {
    return "danger";
  }
  return "neutral";
}

/** Demo payloads so the page is usable without prior inventory load. */
const demoSource = [
  {
    object_type: "table",
    key: "public.orders",
    name: "orders",
    fingerprint: "fp-orders-v1",
  },
  {
    object_type: "table",
    key: "public.legacy",
    name: "legacy",
    fingerprint: "fp-legacy",
  },
  {
    object_type: "schema",
    key: "billing",
    name: "billing",
    fields: { table_count: "12" },
  },
];

const demoTarget = [
  {
    object_type: "table",
    key: "public.orders",
    name: "orders",
    fingerprint: "fp-orders-v2",
  },
  {
    object_type: "table",
    key: "public.new_tbl",
    name: "new_tbl",
    fingerprint: "fp-new",
  },
  {
    object_type: "schema",
    key: "billing",
    name: "billing",
    fields: { table_count: "12" },
  },
];

export function SchemaDriftPage() {
  const [result, setResult] = useState<CompareResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState("");
  const [typeFilter, setTypeFilter] = useState("");
  const [busy, setBusy] = useState(false);
  const [selected, setSelected] = useState<ObjectDiff | null>(null);

  const runCompare = async () => {
    setBusy(true);
    setError(null);
    setSelected(null);
    try {
      const statuses = statusFilter
        ? statusFilter.split(",").map((s) => s.trim()).filter(Boolean)
        : undefined;
      const res = await api.compare({
        source: demoSource,
        target: demoTarget,
        statuses,
        object_type: typeFilter || undefined,
      });
      setResult(res);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Falha na comparação");
      setResult(null);
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <p className="text-xs font-bold tracking-[0.12em] text-emerald-300">
        SPRINT 5
      </p>
      <h1 className="mt-2 text-3xl font-semibold text-slate-50 md:text-4xl">
        Schema Drift
      </h1>
      <p className="mt-3 max-w-2xl text-slate-400">
        Compare sets de objetos (fingerprints e campos) e classifique MATCH,
        DRIFT, ONLY_SOURCE, ONLY_TARGET e UNKNOWN.
      </p>

      <div className="mt-8 space-y-6">
        <div className="grid gap-3 sm:grid-cols-2">
          <Input
            label="Filtrar status (csv)"
            placeholder="MATCH,DRIFT,ONLY_SOURCE"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
          />
          <Input
            label="Filtrar object_type"
            placeholder="table, schema…"
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
          />
        </div>
        <Button onClick={runCompare} disabled={busy}>
          {busy ? "Comparando…" : "Comparar (demo payloads)"}
        </Button>

        {error ? <Card title="Erro" subtitle={error} /> : null}

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
            <Card title="Sem diferenças" subtitle="Nenhum objeto após filtros." />
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
          </Card>
        ) : null}
      </div>
    </>
  );
}

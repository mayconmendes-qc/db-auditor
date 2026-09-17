import { useEffect, useMemo, useState } from "react";
import { Badge, Card, Input, Skeleton, Table } from "../components/ui";
import { formatBytes, matchesSearch } from "../lib/format";
import { api } from "../services/api";
import type {
  CAGGSnapshot,
  ChunkSnapshot,
  DimensionSnapshot,
  Environment,
  HypertableSnapshot,
  JobSnapshot,
  PolicySnapshot,
} from "../types";

function hypertableKey(h: {
  database_name: string;
  schema_name: string;
  hypertable_name: string;
}) {
  return `${h.database_name}.${h.schema_name}.${h.hypertable_name}`;
}

export function InventoryPage() {
  const [envs, setEnvs] = useState<Environment[] | null>(null);
  const [envId, setEnvId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState("");
  const [hypertables, setHypertables] = useState<HypertableSnapshot[] | null>(
    null,
  );
  const [dimensions, setDimensions] = useState<DimensionSnapshot[] | null>(
    null,
  );
  const [chunks, setChunks] = useState<ChunkSnapshot[] | null>(null);
  const [caggs, setCaggs] = useState<CAGGSnapshot[] | null>(null);
  const [jobs, setJobs] = useState<JobSnapshot[] | null>(null);
  const [policies, setPolicies] = useState<PolicySnapshot[] | null>(null);
  const [selectedHT, setSelectedHT] = useState<string | null>(null);
  const [detailError, setDetailError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    api
      .environments()
      .then((res) => {
        if (!cancelled) {
          setEnvs(res.items);
          if (res.items.length > 0) {
            setEnvId(res.items[0].id);
          }
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Falha ao carregar");
          setEnvs([]);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!envId) {
      return;
    }
    let cancelled = false;
    setHypertables(null);
    setDimensions(null);
    setChunks(null);
    setCaggs(null);
    setJobs(null);
    setPolicies(null);
    setSelectedHT(null);
    setDetailError(null);
    Promise.all([
      api.hypertables(envId),
      api.dimensions(envId),
      api.chunks(envId),
      api.continuousAggregates(envId),
      api.jobs(envId),
      api.policies(envId),
    ])
      .then(([ht, dim, ch, ca, jb, pol]) => {
        if (!cancelled) {
          setHypertables(ht.items);
          setDimensions(dim.items);
          setChunks(ch.items);
          setCaggs(ca.items);
          setJobs(jb.items);
          setPolicies(pol.items);
          if (ht.items.length > 0) {
            setSelectedHT(hypertableKey(ht.items[0]));
          }
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setDetailError(
            err instanceof Error
              ? err.message
              : "Falha no inventário Timescale",
          );
          setHypertables([]);
          setDimensions([]);
          setChunks([]);
          setCaggs([]);
          setJobs([]);
          setPolicies([]);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [envId]);

  const filteredHT = useMemo(() => {
    if (!hypertables) {
      return null;
    }
    return hypertables.filter(
      (h) =>
        matchesSearch(h.hypertable_name, filter) ||
        matchesSearch(h.schema_name, filter) ||
        matchesSearch(h.database_name, filter),
    );
  }, [hypertables, filter]);

  const selectedDimensions = useMemo(() => {
    if (!dimensions || !selectedHT) {
      return [];
    }
    return dimensions.filter((d) => hypertableKey(d) === selectedHT);
  }, [dimensions, selectedHT]);

  const selectedChunks = useMemo(() => {
    if (!chunks || !selectedHT) {
      return [];
    }
    return chunks.filter((c) => hypertableKey(c) === selectedHT);
  }, [chunks, selectedHT]);

  return (
    <>
      <p className="text-xs font-bold tracking-[0.12em] text-emerald-300">
        SPRINT 2
      </p>
      <h1 className="mt-2 text-3xl font-semibold text-slate-50 md:text-4xl">
        Inventário TimescaleDB
      </h1>
      <p className="mt-3 max-w-2xl text-slate-400">
        Hypertables, dimensions, chunks, CAGGs, jobs e policies via API Go.
      </p>

      <div className="mt-8 space-y-8">
        {error ? (
          <Card title="Erro" subtitle={error} />
        ) : envs === null ? (
          <Skeleton className="h-24 w-full" />
        ) : envs.length === 0 ? (
          <Card
            title="Nenhum ambiente"
            subtitle="Registre ambientes e execute coletores Timescale."
          />
        ) : (
          <div className="flex flex-wrap gap-2">
            {envs.map((e) => (
              <button
                key={e.id}
                type="button"
                onClick={() => setEnvId(e.id)}
                className={`rounded-md px-3 py-1.5 text-sm ${
                  envId === e.id
                    ? "bg-emerald-700 text-white"
                    : "bg-slate-800 text-slate-300"
                }`}
              >
                {e.name}
              </button>
            ))}
          </div>
        )}

        {detailError ? (
          <Card title="Erro no inventário" subtitle={detailError} />
        ) : null}

        <div className="w-full sm:max-w-xs">
          <Input
            label="Filtrar hypertables"
            placeholder="Buscar…"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
        </div>

        <section>
          <h2 className="mb-3 text-xl font-semibold text-slate-100">
            Hypertables
          </h2>
          {filteredHT === null ? (
            <Skeleton className="h-40 w-full" />
          ) : filteredHT.length === 0 ? (
            <Card
              title="Nenhuma hypertable"
              subtitle="Sem snapshots ou filtro sem resultados."
            />
          ) : (
            <Table
              headers={[
                "Database",
                "Schema",
                "Nome",
                "Chunks",
                "Dims",
                "Compressão",
                "Tamanho",
              ]}
            >
              {filteredHT.map((h) => {
                const key = hypertableKey(h);
                return (
                  <tr
                    key={h.id}
                    className={`border-t border-slate-800 cursor-pointer ${
                      selectedHT === key ? "bg-slate-900/80" : ""
                    }`}
                    onClick={() => setSelectedHT(key)}
                  >
                    <td className="px-4 py-3 text-slate-300">
                      {h.database_name}
                    </td>
                    <td className="px-4 py-3 text-slate-300">
                      {h.schema_name}
                    </td>
                    <td className="px-4 py-3 text-slate-100">
                      {h.hypertable_name}
                    </td>
                    <td className="px-4 py-3 text-slate-300">{h.num_chunks}</td>
                    <td className="px-4 py-3 text-slate-300">
                      {h.num_dimensions}
                    </td>
                    <td className="px-4 py-3">
                      <Badge
                        tone={h.compression_enabled ? "success" : "neutral"}
                      >
                        {h.compression_enabled ? "on" : "off"}
                      </Badge>
                    </td>
                    <td className="px-4 py-3 text-slate-300">
                      {formatBytes(h.total_size_bytes)}
                    </td>
                  </tr>
                );
              })}
            </Table>
          )}
        </section>

        {selectedHT ? (
          <section className="grid gap-6 lg:grid-cols-2">
            <div>
              <h3 className="mb-2 text-sm font-medium text-slate-300">
                Dimensions — {selectedHT}
              </h3>
              {selectedDimensions.length === 0 ? (
                <Card title="Sem dimensions" subtitle="Nenhum snapshot." />
              ) : (
                <Table headers={["#", "Coluna", "Tipo", "Intervalo"]}>
                  {selectedDimensions.map((d) => (
                    <tr key={d.id} className="border-t border-slate-800">
                      <td className="px-4 py-3 text-slate-300">
                        {d.dimension_number}
                      </td>
                      <td className="px-4 py-3 text-slate-100">
                        {d.column_name}
                      </td>
                      <td className="px-4 py-3 text-slate-300">
                        {d.dimension_type ?? "—"}
                      </td>
                      <td className="px-4 py-3 text-slate-300">
                        {d.time_interval ?? "—"}
                      </td>
                    </tr>
                  ))}
                </Table>
              )}
            </div>
            <div>
              <h3 className="mb-2 text-sm font-medium text-slate-300">
                Chunks
              </h3>
              {selectedChunks.length === 0 ? (
                <Card title="Sem chunks" subtitle="Nenhum snapshot." />
              ) : (
                <Table headers={["Chunk", "Compressed", "Tamanho"]}>
                  {selectedChunks.slice(0, 50).map((c) => (
                    <tr key={c.id} className="border-t border-slate-800">
                      <td className="px-4 py-3 text-slate-100">
                        {c.chunk_name}
                      </td>
                      <td className="px-4 py-3">
                        <Badge tone={c.is_compressed ? "success" : "neutral"}>
                          {c.is_compressed ? "sim" : "não"}
                        </Badge>
                      </td>
                      <td className="px-4 py-3 text-slate-300">
                        {formatBytes(c.total_size_bytes)}
                      </td>
                    </tr>
                  ))}
                </Table>
              )}
            </div>
          </section>
        ) : null}

        <section className="grid gap-6 lg:grid-cols-3">
          <div>
            <h3 className="mb-2 text-sm font-medium text-slate-300">CAGGs</h3>
            {caggs === null ? (
              <Skeleton className="h-32 w-full" />
            ) : caggs.length === 0 ? (
              <Card title="Sem CAGGs" subtitle="Nenhum snapshot." />
            ) : (
              <Table headers={["View", "Compressão"]}>
                {caggs.map((c) => (
                  <tr key={c.id} className="border-t border-slate-800">
                    <td className="px-4 py-3 text-slate-100">
                      {c.schema_name}.{c.view_name}
                    </td>
                    <td className="px-4 py-3 text-slate-300">
                      {c.compression_enabled ? "on" : "off"}
                    </td>
                  </tr>
                ))}
              </Table>
            )}
          </div>
          <div>
            <h3 className="mb-2 text-sm font-medium text-slate-300">Jobs</h3>
            {jobs === null ? (
              <Skeleton className="h-32 w-full" />
            ) : jobs.length === 0 ? (
              <Card title="Sem jobs" subtitle="Nenhum snapshot." />
            ) : (
              <Table headers={["ID", "Proc", "Scheduled"]}>
                {jobs.slice(0, 30).map((j) => (
                  <tr key={j.id} className="border-t border-slate-800">
                    <td className="px-4 py-3 text-slate-100">{j.job_id}</td>
                    <td className="px-4 py-3 text-slate-300">
                      {j.proc_name ?? "—"}
                    </td>
                    <td className="px-4 py-3 text-slate-300">
                      {j.scheduled ? "sim" : "não"}
                    </td>
                  </tr>
                ))}
              </Table>
            )}
          </div>
          <div>
            <h3 className="mb-2 text-sm font-medium text-slate-300">
              Policies
            </h3>
            {policies === null ? (
              <Skeleton className="h-32 w-full" />
            ) : policies.length === 0 ? (
              <Card title="Sem policies" subtitle="Nenhum snapshot." />
            ) : (
              <Table headers={["Tipo", "Hypertable"]}>
                {policies.map((p) => (
                  <tr key={p.id} className="border-t border-slate-800">
                    <td className="px-4 py-3 text-slate-100">
                      {p.policy_type}
                    </td>
                    <td className="px-4 py-3 text-slate-300">
                      {p.hypertable_name ?? "—"}
                    </td>
                  </tr>
                ))}
              </Table>
            )}
          </div>
        </section>
      </div>
    </>
  );
}

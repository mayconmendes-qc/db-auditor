import { useEffect, useMemo, useState } from "react";
import { Badge, Card, Input, Skeleton, Table } from "../components/ui";
import { formatBytes, matchesSearch } from "../lib/format";
import { api } from "../services/api";
import type {
  DatabaseSnapshot,
  Environment,
  SchemaSnapshot,
} from "../types";

export function EnvironmentsPage() {
  const [items, setItems] = useState<Environment[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [databases, setDatabases] = useState<DatabaseSnapshot[] | null>(null);
  const [schemas, setSchemas] = useState<SchemaSnapshot[] | null>(null);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [filter, setFilter] = useState("");

  useEffect(() => {
    let cancelled = false;
    api
      .environments()
      .then((res) => {
        if (!cancelled) {
          setItems(res.items);
          setError(null);
          if (res.items.length > 0) {
            setSelectedId(res.items[0].id);
          }
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          const message =
            err instanceof Error ? err.message : "Falha ao carregar ambientes";
          setError(message);
          setItems([]);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!selectedId) {
      setDatabases(null);
      setSchemas(null);
      return;
    }
    let cancelled = false;
    setDatabases(null);
    setSchemas(null);
    setDetailError(null);
    Promise.all([api.databases(selectedId), api.schemas(selectedId)])
      .then(([dbRes, schemaRes]) => {
        if (!cancelled) {
          setDatabases(dbRes.items);
          setSchemas(schemaRes.items);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          const message =
            err instanceof Error
              ? err.message
              : "Falha ao carregar topologia";
          setDetailError(message);
          setDatabases([]);
          setSchemas([]);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [selectedId]);

  const filteredDatabases = useMemo(() => {
    if (!databases) {
      return null;
    }
    return databases.filter((db) =>
      matchesSearch(db.database_name, filter),
    );
  }, [databases, filter]);

  const filteredSchemas = useMemo(() => {
    if (!schemas) {
      return null;
    }
    return schemas.filter(
      (sc) =>
        matchesSearch(sc.schema_name, filter) ||
        matchesSearch(sc.database_name, filter),
    );
  }, [schemas, filter]);

  return (
    <>
      <p className="text-xs font-bold tracking-[0.12em] text-emerald-300">
        SPRINT 1
      </p>
      <h1 className="mt-2 text-3xl font-semibold text-slate-50 md:text-4xl">
        Ambientes
      </h1>
      <p className="mt-3 max-w-2xl text-slate-400">
        Topologia registrada no Snapshot Store. Dados vêm exclusivamente da API
        Go.
      </p>

      <div className="mt-8 space-y-8">
        {error ? (
          <Card title="Erro" subtitle={error} />
        ) : items === null ? (
          <Skeleton className="h-32 w-full" />
        ) : items.length === 0 ? (
          <Card
            title="Nenhum ambiente"
            subtitle="Registre ambientes e execute discovery para popular o inventário."
          />
        ) : (
          <Table headers={["Nome", "Tipo", "Discovery", "Status"]}>
            {items.map((env) => (
              <tr
                key={env.id}
                className={`border-t border-slate-800 cursor-pointer ${
                  selectedId === env.id ? "bg-slate-900/80" : ""
                }`}
                onClick={() => setSelectedId(env.id)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    setSelectedId(env.id);
                  }
                }}
                tabIndex={0}
              >
                <td className="px-4 py-3 text-slate-100">{env.name}</td>
                <td className="px-4 py-3 text-slate-300">{env.type}</td>
                <td className="px-4 py-3 text-slate-300">
                  {env.discovery_mode}
                </td>
                <td className="px-4 py-3">
                  <Badge tone={env.active ? "success" : "neutral"}>
                    {env.active ? "ativo" : "inativo"}
                  </Badge>
                </td>
              </tr>
            ))}
          </Table>
        )}

        {selectedId ? (
          <section className="space-y-4">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
              <h2 className="text-xl font-semibold text-slate-100">
                Topologia
              </h2>
              <div className="w-full sm:max-w-xs">
                <Input
                  label="Filtrar databases e schemas"
                  placeholder="Buscar…"
                  value={filter}
                  onChange={(e) => setFilter(e.target.value)}
                />
              </div>
            </div>

            {detailError ? (
              <Card title="Erro na topologia" subtitle={detailError} />
            ) : null}

            <div className="grid gap-6 lg:grid-cols-2">
              <div>
                <h3 className="mb-2 text-sm font-medium text-slate-300">
                  Databases
                </h3>
                {filteredDatabases === null ? (
                  <Skeleton className="h-40 w-full" />
                ) : filteredDatabases.length === 0 ? (
                  <Card
                    title="Nenhum database"
                    subtitle="Sem snapshots ou nenhum resultado para o filtro."
                  />
                ) : (
                  <Table headers={["Nome", "Tamanho", "Conexões"]}>
                    {filteredDatabases.map((db) => (
                      <tr key={db.id} className="border-t border-slate-800">
                        <td className="px-4 py-3 text-slate-100">
                          {db.database_name}
                        </td>
                        <td className="px-4 py-3 text-slate-300">
                          {formatBytes(db.size_bytes)}
                        </td>
                        <td className="px-4 py-3 text-slate-300">
                          {db.connection_count}
                        </td>
                      </tr>
                    ))}
                  </Table>
                )}
              </div>

              <div>
                <h3 className="mb-2 text-sm font-medium text-slate-300">
                  Schemas
                </h3>
                {filteredSchemas === null ? (
                  <Skeleton className="h-40 w-full" />
                ) : filteredSchemas.length === 0 ? (
                  <Card
                    title="Nenhum schema"
                    subtitle="Sem snapshots ou nenhum resultado para o filtro."
                  />
                ) : (
                  <Table
                    headers={[
                      "Database",
                      "Schema",
                      "Tables",
                      "Views",
                      "Tamanho",
                    ]}
                  >
                    {filteredSchemas.map((sc) => (
                      <tr key={sc.id} className="border-t border-slate-800">
                        <td className="px-4 py-3 text-slate-300">
                          {sc.database_name}
                        </td>
                        <td className="px-4 py-3 text-slate-100">
                          {sc.schema_name}
                        </td>
                        <td className="px-4 py-3 text-slate-300">
                          {sc.table_count}
                        </td>
                        <td className="px-4 py-3 text-slate-300">
                          {sc.view_count + sc.materialized_view_count}
                        </td>
                        <td className="px-4 py-3 text-slate-300">
                          {formatBytes(sc.size_bytes)}
                        </td>
                      </tr>
                    ))}
                  </Table>
                )}
              </div>
            </div>
          </section>
        ) : null}
      </div>
    </>
  );
}

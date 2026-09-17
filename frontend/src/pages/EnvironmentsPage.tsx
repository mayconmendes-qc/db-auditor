import { useEffect, useState } from "react";
import { Badge, Card, Skeleton, Table } from "../components/ui";
import { api } from "../services/api";
import type { Environment } from "../types";

export function EnvironmentsPage() {
  const [items, setItems] = useState<Environment[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    api
      .environments()
      .then((res) => {
        if (!cancelled) {
          setItems(res.items);
          setError(null);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Falha ao carregar ambientes");
          setItems([]);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <>
      <p className="text-xs font-bold tracking-[0.12em] text-emerald-300">SPRINT 1</p>
      <h1 className="mt-2 text-3xl font-semibold text-slate-50 md:text-4xl">Ambientes</h1>
      <p className="mt-3 max-w-2xl text-slate-400">
        Topologia registrada no Snapshot Store. Dados vêm exclusivamente da API Go.
      </p>

      <div className="mt-8">
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
              <tr key={env.id} className="border-t border-slate-800">
                <td className="px-4 py-3 text-slate-100">{env.name}</td>
                <td className="px-4 py-3 text-slate-300">{env.type}</td>
                <td className="px-4 py-3 text-slate-300">{env.discovery_mode}</td>
                <td className="px-4 py-3">
                  <Badge tone={env.active ? "success" : "neutral"}>
                    {env.active ? "ativo" : "inativo"}
                  </Badge>
                </td>
              </tr>
            ))}
          </Table>
        )}
      </div>
    </>
  );
}

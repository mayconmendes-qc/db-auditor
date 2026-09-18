import { useEffect, useState } from "react";
import {
  Badge,
  Button,
  EmptyState,
  ErrorBanner,
  Input,
  Skeleton,
  Table,
} from "../components/ui";
import { formatError } from "../lib/errors";
import { labels } from "../lib/labels";
import { api } from "../services/api";
import type { ObjectMapping } from "../types";

function statusTone(
  status: string,
): "success" | "warning" | "danger" | "neutral" {
  if (status === "validated") {
    return "success";
  }
  if (status === "suggested" || status === "manual") {
    return "warning";
  }
  if (status === "rejected") {
    return "danger";
  }
  return "neutral";
}

export function MappingsPage() {
  const [items, setItems] = useState<ObjectMapping[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState("");
  const [busyId, setBusyId] = useState<string | null>(null);

  const load = () => {
    setItems(null);
    setError(null);
    api
      .mappings({ status: statusFilter || undefined })
      .then((res) => setItems(res.items))
      .catch((err: unknown) => {
        setError(formatError(err, "Falha ao listar mapeamentos"));
        setItems([]);
      });
  };

  useEffect(() => {
    load();
  }, [statusFilter]);

  const setStatus = async (id: string, status: string) => {
    setBusyId(id);
    try {
      await api.updateMappingStatus(id, status);
      load();
    } catch (err: unknown) {
      setError(formatError(err, "Falha ao atualizar mapeamento"));
    } finally {
      setBusyId(null);
    }
  };

  return (
    <>
      <p className="text-xs font-bold tracking-[0.12em] text-emerald-300">
        MAPEAMENTOS
      </p>
      <h1 className="mt-2 text-3xl font-semibold text-slate-50 md:text-4xl">
        Mapeamentos
      </h1>
      <p className="mt-3 max-w-3xl text-sm leading-relaxed text-slate-400">
        Correspondências entre ambientes, confidence e validação manual.
      </p>

      <div className="mt-8 space-y-6">
        <Input
          label="Filtrar status"
          placeholder="suggested, validated, rejected, manual"
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
        />

        {error ? (
          <ErrorBanner message={error} onRetry={() => load()} />
        ) : null}

        {items === null ? (
          <Skeleton className="h-40 w-full" />
        ) : items.length === 0 ? (
          <EmptyState
            title="Nenhum mapeamento"
            description="Crie mappings manuais via API ou aceite sugestões por fingerprint."
          />
        ) : (
          <Table
            headers={[
              "Origem",
              "Destino",
              "Relação",
              "Conf.",
              "Status",
              "Ações",
            ]}
          >
            {items.map((m) => (
              <tr key={m.id} className="border-t border-slate-800">
                <td className="px-4 py-3 text-slate-100 font-mono text-xs">
                  {m.source_database}.{m.source_schema}/{m.source_object_name}
                </td>
                <td className="px-4 py-3 text-slate-100 font-mono text-xs">
                  {m.target_database}.{m.target_schema}/{m.target_object_name}
                </td>
                <td className="px-4 py-3 text-slate-300">{m.relation_type}</td>
                <td className="px-4 py-3 text-slate-300">
                  {(m.confidence * 100).toFixed(0)}%
                </td>
                <td className="px-4 py-3">
                  <Badge tone={statusTone(m.status)}>
                    {labels.mappingStatus(m.status)}
                  </Badge>
                </td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap gap-2">
                    <Button
                      variant="secondary"
                      disabled={busyId === m.id}
                      onClick={() => setStatus(m.id, "validated")}
                    >
                      Validar
                    </Button>
                    <Button
                      variant="ghost"
                      disabled={busyId === m.id}
                      onClick={() => setStatus(m.id, "rejected")}
                    >
                      Rejeitar
                    </Button>
                  </div>
                </td>
              </tr>
            ))}
          </Table>
        )}
      </div>
    </>
  );
}

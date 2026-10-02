import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  ErrorBanner,
  Skeleton,
  Table,
} from "../components/ui";
import {
  type PageSize,
  PaginationControls,
} from "../components/ui/PaginationControls";
import { useApp } from "../context/AppContext";
import { formatError } from "../lib/errors";
import { labels } from "../lib/labels";
import { fetchAllPages } from "../lib/pagination";
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
  return "neutral";
}

export function SecurityPage() {
  const { environmentId } = useApp();
  const [items, setItems] = useState<Finding[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [pageSize, setPageSize] = useState<PageSize>(20);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [selected, setSelected] = useState<Finding | null>(null);

  const load = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      const filters = {
        environment_id: environmentId || undefined,
        status: "open",
      };
      if (pageSize === "all") {
        const rows = await fetchAllPages((pageOffset, limit) =>
          api.findingsCategoryPage("security", {
            ...filters,
            offset: pageOffset,
            limit,
          }),
        );
        setItems(rows);
        setTotal(rows.length);
      } else {
        const res = await api.findingsCategoryPage("security", {
          ...filters,
          offset,
          limit: pageSize,
        });
        setItems(res.items);
        setTotal(res.page.total);
      }
    } catch (err: unknown) {
      setError(formatError(err, "Falha ao listar segurança"));
      setItems([]);
    } finally {
      setBusy(false);
    }
  }, [environmentId, offset, pageSize]);

  useEffect(() => {
    void load();
  }, [load]);

  const summary = useMemo(() => {
    const by = (type: string) =>
      items.filter((f) => f.finding_type === type).length;
    return {
      definer: by("security.security_definer"),
      roles: by("security.powerful_role"),
      grants: by("security.excessive_privilege"),
    };
  }, [items]);

  return (
    <>
      <p className="text-xs font-medium text-slate-400">Segurança</p>
      <h1 className="mt-2 text-3xl font-semibold text-slate-50 md:text-4xl">
        Revisão de segurança
      </h1>
      <p className="mt-3 max-w-3xl text-sm leading-relaxed text-slate-400">
        Funções com privilégios elevados, roles poderosas e permissões amplas
        para revisão humana. O auditor nunca emite REVOKE, ALTER ROLE ou DROP.
      </p>

      <div className="mt-8 space-y-6">
        <div className="grid gap-3 sm:grid-cols-3">
          <Card
            title="Funções com privilégios elevados"
            subtitle={String(summary.definer)}
          />
          <Card title="Roles poderosas" subtitle={String(summary.roles)} />
          <Card title="Privilégios amplos" subtitle={String(summary.grants)} />
        </div>
        <p className="text-xs text-slate-400">Resumo da página exibida.</p>

        <div className="flex flex-wrap gap-2">
          <Button onClick={() => void load()} disabled={busy}>
            Atualizar
          </Button>
        </div>

        {error ? (
          <ErrorBanner message={error} onRetry={() => void load()} />
        ) : null}
        {busy ? <Skeleton className="h-40 w-full" /> : null}

        {!busy && items.length === 0 ? (
          <EmptyState
            title="Sem findings de segurança"
            description="Execute uma auditoria para analisar funções com privilégios elevados e permissões amplas."
          />
        ) : null}

        {!busy && items.length > 0 ? (
          <>
            <Table
              pagination={false}
              headers={["Tipo", "Severidade", "Título", "Objeto", "Última vez"]}
            >
              {items.map((f) => (
                <tr
                  key={f.id}
                  className="cursor-pointer border-t border-slate-800"
                  onClick={() => setSelected(f)}
                >
                  <td className="px-4 py-3 font-mono text-xs text-slate-300">
                    {f.finding_type}
                  </td>
                  <td className="px-4 py-3">
                    <Badge tone={severityTone(f.severity)}>
                      {labels.severity(f.severity)}
                    </Badge>
                  </td>
                  <td className="px-4 py-3 text-slate-100">{f.title}</td>
                  <td className="px-4 py-3 font-mono text-xs text-slate-400">
                    {f.object_key || "—"}
                  </td>
                  <td className="px-4 py-3 text-xs text-slate-400">
                    {f.last_seen_at
                      ? new Date(f.last_seen_at).toLocaleString()
                      : "—"}
                  </td>
                </tr>
              ))}
            </Table>
            <PaginationControls
              total={total}
              offset={pageSize === "all" ? 0 : offset}
              size={pageSize}
              onSizeChange={(next) => {
                setPageSize(next);
                setOffset(0);
              }}
              onOffsetChange={setOffset}
              label="Segurança"
            />
          </>
        ) : null}

        {selected ? (
          <Card
            title={`Detalhe · ${labels.severity(selected.severity)}`}
            subtitle={selected.title}
          >
            <ul className="mt-3 space-y-1 text-sm text-slate-300">
              <li>Tipo: {selected.finding_type}</li>
              <li>Objeto: {selected.object_key || "—"}</li>
              <li>Resumo: {selected.summary}</li>
            </ul>
            <p className="mt-3 text-xs text-amber-300">
              Revisão apenas. Nenhuma ação destrutiva (REVOKE / ALTER / DROP) é
              sugerida ou executada automaticamente.
            </p>
            {selected.evidence ? (
              <pre className="mt-3 overflow-auto rounded bg-slate-900 p-3 text-xs text-slate-400">
                {JSON.stringify(selected.evidence, null, 2)}
              </pre>
            ) : null}
          </Card>
        ) : null}
      </div>
    </>
  );
}

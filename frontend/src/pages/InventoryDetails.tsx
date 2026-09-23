import { useEffect, useState } from "react";
import { DetailGrid, DetailSection } from "../components/ui/Sheet";
import { Skeleton } from "../components/ui";
import { formatBytes } from "../lib/format";
import { api } from "../services/api";
import type {
  ColumnSnapshot,
  FunctionSnapshot,
  HypertableSnapshot,
  IndexSnapshot,
  InventoryObjectKind,
  TableSnapshot,
  ViewSnapshot,
} from "../types";

function formatDate(iso: string | null | undefined): string {
  if (!iso) {
    return "—";
  }
  try {
    return new Date(iso).toLocaleString("pt-BR");
  } catch {
    return iso;
  }
}

function formatNumber(n: number | null | undefined): string {
  if (n == null || !Number.isFinite(n)) {
    return "—";
  }
  return n.toLocaleString("pt-BR");
}

function boolLabel(v: boolean | null | undefined): string {
  if (v == null) {
    return "—";
  }
  return v ? "Sim" : "Não";
}

function TableColumnsList({ t }: { t: TableSnapshot }) {
  const [columns, setColumns] = useState<ColumnSnapshot[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setColumns(null);
    setError(null);
    api
      .columns(t.environment_id, {
        database: t.database_name,
        schema: t.schema_name,
        table: t.table_name,
        limit: 500,
      })
      .then((res) => {
        if (!cancelled) {
          setColumns(res.items);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setError("Não foi possível carregar as colunas.");
          setColumns([]);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [t.environment_id, t.database_name, t.schema_name, t.table_name]);

  if (columns === null) {
    return <Skeleton className="h-24 w-full" />;
  }
  if (error) {
    return <p className="text-sm text-slate-400">{error}</p>;
  }
  if (columns.length === 0) {
    return (
      <p className="text-sm text-slate-400">
        Nenhuma coluna no snapshot para esta tabela (rode uma auditoria se o
        coletor ainda não rodou).
      </p>
    );
  }

  return (
    <div className="overflow-x-auto rounded-md border border-slate-800">
      <table className="w-full min-w-[20rem] text-left text-xs">
        <thead className="bg-slate-900/80 text-[10px] uppercase tracking-wider text-slate-500">
          <tr>
            <th className="px-2 py-1.5 font-semibold">#</th>
            <th className="px-2 py-1.5 font-semibold">Coluna</th>
            <th className="px-2 py-1.5 font-semibold">Tipo</th>
            <th className="px-2 py-1.5 font-semibold">Nullable</th>
            <th className="px-2 py-1.5 font-semibold">Default</th>
          </tr>
        </thead>
        <tbody>
          {columns.map((c) => (
            <tr
              key={c.id}
              className="border-t border-slate-800/80 text-slate-300"
            >
              <td className="px-2 py-1 tabular-nums text-slate-500">
                {c.ordinal_position}
              </td>
              <td className="px-2 py-1 font-medium text-slate-100">
                {c.column_name}
              </td>
              <td className="px-2 py-1 font-mono text-[11px] text-slate-300">
                {c.data_type}
              </td>
              <td className="px-2 py-1">{boolLabel(c.is_nullable)}</td>
              <td className="max-w-[10rem] truncate px-2 py-1 font-mono text-[11px] text-slate-500">
                {c.column_default ?? "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function TableDetail({ t }: { t: TableSnapshot }) {
  return (
    <>
      <DetailSection title="Identificação">
        <DetailGrid
          items={[
            { label: "Database", value: t.database_name },
            { label: "Schema", value: t.schema_name },
            { label: "Tabela", value: t.table_name },
            { label: "Owner", value: t.owner_name ?? "—" },
            { label: "Relkind", value: t.relkind },
            { label: "Chave primária", value: boolLabel(t.has_primary_key) },
            {
              label: "Colunas (contagem)",
              value: formatNumber(t.column_count),
            },
            { label: "Coletado em", value: formatDate(t.collected_at) },
          ]}
        />
      </DetailSection>
      <DetailSection title="Tamanho">
        <DetailGrid
          items={[
            { label: "Total", value: formatBytes(t.total_size_bytes) },
            { label: "Dados", value: formatBytes(t.data_size_bytes) },
            { label: "Índices", value: formatBytes(t.index_size_bytes) },
            {
              label: "Estimativa de linhas",
              value: formatNumber(t.row_estimate),
            },
          ]}
        />
      </DetailSection>
      <DetailSection title="Colunas">
        <TableColumnsList t={t} />
      </DetailSection>
      <DetailSection title="Referência">
        <DetailGrid
          items={[
            { label: "ID snapshot", value: t.id },
            { label: "Audit run", value: t.audit_run_id },
            { label: "Environment", value: t.environment_id },
          ]}
        />
      </DetailSection>
    </>
  );
}

export function IndexDetail({ i }: { i: IndexSnapshot }) {
  return (
    <>
      <DetailSection title="Identificação">
        <DetailGrid
          items={[
            { label: "Database", value: i.database_name },
            { label: "Schema", value: i.schema_name },
            { label: "Índice", value: i.index_name },
            { label: "Tabela", value: i.table_name },
            { label: "Método de acesso", value: i.access_method ?? "—" },
            { label: "Único", value: boolLabel(i.is_unique) },
            { label: "Primário", value: boolLabel(i.is_primary) },
            { label: "Coletado em", value: formatDate(i.collected_at) },
          ]}
        />
      </DetailSection>
      <DetailSection title="Uso e tamanho">
        <DetailGrid
          items={[
            { label: "Tamanho", value: formatBytes(i.size_bytes) },
            { label: "Index scans", value: formatNumber(i.idx_scan) },
          ]}
        />
      </DetailSection>
      {i.index_definition ? (
        <DetailSection title="Definição">
          <pre className="overflow-x-auto whitespace-pre-wrap break-all rounded-md border border-slate-800 bg-slate-950 p-3 font-mono text-[11px] leading-relaxed text-slate-300">
            {i.index_definition}
          </pre>
        </DetailSection>
      ) : null}
      <DetailSection title="Referência">
        <DetailGrid items={[{ label: "ID snapshot", value: i.id }]} />
      </DetailSection>
    </>
  );
}

export function ViewDetail({
  v,
  kind,
}: {
  v: ViewSnapshot;
  kind: InventoryObjectKind;
}) {
  return (
    <>
      <DetailSection title="Identificação">
        <DetailGrid
          items={[
            { label: "Database", value: v.database_name },
            { label: "Schema", value: v.schema_name },
            { label: "Nome", value: v.view_name },
            {
              label: "Tipo",
              value:
                kind === "caggs" ? "Continuous Aggregate" : v.relkind || "view",
            },
            { label: "Owner", value: v.owner_name ?? "—" },
            { label: "Tamanho", value: formatBytes(v.size_bytes) },
            { label: "Coletado em", value: formatDate(v.collected_at) },
          ]}
        />
      </DetailSection>
      <DetailSection title="Referência">
        <DetailGrid items={[{ label: "ID snapshot", value: v.id }]} />
      </DetailSection>
    </>
  );
}

export function FunctionDetail({ f }: { f: FunctionSnapshot }) {
  return (
    <>
      <DetailSection title="Identificação">
        <DetailGrid
          items={[
            { label: "Database", value: f.database_name },
            { label: "Schema", value: f.schema_name },
            { label: "Função", value: f.function_name },
            { label: "Argumentos", value: f.identity_arguments || "—" },
            { label: "Linguagem", value: f.language_name ?? "—" },
            { label: "Kind", value: f.kind ?? "—" },
            {
              label: "SECURITY DEFINER",
              value: boolLabel(f.is_security_definer),
            },
            { label: "Owner", value: f.owner_name ?? "—" },
            { label: "Coletado em", value: formatDate(f.collected_at) },
          ]}
        />
      </DetailSection>
      <DetailSection title="Referência">
        <DetailGrid items={[{ label: "ID snapshot", value: f.id }]} />
      </DetailSection>
    </>
  );
}

export function HypertableDetail({ h }: { h: HypertableSnapshot }) {
  return (
    <>
      <DetailSection title="Identificação">
        <DetailGrid
          items={[
            { label: "Database", value: h.database_name },
            { label: "Schema", value: h.schema_name },
            { label: "Hypertable", value: h.hypertable_name },
            { label: "Owner", value: h.owner_name ?? "—" },
            { label: "Dimensões", value: formatNumber(h.num_dimensions) },
            { label: "Chunks", value: formatNumber(h.num_chunks) },
            { label: "Compressão", value: boolLabel(h.compression_enabled) },
            { label: "Distribuída", value: boolLabel(h.is_distributed) },
            { label: "Coletado em", value: formatDate(h.collected_at) },
          ]}
        />
      </DetailSection>
      <DetailSection title="Tamanho">
        <DetailGrid
          items={[
            { label: "Total", value: formatBytes(h.total_size_bytes) },
            { label: "Dados", value: formatBytes(h.data_size_bytes) },
            { label: "Índices", value: formatBytes(h.index_size_bytes) },
          ]}
        />
      </DetailSection>
      <DetailSection title="Referência">
        <DetailGrid
          items={[
            { label: "ID snapshot", value: h.id },
            { label: "Audit run", value: h.audit_run_id },
            { label: "Environment", value: h.environment_id },
          ]}
        />
      </DetailSection>
    </>
  );
}

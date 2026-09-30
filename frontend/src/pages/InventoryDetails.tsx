import { DetailGrid, DetailSection } from "../components/ui/Sheet";
import { formatBytes } from "../lib/format";
import {
  relationClassBadgeClass,
  relationClassLabel,
} from "../lib/relationClass";
import type {
  ColumnSnapshot,
  FunctionSnapshot,
  HypertableSnapshot,
  IndexSnapshot,
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

export function TableDetail({
  t,
  columns = [],
}: {
  t: TableSnapshot;
  columns?: ColumnSnapshot[];
}) {
  const classLabel = relationClassLabel(t.relation_class, t.relkind);
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
            {
              label: "Classe de relação",
              value: (
                <span
                  className={`inline-flex rounded border px-1.5 py-0.5 text-[11px] font-medium ${
                    relationClassBadgeClass(t.relation_class)
                  }`}
                >
                  {classLabel}
                </span>
              ),
            },
            { label: "Partição", value: boolLabel(t.is_partition) },
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
        {columns.length === 0 ? (
          <p className="text-sm text-slate-400">
            Contagem no snapshot:{" "}
            <span className="font-medium text-slate-200">
              {formatNumber(t.column_count)}
            </span>
            . Lista detalhada indisponível para este objeto.
          </p>
        ) : (
          <div className="overflow-x-auto rounded-md border border-slate-700">
            <table className="min-w-full text-left text-xs text-slate-200">
              <thead className="bg-slate-900/80 text-slate-400">
                <tr>
                  <th className="px-2 py-1.5">#</th>
                  <th className="px-2 py-1.5">Nome</th>
                  <th className="px-2 py-1.5">Tipo</th>
                  <th className="px-2 py-1.5">Nullable</th>
                  <th className="px-2 py-1.5">Default</th>
                </tr>
              </thead>
              <tbody>
                {columns.map((c) => (
                  <tr key={c.id} className="border-t border-slate-800">
                    <td className="px-2 py-1.5">{c.ordinal_position}</td>
                    <td className="px-2 py-1.5 font-medium text-slate-100">
                      {c.column_name}
                    </td>
                    <td className="px-2 py-1.5">{c.data_type}</td>
                    <td className="px-2 py-1.5">{boolLabel(c.is_nullable)}</td>
                    <td className="px-2 py-1.5 font-mono text-[11px] text-slate-400">
                      {c.column_default ?? "—"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
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
            { label: "Tabela", value: i.table_name },
            { label: "Índice", value: i.index_name },
            { label: "Método", value: i.access_method ?? "—" },
            { label: "Unique", value: boolLabel(i.is_unique) },
            { label: "Primary", value: boolLabel(i.is_primary) },
            { label: "Coletado em", value: formatDate(i.collected_at) },
          ]}
        />
      </DetailSection>
      <DetailSection title="Métricas">
        <DetailGrid
          items={[
            { label: "Tamanho", value: formatBytes(i.size_bytes) },
            { label: "idx_scan", value: formatNumber(i.idx_scan) },
          ]}
        />
      </DetailSection>
      <DetailSection title="Definição">
        <pre className="overflow-x-auto rounded-md border border-slate-700 bg-slate-950/60 p-3 text-xs text-slate-300">
          {i.index_definition || "—"}
        </pre>
      </DetailSection>
    </>
  );
}

export function ViewDetail({ v }: { v: ViewSnapshot }) {
  return (
    <>
      <DetailSection title="Identificação">
        <DetailGrid
          items={[
            { label: "Database", value: v.database_name },
            { label: "Schema", value: v.schema_name },
            { label: "View", value: v.view_name },
            { label: "Owner", value: v.owner_name ?? "—" },
            { label: "Relkind", value: v.relkind },
            { label: "Tamanho", value: formatBytes(v.size_bytes) },
            { label: "Coletado em", value: formatDate(v.collected_at) },
          ]}
        />
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
            { label: "Owner", value: f.owner_name ?? "—" },
            { label: "Linguagem", value: f.language_name ?? "—" },
            { label: "Kind", value: f.kind ?? "—" },
            {
              label: "Security definer",
              value: boolLabel(f.is_security_definer),
            },
            { label: "Coletado em", value: formatDate(f.collected_at) },
          ]}
        />
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
            {
              label: "Compressão",
              value: boolLabel(h.compression_enabled),
            },
            { label: "Distributed", value: boolLabel(h.is_distributed) },
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
    </>
  );
}

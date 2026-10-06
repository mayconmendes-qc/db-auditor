export type PageSize = 20 | 50 | 100 | "all";

export interface PaginationControlsProps {
  total: number;
  offset: number;
  size: PageSize;
  onSizeChange: (size: PageSize) => void;
  onOffsetChange: (offset: number) => void;
  label?: string;
  allowAll?: boolean;
}

export function PaginationControls({
  total,
  offset,
  size,
  onSizeChange,
  onOffsetChange,
  label = "Tabela",
  allowAll = true,
}: PaginationControlsProps) {
  const start = total === 0 ? 0 : offset + 1;
  const end = size === "all" ? total : Math.min(offset + size, total);
  const step = size === "all" ? total : size;
  return (
    <nav
      aria-label={`Paginação: ${label}`}
      className="mt-3 flex flex-wrap items-center justify-between gap-3 text-xs text-slate-300"
    >
      <span aria-live="polite">
        {start}–{end} de {total.toLocaleString("pt-BR")}
      </span>
      <div className="flex items-center gap-2">
        <label>
          Linhas por página{" "}
          <select
            aria-label={`Linhas por página: ${label}`}
            value={size}
            onChange={(event) =>
              onSizeChange(
                event.target.value === "all"
                  ? "all"
                  : (Number(event.target.value) as PageSize),
              )
            }
            className="rounded border border-slate-600 bg-slate-900 px-2 py-1 text-slate-100"
          >
            <option value={20}>20</option>
            <option value={50}>50</option>
            <option value={100}>100</option>
            {allowAll ? <option value="all">Todas</option> : null}
          </select>
        </label>
        <button
          type="button"
          disabled={offset === 0 || size === "all"}
          onClick={() => onOffsetChange(Math.max(0, offset - step))}
          className="rounded border border-slate-600 px-2 py-1 disabled:opacity-40"
        >
          Anterior
        </button>
        <button
          type="button"
          disabled={size === "all" || offset + step >= total}
          onClick={() => onOffsetChange(offset + step)}
          className="rounded border border-slate-600 px-2 py-1 disabled:opacity-40"
        >
          Próxima
        </button>
      </div>
    </nav>
  );
}

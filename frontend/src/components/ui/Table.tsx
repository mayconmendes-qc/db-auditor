import { Children, type ReactNode, useState } from "react";
import type { SortDir } from "../../lib/sort";
import { type PageSize, PaginationControls } from "./PaginationControls";

export type TableHeader =
  | string
  | {
      id: string;
      label: string;
      sortable?: boolean;
    };

export interface TableProps {
  headers: TableHeader[];
  children: ReactNode;
  className?: string;
  /** Compact row padding for dense inventories. */
  dense?: boolean;
  sortKey?: string;
  sortDir?: SortDir;
  onSort?: (id: string) => void;
  pagination?: boolean;
}

function headerId(header: TableHeader, index: number): string {
  if (typeof header === "string") {
    return `h-${index}-${header}`;
  }
  return header.id;
}

function headerLabel(header: TableHeader): string {
  return typeof header === "string" ? header : header.label;
}

function headerSortable(header: TableHeader): boolean {
  return typeof header !== "string" && Boolean(header.sortable);
}

export function Table({
  headers,
  children,
  className = "",
  dense = false,
  sortKey,
  sortDir,
  onSort,
  pagination = true,
}: TableProps) {
  const thPad = dense ? "px-3 py-2" : "px-4 py-3";
  const [size, setSize] = useState<PageSize>(20);
  const [offset, setOffset] = useState(0);
  const rows = Children.toArray(children);
  const safeOffset =
    size === "all"
      ? 0
      : Math.min(offset, Math.max(0, Math.ceil(rows.length / size) - 1) * size);
  const visible =
    !pagination || size === "all"
      ? rows
      : rows.slice(safeOffset, safeOffset + size);
  return (
    <div className={className}>
      <div className="overflow-x-auto rounded-lg border border-slate-700">
        <table className="min-w-full text-left text-sm text-slate-200">
          <thead className="sticky top-0 z-10 bg-slate-900/95 text-slate-400 backdrop-blur-sm">
            <tr>
              {headers.map((header, index) => {
                const id = headerId(header, index);
                const label = headerLabel(header);
                const sortable = headerSortable(header);
                const active = sortable && sortKey === id;
                if (sortable && onSort) {
                  return (
                    <th key={id} className={`${thPad} font-medium`}>
                      <button
                        type="button"
                        onClick={() => onSort(id)}
                        className="inline-flex items-center gap-1 text-left text-slate-300 transition hover:text-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-slate-400"
                      >
                        <span>{label}</span>
                        <span
                          className="font-mono text-[10px] text-slate-500"
                          aria-hidden
                        >
                          {active ? (sortDir === "asc" ? "↑" : "↓") : "↕"}
                        </span>
                      </button>
                    </th>
                  );
                }
                return (
                  <th key={id} className={`${thPad} font-medium`}>
                    {label}
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800">{visible}</tbody>
        </table>
      </div>
      {pagination ? (
        <PaginationControls
          total={rows.length}
          offset={safeOffset}
          size={size}
          onSizeChange={(next) => {
            setSize(next);
            setOffset(0);
          }}
          onOffsetChange={setOffset}
          label={headers.map(headerLabel).join(", ")}
        />
      ) : null}
    </div>
  );
}

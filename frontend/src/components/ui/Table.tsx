import type { ReactNode } from "react";

export interface TableProps {
  headers: string[];
  children: ReactNode;
  className?: string;
  /** Compact row padding for dense inventories. */
  dense?: boolean;
}

export function Table({
  headers,
  children,
  className = "",
  dense = false,
}: TableProps) {
  const thPad = dense ? "px-3 py-2" : "px-4 py-3";
  return (
    <div
      className={`overflow-x-auto rounded-lg border border-slate-700 ${className}`}
    >
      <table className="min-w-full text-left text-sm text-slate-200">
        <thead className="sticky top-0 z-10 bg-slate-900/95 text-slate-400 backdrop-blur-sm">
          <tr>
            {headers.map((header) => (
              <th key={header} className={`${thPad} font-medium`}>
                {header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-800">{children}</tbody>
      </table>
    </div>
  );
}

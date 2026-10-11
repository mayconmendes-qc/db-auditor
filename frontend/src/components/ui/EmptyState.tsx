import type { ReactNode } from "react";
import { cn } from "../../lib/cn";

export interface EmptyStateProps {
  title: string;
  description?: string;
  action?: ReactNode;
  className?: string;
}

export function EmptyState({
  title,
  description,
  action,
  className,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        "rounded-xl border border-dashed border-slate-700 bg-slate-900/40 px-6 py-10 text-center",
        className,
      )}
    >
      <p className="text-base font-medium text-slate-100">{title}</p>
      {description ? (
        <p className="mx-auto mt-2 max-w-lg text-sm leading-relaxed text-slate-400">
          {description}
        </p>
      ) : null}
      {action ? <div className="mt-4 flex justify-center">{action}</div> : null}
    </div>
  );
}

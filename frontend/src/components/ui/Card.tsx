import type { ReactNode } from "react";

export interface CardProps {
  title?: string;
  subtitle?: string;
  children?: ReactNode;
  className?: string;
  onClick?: () => void;
}

export function Card({
  title,
  subtitle,
  children,
  className = "",
  onClick,
}: CardProps) {
  const interactive = Boolean(onClick);
  return (
    <article
      className={`flex h-full flex-col gap-3 rounded-xl border border-slate-700/80 bg-slate-900/80 p-5 ${
        interactive
          ? "cursor-pointer transition hover:border-slate-600 hover:bg-slate-900"
          : ""
      } ${className}`}
      onClick={onClick}
      onKeyDown={
        onClick
          ? (e) => {
              if (e.key === "Enter" || e.key === " ") {
                onClick();
              }
            }
          : undefined
      }
      role={interactive ? "button" : undefined}
      tabIndex={interactive ? 0 : undefined}
    >
      {subtitle ? (
        <span className="text-sm text-slate-400">{subtitle}</span>
      ) : null}
      {title ? (
        <strong className="text-base text-slate-50">{title}</strong>
      ) : null}
      {children ? (
        <div className="mt-auto flex flex-1 flex-col">{children}</div>
      ) : null}
    </article>
  );
}

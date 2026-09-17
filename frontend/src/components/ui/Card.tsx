import type { ReactNode } from "react";

export interface CardProps {
  title?: string;
  subtitle?: string;
  children?: ReactNode;
  className?: string;
}

export function Card({ title, subtitle, children, className = "" }: CardProps) {
  return (
    <article
      className={`grid gap-3 rounded-xl border border-slate-700/80 bg-slate-900/80 p-5 ${className}`}
    >
      {subtitle ? <span className="text-sm text-slate-400">{subtitle}</span> : null}
      {title ? <strong className="text-base text-slate-50">{title}</strong> : null}
      {children}
    </article>
  );
}

import type { ButtonHTMLAttributes, ReactNode } from "react";

type Variant = "primary" | "secondary" | "ghost";

const variants: Record<Variant, string> = {
  primary:
    "bg-emerald-400/90 text-slate-950 hover:bg-emerald-300 focus-visible:ring-emerald-300",
  secondary:
    "bg-slate-800 text-slate-100 border border-slate-600 hover:bg-slate-700 focus-visible:ring-slate-400",
  ghost:
    "bg-transparent text-slate-300 hover:bg-slate-800/80 focus-visible:ring-slate-500",
};

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  children: ReactNode;
}

export function Button({
  variant = "primary",
  className = "",
  children,
  ...props
}: ButtonProps) {
  return (
    <button
      type="button"
      className={`inline-flex items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition focus-visible:outline-none focus-visible:ring-2 disabled:opacity-50 ${variants[variant]} ${className}`}
      {...props}
    >
      {children}
    </button>
  );
}

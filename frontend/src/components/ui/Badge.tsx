import type { ReactNode } from "react";

type Tone = "neutral" | "success" | "warning" | "danger";

const tones: Record<Tone, string> = {
  neutral: "bg-slate-800 text-slate-300 border-slate-600",
  success: "bg-emerald-950 text-emerald-300 border-emerald-700",
  warning: "bg-amber-950 text-amber-300 border-amber-700",
  danger: "bg-rose-950 text-rose-300 border-rose-700",
};

export interface BadgeProps {
  tone?: Tone;
  children: ReactNode;
  className?: string;
}

export function Badge({
  tone = "neutral",
  children,
  className = "",
}: BadgeProps) {
  return (
    <span
      className={`inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium ${tones[tone]} ${className}`}
    >
      {children}
    </span>
  );
}

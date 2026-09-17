import type { ReactNode } from "react";
import type { NavigationSection } from "../../types";

export const navigationSections: NavigationSection[] = [
  "Visão geral",
  "Ambientes",
  "Audit runs",
  "Inventário",
  "Mappings",
  "Findings",
];

export interface ShellProps {
  children: ReactNode;
  activeSection?: NavigationSection;
  onNavigate?: (section: NavigationSection) => void;
}

export function Shell({
  children,
  activeSection = "Visão geral",
  onNavigate,
}: ShellProps) {
  return (
    <div className="grid min-h-screen grid-cols-1 md:grid-cols-[15rem_1fr]">
      <aside className="border-b border-slate-700 p-5 md:border-b-0 md:border-r">
        <strong className="text-sm font-semibold tracking-wide text-slate-50">
          Timescale Auditor
        </strong>
        <nav
          className="mt-6 grid grid-cols-2 gap-1 md:grid-cols-1"
          aria-label="Principal"
        >
          {navigationSections.map((section) => {
            const isActive = section === activeSection;
            return (
              <button
                key={section}
                type="button"
                onClick={() => onNavigate?.(section)}
                className={`rounded-md px-3 py-2 text-left text-sm transition ${
                  isActive
                    ? "bg-slate-800 text-white"
                    : "text-slate-400 hover:bg-slate-800/70 hover:text-white"
                }`}
              >
                {section}
              </button>
            );
          })}
        </nav>
      </aside>
      <main className="mx-auto w-full max-w-5xl px-6 py-12 md:px-12 md:py-16">
        {children}
      </main>
    </div>
  );
}

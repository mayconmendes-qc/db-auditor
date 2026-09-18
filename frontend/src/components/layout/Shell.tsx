import type { ReactNode } from "react";
import type { NavigationSection } from "../../types";

export const navigationSections: NavigationSection[] = [
  "Visão geral",
  "Dashboard",
  "Documentação",
  "Ambientes",
  "Execuções",
  "Inventário",
  "Mapeamentos",
  "Desvio de schema",
  "Findings",
  "Performance",
  "Segurança",
  "Status",
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
    <div className="grid min-h-screen grid-cols-1 md:grid-cols-[14rem_1fr]">
      <aside className="border-b border-slate-700 p-4 md:border-b-0 md:border-r md:sticky md:top-0 md:h-screen md:overflow-y-auto">
        <strong className="text-sm font-semibold tracking-wide text-slate-50">
          Timescale Auditor
        </strong>
        <nav
          className="mt-5 grid grid-cols-2 gap-1 md:grid-cols-1"
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
      <main className="mx-auto w-full max-w-7xl px-4 py-8 sm:px-6 md:px-8 md:py-10">
        {children}
      </main>
    </div>
  );
}

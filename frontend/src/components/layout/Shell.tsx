import type { ReactNode } from "react";
import { useEffect, useState } from "react";
import { useApp } from "../../context/AppContext";
import { api } from "../../services/api";
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

const navGroups: Array<{ label: string; items: NavigationSection[] }> = [
  { label: "Início", items: ["Visão geral", "Dashboard", "Documentação"] },
  {
    label: "Operação",
    items: ["Ambientes", "Execuções", "Inventário", "Mapeamentos"],
  },
  {
    label: "Análise",
    items: ["Desvio de schema", "Findings", "Performance", "Segurança"],
  },
  { label: "Sistema", items: ["Status"] },
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
  const { environments, environmentId, setEnvironmentId } = useApp();
  const [apiOk, setApiOk] = useState<boolean | null>(null);
  const [dsnDown, setDsnDown] = useState(0);

  useEffect(() => {
    let cancelled = false;
    api
      .health()
      .then(() => {
        if (!cancelled) {
          setApiOk(true);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setApiOk(false);
        }
      });
    api
      .connectionStatus()
      .then((res) => {
        if (!cancelled) {
          setDsnDown(
            res.items.filter((c) => c.dsn_configured && !c.reachable).length,
          );
        }
      })
      .catch(() => {
        if (!cancelled) {
          setDsnDown(0);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [activeSection]);

  let healthTitle = "…";
  if (apiOk === true) {
    healthTitle =
      dsnDown > 0 ? `API ok · ${dsnDown} DSN indisponível(is)` : "API ok";
  } else if (apiOk === false) {
    healthTitle = "API indisponível";
  }

  return (
    <div className="grid min-h-screen grid-cols-1 bg-slate-950 md:grid-cols-[15rem_1fr]">
      <aside className="border-b border-slate-800 bg-slate-950 p-4 md:sticky md:top-0 md:h-screen md:overflow-y-auto md:border-b-0 md:border-r md:border-slate-800">
        <div className="flex items-start justify-between gap-2">
          <div>
            <strong className="text-sm font-semibold tracking-wide text-white">
              DB Auditor
            </strong>
            <p className="mt-0.5 text-[10px] font-medium uppercase tracking-wider text-slate-500">
              Qualle Control
            </p>
          </div>
          <span
            className={`mt-0.5 inline-block h-2 w-2 rounded-full ${
              apiOk === null
                ? "bg-slate-600"
                : apiOk
                  ? dsnDown > 0
                    ? "bg-amber-400"
                    : "bg-emerald-400"
                  : "bg-rose-400"
            }`}
            title={healthTitle}
          />
        </div>
        {apiOk === true && dsnDown > 0 ? (
          <p className="mt-1 text-[10px] text-amber-300/90">
            {dsnDown} DSN down
          </p>
        ) : null}

        <label className="mt-4 block text-[10px] font-semibold uppercase tracking-wider text-slate-400">
          Ambiente
          <select
            className="mt-1 w-full rounded-md border border-slate-700 bg-slate-900 px-2 py-1.5 text-xs text-slate-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/60"
            value={environmentId ?? ""}
            onChange={(e) => setEnvironmentId(e.target.value || null)}
            aria-label="Ambiente global"
          >
            <option value="">Todos</option>
            {environments.map((env) => (
              <option key={env.id} value={env.id}>
                {env.name}
              </option>
            ))}
          </select>
        </label>

        <nav className="mt-5 space-y-4" aria-label="Principal">
          {navGroups.map((group) => (
            <div key={group.label}>
              <p className="mb-1 px-2 text-[10px] font-semibold uppercase tracking-wider text-slate-500">
                {group.label}
              </p>
              <div className="grid grid-cols-2 gap-0.5 md:grid-cols-1">
                {group.items.map((section) => {
                  const isActive = section === activeSection;
                  return (
                    <button
                      key={section}
                      type="button"
                      onClick={() => onNavigate?.(section)}
                      className={`rounded-md px-3 py-2 text-left text-sm transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/50 ${
                        isActive
                          ? "bg-white text-black font-medium"
                          : "text-slate-300 hover:bg-slate-800 hover:text-white"
                      }`}
                    >
                      {section}
                    </button>
                  );
                })}
              </div>
            </div>
          ))}
        </nav>
      </aside>

      <main className="w-full min-w-0 px-4 py-6 sm:px-6 md:px-8 md:py-8 lg:px-10">
        <div className="mx-auto w-full max-w-[1600px]">{children}</div>
      </main>
    </div>
  );
}

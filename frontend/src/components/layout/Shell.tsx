import type { ReactNode } from "react";
import { useEffect, useState } from "react";
import { useApp } from "../../context/AppContext";
import { useTheme } from "../../context/ThemeContext";
import { api } from "../../services/api";
import type { NavigationSection } from "../../types";
import { Select } from "../ui";

export const navigationSections: NavigationSection[] = [
  "Dashboard",
  "Documentação",
  "Ambientes",
  "Execuções",
  "Relatórios",
  "Inventário",
  "Mapeamentos",
  "Desvio de schema",
  "Findings",
  "Performance",
  "Segurança",
  "Status",
];

const navGroups: Array<{ label: string; items: NavigationSection[] }> = [
  { label: "Início", items: ["Dashboard", "Documentação"] },
  {
    label: "Operação",
    items: [
      "Ambientes",
      "Execuções",
      "Relatórios",
      "Inventário",
      "Mapeamentos",
    ],
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

function ThemeToggle() {
  const { theme, toggleTheme } = useTheme();
  const isDark = theme === "dark";
  return (
    <button
      type="button"
      onClick={toggleTheme}
      className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-slate-700 text-slate-300 transition hover:bg-slate-800 hover:text-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/50"
      aria-label={isDark ? "Ativar tema claro" : "Ativar tema escuro"}
      title={isDark ? "Tema claro" : "Tema escuro"}
    >
      {isDark ? (
        <svg
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          aria-hidden="true"
        >
          <circle cx="12" cy="12" r="4" />
          <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41" />
        </svg>
      ) : (
        <svg
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          aria-hidden="true"
        >
          <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
        </svg>
      )}
    </button>
  );
}

function healthLabel(
  apiOk: boolean | null,
  dsnDown: number,
): { title: string; short: string | null } {
  if (apiOk === null) {
    return { title: "Verificando conexão…", short: null };
  }
  if (apiOk === false) {
    return {
      title: "Serviço indisponível — verifique se a API está no ar",
      short: "Indisponível",
    };
  }
  if (dsnDown > 0) {
    const n = dsnDown;
    return {
      title: `Serviço disponível · ${n} ambiente${n > 1 ? "s" : ""} sem conexão`,
      short: `${n} sem conexão`,
    };
  }
  return { title: "Serviço disponível", short: null };
}

export function Shell({
  children,
  activeSection = "Dashboard",
  onNavigate,
}: ShellProps) {
  const { environments, environmentId, setEnvironmentId } = useApp();
  const [apiOk, setApiOk] = useState<boolean | null>(null);
  const [dsnDown, setDsnDown] = useState(0);
  const [openFindings, setOpenFindings] = useState(0);
  const [runningRuns, setRunningRuns] = useState(0);

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
    api
      .analyticsKpis()
      .then((k) => {
        if (!cancelled) {
          setOpenFindings(k.open_findings ?? 0);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setOpenFindings(0);
        }
      });
    api
      .auditRuns({ status: "running" })
      .then((res) => {
        if (!cancelled) {
          setRunningRuns(res.items.length);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setRunningRuns(0);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [activeSection]);

  const { title: healthTitle, short: healthShort } = healthLabel(
    apiOk,
    dsnDown,
  );

  const statusDotClass =
    apiOk === null
      ? "bg-slate-600"
      : apiOk
        ? dsnDown > 0
          ? "bg-amber-400"
          : "bg-emerald-400"
        : "bg-rose-400";

  const envOptions = [
    { value: "", label: "Todos" },
    ...environments.map((env) => ({ value: env.id, label: env.name })),
  ];

  const badgeFor = (section: NavigationSection): number | null => {
    if (section === "Findings" && openFindings > 0) {
      return openFindings;
    }
    if (section === "Execuções" && runningRuns > 0) {
      return runningRuns;
    }
    return null;
  };

  return (
    <div className="grid min-h-screen grid-cols-1 bg-slate-950 md:grid-cols-[15rem_1fr]">
      <aside className="flex flex-col border-b border-slate-800 bg-slate-950 p-4 md:sticky md:top-0 md:h-screen md:overflow-y-auto md:border-b-0 md:border-r md:border-slate-800">
        <div className="min-w-0">
          <strong className="text-sm font-semibold text-slate-50">
            DB Auditor
          </strong>
          <p className="mt-0.5 text-xs text-slate-400">Qualle Control</p>
        </div>

        <div className="mt-4">
          <Select
            label="Ambiente"
            options={envOptions}
            value={environmentId ?? ""}
            onChange={(e) => setEnvironmentId(e.target.value || null)}
            aria-label="Ambiente global"
          />
        </div>

        <nav className="mt-5 min-h-0 flex-1 space-y-4" aria-label="Principal">
          {navGroups.map((group) => (
            <div key={group.label}>
              <p className="mb-1 px-2 text-xs font-medium text-slate-500">
                {group.label}
              </p>
              <div className="grid grid-cols-1 gap-0.5">
                {group.items.map((section) => {
                  const isActive = section === activeSection;
                  const count = badgeFor(section);
                  return (
                    <button
                      key={section}
                      type="button"
                      onClick={() => onNavigate?.(section)}
                      className={
                        isActive
                          ? "flex items-center justify-between gap-2 rounded-md px-3 py-2 text-left text-sm font-medium transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-slate-400"
                          : "flex items-center justify-between gap-2 rounded-md px-3 py-2 text-left text-sm text-slate-300 transition hover:bg-slate-800 hover:text-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-slate-400"
                      }
                      style={
                        isActive
                          ? {
                              backgroundColor: "var(--nav-active-bg)",
                              color: "var(--nav-active-fg)",
                            }
                          : undefined
                      }
                    >
                      <span className="min-w-0 truncate">{section}</span>
                      {count != null ? (
                        <span
                          className={
                            isActive
                              ? "shrink-0 rounded-full px-1.5 py-0.5 font-mono text-[10px] opacity-70"
                              : "shrink-0 rounded-full bg-slate-800 px-1.5 py-0.5 font-mono text-[10px] text-slate-300"
                          }
                        >
                          {count > 99 ? "99+" : count}
                        </span>
                      ) : null}
                    </button>
                  );
                })}
              </div>
            </div>
          ))}
        </nav>

        <div className="mt-4 flex shrink-0 items-center justify-between gap-2 border-t border-slate-800 pt-3">
          <button
            type="button"
            onClick={() => onNavigate?.("Status")}
            className="flex min-w-0 items-center gap-2 rounded-md px-1 py-1 text-left text-xs text-slate-400 transition hover:text-slate-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-slate-400"
            title={healthTitle}
            aria-label={healthTitle}
          >
            <span
              className={`inline-block h-2 w-2 shrink-0 rounded-full ${statusDotClass}`}
              aria-hidden="true"
            />
            <span className="truncate">
              {apiOk === null
                ? "Verificando…"
                : apiOk === false
                  ? "Indisponível"
                  : (healthShort ?? "Disponível")}
            </span>
          </button>
          <ThemeToggle />
        </div>
      </aside>

      <main className="w-full min-w-0 px-4 py-6 sm:px-6 md:px-8 md:py-8 lg:px-10">
        <div className="mx-auto w-full max-w-[1600px]">{children}</div>
      </main>
    </div>
  );
}

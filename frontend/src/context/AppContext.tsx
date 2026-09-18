import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { api } from "../services/api";
import type { Environment, NavigationSection } from "../types";

const SECTION_SLUGS: Record<NavigationSection, string> = {
  "Visão geral": "overview",
  Dashboard: "dashboard",
  Documentação: "docs",
  Ambientes: "environments",
  Execuções: "runs",
  Inventário: "inventory",
  Mapeamentos: "mappings",
  "Desvio de schema": "schema-drift",
  Findings: "findings",
  Performance: "performance",
  Segurança: "security",
  Status: "status",
};

const SLUG_TO_SECTION = Object.fromEntries(
  Object.entries(SECTION_SLUGS).map(([k, v]) => [v, k]),
) as Record<string, NavigationSection>;

function sectionFromHash(): NavigationSection {
  const raw = window.location.hash.replace(/^#\/?/, "");
  const slug = raw.split("?")[0] || "overview";
  return SLUG_TO_SECTION[slug] ?? "Visão geral";
}

function writeHash(section: NavigationSection) {
  const slug = SECTION_SLUGS[section];
  const next = `#/${slug}`;
  if (window.location.hash !== next) {
    window.history.replaceState(null, "", next);
  }
}

export interface AppContextValue {
  section: NavigationSection;
  setSection: (section: NavigationSection) => void;
  environments: Environment[];
  environmentsLoading: boolean;
  environmentId: string | null;
  setEnvironmentId: (id: string | null) => void;
  selectedEnvironment: Environment | null;
  refreshEnvironments: () => Promise<void>;
}

const AppContext = createContext<AppContextValue | null>(null);

export function AppProvider({ children }: { children: ReactNode }) {
  const [section, setSectionState] = useState<NavigationSection>(() =>
    typeof window !== "undefined" ? sectionFromHash() : "Visão geral",
  );
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [environmentsLoading, setEnvironmentsLoading] = useState(true);
  const [environmentId, setEnvironmentId] = useState<string | null>(null);

  const setSection = useCallback((next: NavigationSection) => {
    setSectionState(next);
    writeHash(next);
  }, []);

  useEffect(() => {
    const onHash = () => setSectionState(sectionFromHash());
    window.addEventListener("hashchange", onHash);
    writeHash(sectionFromHash());
    return () => window.removeEventListener("hashchange", onHash);
  }, []);

  const refreshEnvironments = useCallback(async () => {
    setEnvironmentsLoading(true);
    try {
      const res = await api.environments();
      setEnvironments(res.items);
      setEnvironmentId((prev) => {
        if (prev && res.items.some((e) => e.id === prev)) {
          return prev;
        }
        return res.items[0]?.id ?? null;
      });
    } catch {
      setEnvironments([]);
    } finally {
      setEnvironmentsLoading(false);
    }
  }, []);

  useEffect(() => {
    void refreshEnvironments();
  }, [refreshEnvironments]);

  const selectedEnvironment = useMemo(
    () => environments.find((e) => e.id === environmentId) ?? null,
    [environments, environmentId],
  );

  const value = useMemo(
    () => ({
      section,
      setSection,
      environments,
      environmentsLoading,
      environmentId,
      setEnvironmentId,
      selectedEnvironment,
      refreshEnvironments,
    }),
    [
      section,
      setSection,
      environments,
      environmentsLoading,
      environmentId,
      selectedEnvironment,
      refreshEnvironments,
    ],
  );

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>;
}

export function useApp(): AppContextValue {
  const ctx = useContext(AppContext);
  if (!ctx) {
    throw new Error("useApp must be used within AppProvider");
  }
  return ctx;
}

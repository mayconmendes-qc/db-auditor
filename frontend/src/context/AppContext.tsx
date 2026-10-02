import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { api } from "../services/api";
import type { Environment, NavigationSection } from "../types";

const SECTION_SLUGS: Record<NavigationSection, string> = {
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

function parseHash(): { section: NavigationSection; env: string | null } {
  const raw = window.location.hash.replace(/^#\/?/, "");
  const [pathPart, queryPart] = raw.split("?");
  const slug = pathPart || "dashboard";
  // Legacy bookmark #/overview → Dashboard
  const section =
    slug === "overview" ? "Dashboard" : (SLUG_TO_SECTION[slug] ?? "Dashboard");
  let env: string | null = null;
  if (queryPart) {
    const params = new URLSearchParams(queryPart);
    env = params.get("env");
  }
  return { section, env };
}

function writeHash(section: NavigationSection, envId: string | null) {
  const slug = SECTION_SLUGS[section];
  let next = `#/${slug}`;
  const query = new URLSearchParams();
  if (envId) query.set("env", envId);
  if (slug === "inventory" && window.location.hash.startsWith("#/inventory?")) {
    const current = new URLSearchParams(window.location.hash.split("?")[1]);
    if (envId && current.get("env") === envId) {
      for (const field of ["run", "database", "schema", "table"]) {
        const value = current.get(field);
        if (value) query.set(field, value);
      }
    }
  }
  if (query.size) next += `?${query}`;
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
  const initial =
    typeof window !== "undefined"
      ? parseHash()
      : { section: "Dashboard" as NavigationSection, env: null };

  const [section, setSectionState] = useState<NavigationSection>(
    initial.section,
  );
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [environmentsLoading, setEnvironmentsLoading] = useState(true);
  const [environmentId, setEnvironmentIdState] = useState<string | null>(
    initial.env,
  );

  const setSection = useCallback(
    (next: NavigationSection) => {
      setSectionState(next);
      // Dashboard sempre abre com filtro "Todos" (todos os ambientes).
      if (next === "Dashboard") {
        setEnvironmentIdState(null);
        writeHash(next, null);
        return;
      }
      writeHash(next, environmentId);
    },
    [environmentId],
  );

  const setEnvironmentId = useCallback(
    (id: string | null) => {
      setEnvironmentIdState(id);
      writeHash(section, id);
    },
    [section],
  );

  useEffect(() => {
    const onHash = () => {
      const parsed = parseHash();
      setSectionState(parsed.section);
      if (parsed.env !== undefined) {
        setEnvironmentIdState(parsed.env);
      }
    };
    window.addEventListener("hashchange", onHash);
    writeHash(section, environmentId);
    return () => window.removeEventListener("hashchange", onHash);
    // only on mount for hash listener
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const refreshEnvironments = useCallback(async () => {
    setEnvironmentsLoading(true);
    try {
      const res = await api.environments();
      setEnvironments(res.items);
      // Preserva null ("Todos") e IDs ainda válidos; não força o primeiro ambiente.
      setEnvironmentIdState((prev) => {
        if (prev == null) {
          return null;
        }
        if (res.items.some((e) => e.id === prev)) {
          return prev;
        }
        return null;
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

  useEffect(() => {
    writeHash(section, environmentId);
  }, [section, environmentId]);

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
      setEnvironmentId,
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

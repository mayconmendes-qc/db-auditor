import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { api } from "../services/api";
import type { Environment, NavigationSection } from "../types";

const SECTION_SLUGS: Record<NavigationSection, string> = {
  Dashboard: "dashboard",
  Documentação: "docs",
  Ambientes: "environments",
  Execuções: "runs",
  Relatórios: "reports",
  Inventário: "inventory",
  Mapeamentos: "mappings",
  "Desvio de schema": "schema-drift",
  Comparar: "compare",
  Findings: "findings",
  Performance: "performance",
  Segurança: "security",
  Regras: "rules",
  Status: "status",
};

const SLUG_TO_SECTION = Object.fromEntries(
  Object.entries(SECTION_SLUGS).map(([k, v]) => [v, k]),
) as Record<string, NavigationSection>;

export interface InventoryTarget {
  database: string;
  schema: string;
  table: string;
}

interface LocationState {
  section: NavigationSection;
  env: string | null;
  runId: string | null;
  findingId: string | null;
  inventory: InventoryTarget | null;
  search: Record<string, string>;
}

function parseLocation(): LocationState {
  const raw = window.location.hash.replace(/^#\/?/, "");
  const [pathPart, queryPart] = raw.split("?");
  const parts = pathPart.split("/").filter(Boolean);
  const params = new URLSearchParams(queryPart || "");
  const search: Record<string, string> = {};
  params.forEach((value, key) => {
    if (key !== "env" && value) search[key] = value;
  });
  const head = parts[0] || "dashboard";
  let section: NavigationSection = "Dashboard";
  let runId: string | null = null;
  let findingId: string | null = null;
  let inventory: InventoryTarget | null = null;
  if (head === "runs" && parts[1]) {
    section = "Execuções";
    runId = decodeURIComponent(parts[1]);
  } else if (head === "findings" && parts[1]) {
    section = "Findings";
    findingId = decodeURIComponent(parts[1]);
  } else if (head === "inventory" && parts.length >= 4) {
    section = "Inventário";
    inventory = {
      database: decodeURIComponent(parts[1]),
      schema: decodeURIComponent(parts[2]),
      table: decodeURIComponent(parts[3]),
    };
  } else if (head === "inventory") {
    section = "Inventário";
    const database = params.get("database");
    const schema = params.get("schema");
    const table = params.get("table");
    if (database && schema && table) {
      inventory = { database, schema, table };
    }
  } else if (head === "compare") {
    section = "Comparar";
  } else if (head === "overview") {
    section = "Dashboard";
  } else {
    section = SLUG_TO_SECTION[head] ?? "Dashboard";
  }
  return {
    section,
    env: params.get("env"),
    runId,
    findingId,
    inventory,
    search,
  };
}

function formatLocation(state: LocationState): string {
  let path = SECTION_SLUGS[state.section];
  if (state.section === "Execuções" && state.runId) {
    path = `runs/${encodeURIComponent(state.runId)}`;
  } else if (state.section === "Findings" && state.findingId) {
    path = `findings/${encodeURIComponent(state.findingId)}`;
  } else if (state.section === "Inventário" && state.inventory) {
    const item = state.inventory;
    path = `inventory/${encodeURIComponent(item.database)}/${encodeURIComponent(item.schema)}/${encodeURIComponent(item.table)}`;
  }
  const query = new URLSearchParams();
  if (state.env) query.set("env", state.env);
  for (const [key, value] of Object.entries(state.search)) {
    if (value) query.set(key, value);
  }
  const suffix = query.size ? `?${query}` : "";
  return `#/${path}${suffix}`;
}

function sameSearch(a: Record<string, string>, b: Record<string, string>) {
  const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
  for (const key of keys) {
    if ((a[key] || "") !== (b[key] || "")) return false;
  }
  return true;
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
  runId: string | null;
  findingId: string | null;
  inventory: InventoryTarget | null;
  search: Record<string, string>;
  setSearch: (patch: Record<string, string | null>) => void;
  openRun: (id: string) => void;
  openFinding: (id: string | null) => void;
  openInventory: (target: InventoryTarget, run?: string) => void;
}

const AppContext = createContext<AppContextValue | null>(null);

export function AppProvider({ children }: { children: ReactNode }) {
  const initial =
    typeof window !== "undefined"
      ? parseLocation()
      : {
          section: "Dashboard" as NavigationSection,
          env: null,
          runId: null,
          findingId: null,
          inventory: null,
          search: {},
        };

  const [section, setSectionState] = useState<NavigationSection>(
    initial.section,
  );
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [environmentsLoading, setEnvironmentsLoading] = useState(true);
  const [environmentId, setEnvironmentIdState] = useState<string | null>(
    initial.env,
  );
  const [runId, setRunId] = useState<string | null>(initial.runId);
  const [findingId, setFindingId] = useState<string | null>(initial.findingId);
  const [inventory, setInventory] = useState<InventoryTarget | null>(
    initial.inventory,
  );
  const [search, setSearchState] = useState<Record<string, string>>(
    initial.search,
  );
  const fromHistory = useRef(false);
  const boot = useRef(true);

  const snapshot = useCallback(
    (): LocationState => ({
      section,
      env: environmentId,
      runId,
      findingId,
      inventory,
      search,
    }),
    [section, environmentId, runId, findingId, inventory, search],
  );

  useEffect(() => {
    if (fromHistory.current) {
      fromHistory.current = false;
      return;
    }
    const next = formatLocation(snapshot());
    if (window.location.hash === next) {
      boot.current = false;
      return;
    }
    if (boot.current) {
      window.history.replaceState(null, "", next);
      boot.current = false;
      return;
    }
    window.history.pushState(null, "", next);
  }, [snapshot]);

  useEffect(() => {
    const onPop = () => {
      const parsed = parseLocation();
      fromHistory.current = true;
      setSectionState(parsed.section);
      setEnvironmentIdState(parsed.env);
      setRunId(parsed.runId);
      setFindingId(parsed.findingId);
      setInventory(parsed.inventory);
      setSearchState(parsed.search);
    };
    window.addEventListener("hashchange", onPop);
    window.addEventListener("popstate", onPop);
    return () => {
      window.removeEventListener("hashchange", onPop);
      window.removeEventListener("popstate", onPop);
    };
  }, []);

  const setSection = useCallback((next: NavigationSection) => {
    setSectionState(next);
    setRunId(null);
    setFindingId(null);
    setInventory(null);
    setSearchState({});
  }, []);

  const setEnvironmentId = useCallback((id: string | null) => {
    setEnvironmentIdState(id);
  }, []);

  const setSearch = useCallback((patch: Record<string, string | null>) => {
    setSearchState((prev) => {
      const next = { ...prev };
      for (const [key, value] of Object.entries(patch)) {
        if (!value) delete next[key];
        else next[key] = value;
      }
      return sameSearch(prev, next) ? prev : next;
    });
  }, []);

  const openRun = useCallback((id: string) => {
    setSectionState("Execuções");
    setRunId(id);
    setFindingId(null);
    setInventory(null);
  }, []);

  const openFinding = useCallback((id: string | null) => {
    setSectionState("Findings");
    setFindingId(id);
    setRunId(null);
    setInventory(null);
  }, []);

  const openInventory = useCallback((target: InventoryTarget, run?: string) => {
    setSectionState("Inventário");
    setInventory(target);
    setRunId(null);
    setFindingId(null);
    if (run) {
      setSearchState((prev) => (prev.run === run ? prev : { ...prev, run }));
    }
  }, []);

  const refreshEnvironments = useCallback(async () => {
    setEnvironmentsLoading(true);
    try {
      const res = await api.environments();
      setEnvironments(res.items);
      setEnvironmentIdState((prev) => {
        if (prev == null) {
          return api.hasRole("operator") ? null : (res.items[0]?.id ?? null);
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
      runId,
      findingId,
      inventory,
      search,
      setSearch,
      openRun,
      openFinding,
      openInventory,
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
      runId,
      findingId,
      inventory,
      search,
      setSearch,
      openRun,
      openFinding,
      openInventory,
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

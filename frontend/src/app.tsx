import { type FormEvent, useEffect, useState } from "react";
import { navigationSections, Shell } from "./components/layout/Shell";
import { AppProvider, useApp } from "./context/AppContext";
import { ThemeProvider } from "./context/ThemeContext";
import {
  AuditRunsPage,
  DashboardPage,
  DocsPage,
  EnvironmentsPage,
  FindingsPage,
  InventoryPage,
  MappingsPage,
  PerformancePage,
  ReportsPage,
  RulesPage,
  SchemaDriftPage,
  SecurityPage,
  ServerComparePage,
  StatusPage,
} from "./pages";
import { api } from "./services/api";

export { navigationSections };

function AppRoutes({ onLogout }: { onLogout: () => void }) {
  const { section, setSection } = useApp();

  let content = <DashboardPage />;
  if (section === "Documentação") {
    content = <DocsPage onNavigate={setSection} />;
  } else if (section === "Ambientes") {
    content = <EnvironmentsPage onNavigate={setSection} />;
  } else if (section === "Inventário") {
    content = <InventoryPage />;
  } else if (section === "Execuções") {
    content = <AuditRunsPage />;
  } else if (section === "Relatórios") {
    content = <ReportsPage />;
  } else if (section === "Mapeamentos") {
    content = <MappingsPage />;
  } else if (section === "Desvio de schema") {
    content = <SchemaDriftPage />;
  } else if (section === "Comparar") {
    content = <ServerComparePage />;
  } else if (section === "Findings") {
    content = <FindingsPage />;
  } else if (section === "Performance") {
    content = <PerformancePage />;
  } else if (section === "Segurança") {
    content = <SecurityPage />;
  } else if (section === "Regras") {
    content = <RulesPage />;
  } else if (section === "Status") {
    content = <StatusPage />;
  }

  return (
    <Shell activeSection={section} onNavigate={setSection} onLogout={onLogout}>
      {content}
    </Shell>
  );
}

export function App() {
  const [user, setUser] = useState<{ username: string; role: string } | null>(
    null,
  );
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    const expire = () => setUser(null);
    window.addEventListener("auditor:session-expired", expire);
    return () => window.removeEventListener("auditor:session-expired", expire);
  }, []);
  const login = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      setUser(await api.login(username, password));
      setPassword("");
    } catch {
      setError("Não foi possível entrar. Confira usuário e senha.");
    } finally {
      setBusy(false);
    }
  };
  if (!user) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-slate-950 p-6 text-slate-100">
        <form
          onSubmit={(event) => void login(event)}
          className="w-full max-w-sm space-y-4 rounded-xl border border-slate-700 bg-slate-900 p-7"
        >
          <h1 className="text-xl font-semibold">Entrar no DB Auditor</h1>
          <p className="text-sm text-slate-400">
            Sua sessão permanece apenas nesta aba.
          </p>
          <label className="block text-sm">
            Usuário
            <input
              required
              autoComplete="username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              className="mt-1 w-full rounded border border-slate-600 bg-slate-950 p-2"
            />
          </label>
          <label className="block text-sm">
            Senha
            <input
              required
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              className="mt-1 w-full rounded border border-slate-600 bg-slate-950 p-2"
            />
          </label>
          {error ? (
            <p role="alert" className="text-sm text-rose-300">
              {error}
            </p>
          ) : null}
          <button
            disabled={busy}
            className="w-full rounded bg-cyan-600 px-4 py-2 font-medium disabled:opacity-50"
            type="submit"
          >
            {busy ? "Entrando…" : "Entrar"}
          </button>
        </form>
      </main>
    );
  }
  return (
    <ThemeProvider>
      <AppProvider>
        <AppRoutes
          onLogout={() => {
            void api.logout().finally(() => setUser(null));
          }}
        />
      </AppProvider>
    </ThemeProvider>
  );
}

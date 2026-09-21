import { navigationSections, Shell } from "./components/layout/Shell";
import { AppProvider, useApp } from "./context/AppContext";
import {
  AuditRunsPage,
  DashboardPage,
  DocsPage,
  EnvironmentsPage,
  FindingsPage,
  InventoryPage,
  MappingsPage,
  PerformancePage,
  SchemaDriftPage,
  SecurityPage,
  StatusPage,
} from "./pages";

export { navigationSections };

function AppRoutes() {
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
  } else if (section === "Mapeamentos") {
    content = <MappingsPage />;
  } else if (section === "Desvio de schema") {
    content = <SchemaDriftPage />;
  } else if (section === "Findings") {
    content = <FindingsPage />;
  } else if (section === "Performance") {
    content = <PerformancePage />;
  } else if (section === "Segurança") {
    content = <SecurityPage />;
  } else if (section === "Status") {
    content = <StatusPage />;
  }

  return (
    <Shell activeSection={section} onNavigate={setSection}>
      {content}
    </Shell>
  );
}

export function App() {
  return (
    <AppProvider>
      <AppRoutes />
    </AppProvider>
  );
}

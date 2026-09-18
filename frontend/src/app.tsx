import { useState } from "react";
import { navigationSections, Shell } from "./components/layout/Shell";
import {
  AuditRunsPage,
  DashboardPage,
  DocsPage,
  EnvironmentsPage,
  FindingsPage,
  InventoryPage,
  MappingsPage,
  OverviewPage,
  PerformancePage,
  SchemaDriftPage,
  SecurityPage,
  StatusPage,
} from "./pages";
import type { NavigationSection } from "./types";

export { navigationSections };

export function App() {
  const [section, setSection] = useState<NavigationSection>("Visão geral");

  let content = <OverviewPage onNavigate={setSection} />;
  if (section === "Dashboard") {
    content = <DashboardPage />;
  } else if (section === "Documentação") {
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

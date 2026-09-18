import { useState } from "react";
import { navigationSections, Shell } from "./components/layout/Shell";
import {
  AuditRunsPage,
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

  let content = <OverviewPage />;
  if (section === "Ambientes") {
    content = <EnvironmentsPage />;
  } else if (section === "Inventário") {
    content = <InventoryPage />;
  } else if (section === "Audit runs") {
    content = <AuditRunsPage />;
  } else if (section === "Mappings") {
    content = <MappingsPage />;
  } else if (section === "Schema Drift") {
    content = <SchemaDriftPage />;
  } else if (section === "Findings") {
    content = <FindingsPage />;
  } else if (section === "Performance") {
    content = <PerformancePage />;
  } else if (section === "Security") {
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

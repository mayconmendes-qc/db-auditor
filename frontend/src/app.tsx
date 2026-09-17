import { useState } from "react";
import { navigationSections, Shell } from "./components/layout/Shell";
import { EnvironmentsPage, OverviewPage } from "./pages";
import type { NavigationSection } from "./types";

export { navigationSections };

export function App() {
  const [section, setSection] = useState<NavigationSection>("Visão geral");

  return (
    <Shell activeSection={section} onNavigate={setSection}>
      {section === "Ambientes" ? <EnvironmentsPage /> : <OverviewPage />}
    </Shell>
  );
}

import { useState } from "react";
import { navigationSections, Shell } from "./components/layout/Shell";
import { EnvironmentsPage, InventoryPage, OverviewPage } from "./pages";
import type { NavigationSection } from "./types";

export { navigationSections };

export function App() {
  const [section, setSection] = useState<NavigationSection>("Visão geral");

  let content = <OverviewPage />;
  if (section === "Ambientes") {
    content = <EnvironmentsPage />;
  } else if (section === "Inventário") {
    content = <InventoryPage />;
  }

  return (
    <Shell activeSection={section} onNavigate={setSection}>
      {content}
    </Shell>
  );
}

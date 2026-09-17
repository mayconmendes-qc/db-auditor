import { navigationSections, Shell } from "./components/layout/Shell";
import { OverviewPage } from "./pages";

export { navigationSections };

export function App() {
  return (
    <Shell activeSection="Visão geral">
      <OverviewPage />
    </Shell>
  );
}

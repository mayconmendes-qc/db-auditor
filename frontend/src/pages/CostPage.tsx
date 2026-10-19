import { PageHeader } from "../components/PageHeader";
import { useApp } from "../context/AppContext";
import { DashboardCost } from "./parts/DashboardCost";

export function CostPage() {
  const { environmentId } = useApp();
  return (
    <>
      <PageHeader
        eyebrow="Análise"
        title="Custo estimado"
        description="Ranking de desperdícios (armazenamento, índice sem uso, bloat estimado e consultas) com valores estimado/mês."
      />
      <div className="mt-6">
        <DashboardCost environmentId={environmentId} />
      </div>
    </>
  );
}

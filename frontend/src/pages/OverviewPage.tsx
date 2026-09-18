import { PageHeader } from "../components/PageHeader";
import { Button, Card } from "../components/ui";
import type { NavigationSection } from "../types";

export interface OverviewPageProps {
  onNavigate?: (section: NavigationSection) => void;
}

export function OverviewPage({ onNavigate }: OverviewPageProps) {
  return (
    <>
      <PageHeader
        eyebrow="TIMESCALE AUDITOR"
        title="Auditoria com evidências, sem mudanças automáticas."
        description={
          <>
            O painel consome exclusivamente a API do Timescale Auditor. A coleta
            dos ambientes permanece somente leitura. Configure os bancos via{" "}
            <code className="text-slate-300">.env</code> e siga o guia de
            documentação para o primeiro uso.
          </>
        }
        actions={
          onNavigate ? (
            <>
              <Button type="button" onClick={() => onNavigate("Documentação")}>
                Ver documentação
              </Button>
              <Button type="button" onClick={() => onNavigate("Execuções")}>
                Executar auditoria
              </Button>
              <Button type="button" onClick={() => onNavigate("Dashboard")}>
                Dashboard
              </Button>
            </>
          ) : null
        }
      />

      <div className="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <Card
          subtitle="Configuração"
          title="Bancos via .env"
          onClick={onNavigate ? () => onNavigate("Documentação") : undefined}
        >
          <p className="text-sm text-slate-400">
            Connection strings e denylists fora do git; reinicie a API após
            alterar secrets.
          </p>
        </Card>
        <Card
          subtitle="Coleta"
          title="Execuções de auditoria"
          onClick={onNavigate ? () => onNavigate("Execuções") : undefined}
        >
          <p className="text-sm text-slate-400">
            Snapshots read-only no store interno, com status por collector.
          </p>
        </Card>
        <Card
          subtitle="Diagnóstico"
          title="Findings e desvios"
          onClick={onNavigate ? () => onNavigate("Findings") : undefined}
        >
          <p className="text-sm text-slate-400">
            Analyzers com evidências; triagem manual na UI.
          </p>
        </Card>
      </div>
    </>
  );
}

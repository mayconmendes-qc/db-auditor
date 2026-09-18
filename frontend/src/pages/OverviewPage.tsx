import { Button, Card } from "../components/ui";
import type { NavigationSection } from "../types";

export interface OverviewPageProps {
  onNavigate?: (section: NavigationSection) => void;
}

/** Landing page com atalhos para o fluxo principal. */
export function OverviewPage({ onNavigate }: OverviewPageProps) {
  return (
    <>
      <p className="text-xs font-bold tracking-[0.12em] text-emerald-300">
        TIMESCALЕ AUDITOR
      </p>
      <h1 className="mt-2 max-w-3xl text-3xl font-semibold leading-tight text-slate-50 md:text-4xl">
        Auditoria com evidências, sem mudanças automáticas.
      </h1>
      <p className="mt-4 max-w-3xl text-base leading-relaxed text-slate-400">
        O painel consome exclusivamente a API do Timescale Auditor. A coleta dos
        ambientes permanece somente leitura. Configure os bancos via{" "}
        <code className="text-slate-300">.env</code> e siga o guia de
        documentação para o primeiro uso.
      </p>

      <div className="mt-8 flex flex-wrap gap-3">
        {onNavigate ? (
          <>
            <Button type="button" onClick={() => onNavigate("Documentação")}>
              Ver documentação
            </Button>
            <Button type="button" onClick={() => onNavigate("Ambientes")}>
              Ambientes
            </Button>
            <Button type="button" onClick={() => onNavigate("Dashboard")}>
              Dashboard
            </Button>
          </>
        ) : null}
      </div>

      <div className="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <Card
          subtitle="Configuração"
          title="Bancos via .env"
        >
          <p className="text-sm text-slate-400">
            Connection strings e denylists fora do git; reinicie a API após
            alterar secrets.
          </p>
        </Card>
        <Card subtitle="Coleta" title="Execuções de auditoria">
          <p className="text-sm text-slate-400">
            Snapshots read-only no store interno, com status por collector.
          </p>
        </Card>
        <Card subtitle="Diagnóstico" title="Findings e desvios">
          <p className="text-sm text-slate-400">
            Analyzers com evidências; triagem manual na UI.
          </p>
        </Card>
      </div>
    </>
  );
}

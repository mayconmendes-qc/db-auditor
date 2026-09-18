import { Button, Card } from "../components/ui";
import type { NavigationSection } from "../types";

export interface DocsPageProps {
  onNavigate?: (section: NavigationSection) => void;
}

const steps: Array<{
  title: string;
  body: string;
  tip?: string;
  goTo?: NavigationSection;
}> = [
  {
    title: "1. Preparar o host",
    body: "Instale apenas Podman (ou Docker) com Compose. Copie .env.example para .env e defina POSTGRES_PASSWORD. Não é necessário instalar Go ou Bun na máquina.",
    tip: "Comandos: cp .env.example .env → make up",
  },
  {
    title: "2. Configurar bancos auditados (.env)",
    body: "As connection strings dos ambientes Timescale (Tiger Cloud ou self-hosted) ficam fora do repositório — em variáveis de ambiente ou secrets do backend. Use sempre credenciais somente leitura. Ajuste AUDITOR_DATABASE_DENYLIST e AUDITOR_SCHEMA_DENYLIST para excluir templates e schemas de sistema.",
    tip: "Nunca versionar senhas. Após mudar o .env, reinicie a API (make down && make up).",
  },
  {
    title: "3. Registrar e descobrir ambientes",
    body: "Ambientes aparecem em Ambientes após o seed/configuração no backend. Cada ambiente tem tipo (Tiger Cloud ou self-hosted) e modo de discovery (banco único ou múltiplos).",
    goTo: "Ambientes",
  },
  {
    title: "4. Disparar a primeira coleta",
    body: "Em Execuções, inicie um audit run com perfil manual ou fast. Os coletores são somente leitura e gravam snapshots no store interno. Falhas parciais de um collector não impedem o restante.",
    goTo: "Execuções",
  },
  {
    title: "5. Explorar o inventário",
    body: "Use Inventário para navegar databases, schemas, tabelas, hypertables, índices, views, funções, CAGGs, jobs e policies. Filtros e paginação são server-side.",
    goTo: "Inventário",
  },
  {
    title: "6. Mapear e comparar (dois ambientes)",
    body: "Com pelo menos dois ambientes coletados, use Mapeamentos para relacionar objetos e Desvio de schema para ver match, drift, only_source e only_target.",
    goTo: "Mapeamentos",
  },
  {
    title: "7. Triar findings",
    body: "Analyzers geram findings com severidade e evidências. Em Findings, reconheça, resolva ou suprima itens. Nada é alterado automaticamente nos bancos auditados.",
    goTo: "Findings",
  },
  {
    title: "8. Acompanhar saúde e KPIs",
    body: "Dashboard resume storage, findings e jobs. Status mostra API, store e runs recentes. Em produção, métricas Prometheus ficam em GET /metrics.",
    goTo: "Dashboard",
  },
];

/** Guia de uso e fluxo de configuração (via .env). */
export function DocsPage({ onNavigate }: DocsPageProps) {
  return (
    <>
      <p className="text-xs font-bold tracking-[0.12em] text-emerald-300">
        GUIA
      </p>
      <h1 className="mt-2 text-3xl font-semibold text-slate-50 md:text-4xl">
        Como usar o Timescale Auditor
      </h1>
      <p className="mt-3 max-w-3xl text-sm leading-relaxed text-slate-400">
        Fluxo recomendado para configurar os bancos auditados (somente via{" "}
        <code className="text-slate-300">.env</code> / secrets), coletar
        inventário e triar findings. A aplicação nunca aplica mudanças
        automáticas nos ambientes Timescale.
      </p>

      <ol className="mt-8 grid gap-4 lg:grid-cols-2">
        {steps.map((step) => (
          <li key={step.title}>
            <Card title={step.title}>
              <p className="text-sm leading-relaxed text-slate-300">{step.body}</p>
              {step.tip ? (
                <p className="mt-2 text-xs text-slate-500">{step.tip}</p>
              ) : null}
              {step.goTo && onNavigate ? (
                <div className="mt-3">
                  <Button type="button" onClick={() => onNavigate(step.goTo!)}>
                    Ir para {step.goTo}
                  </Button>
                </div>
              ) : null}
            </Card>
          </li>
        ))}
      </ol>

      <section className="mt-10 rounded-xl border border-slate-800 bg-slate-900/50 p-5">
        <h2 className="text-sm font-semibold text-slate-100">
          Checklist rápido após make up
        </h2>
        <ul className="mt-3 list-inside list-disc space-y-1 text-sm text-slate-400">
          <li>
            <code className="text-slate-300">make smoke</code> — health e ready
          </li>
          <li>UI Status — API e snapshot store verdes</li>
          <li>Ambientes listados (seed/config)</li>
          <li>Primeira execução de auditoria concluída</li>
          <li>
            Após mudar o baseline SQL:{" "}
            <code className="text-slate-300">make reset-volume && make up</code>
          </li>
        </ul>
      </section>
    </>
  );
}

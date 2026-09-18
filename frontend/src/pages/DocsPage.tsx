import { PageHeader } from "../components/PageHeader";
import { Button, Card } from "../components/ui";
import type { NavigationSection } from "../types";

export interface DocsPageProps {
  onNavigate?: (section: NavigationSection) => void;
}

const steps: Array<{
  n: number;
  title: string;
  body: string;
  tip?: string;
  goTo?: NavigationSection;
}> = [
  {
    n: 1,
    title: "Preparar o host",
    body: "Instale apenas Podman (ou Docker) com Compose. Copie .env.example para .env e defina POSTGRES_PASSWORD. Não é necessário instalar Go ou Bun na máquina.",
    tip: "Comandos: cp .env.example .env → make up",
  },
  {
    n: 2,
    title: "Configurar bancos auditados (.env)",
    body: "As connection strings dos ambientes Timescale (Tiger Cloud ou self-hosted) ficam fora do repositório — em variáveis de ambiente ou secrets do backend. Use sempre credenciais somente leitura. Ajuste AUDITOR_DATABASE_DENYLIST e AUDITOR_SCHEMA_DENYLIST para excluir templates e schemas de sistema.",
    tip: "Nunca versionar senhas. Após mudar o .env, reinicie a API (make down && make up).",
  },
  {
    n: 3,
    title: "Registrar e descobrir ambientes",
    body: "Ambientes aparecem em Ambientes após o seed/configuração no backend. Cada ambiente tem tipo (Tiger Cloud ou self-hosted) e modo de discovery (banco único ou múltiplos).",
    goTo: "Ambientes",
  },
  {
    n: 4,
    title: "Disparar a primeira coleta",
    body: "Em Execuções, inicie um audit run com perfil manual ou fast. Os coletores são somente leitura e gravam snapshots no store interno. Falhas parciais de um collector não impedem o restante.",
    goTo: "Execuções",
  },
  {
    n: 5,
    title: "Explorar o inventário",
    body: "Use Inventário para navegar databases, schemas, tabelas, hypertables, índices, views, funções, CAGGs, jobs e policies. Filtros e paginação são server-side. Prefira o filtro (todos) para ver objetos do DSN.",
    goTo: "Inventário",
  },
  {
    n: 6,
    title: "Mapear e comparar (dois ambientes)",
    body: "Com pelo menos dois ambientes coletados, use Mapeamentos para sugerir e validar correspondências por fingerprint, e Desvio de schema para ver match, drift, only_source e only_target.",
    goTo: "Mapeamentos",
  },
  {
    n: 7,
    title: "Triar findings",
    body: "Analyzers geram findings com severidade e evidências. Em Findings, reconheça, resolva ou suprima itens. Nada é alterado automaticamente nos bancos auditados.",
    goTo: "Findings",
  },
  {
    n: 8,
    title: "Acompanhar saúde e KPIs",
    body: "Dashboard resume storage, findings e jobs. Status mostra API, store e runs recentes. Em produção, métricas Prometheus ficam em GET /metrics.",
    goTo: "Dashboard",
  },
];

export function DocsPage({ onNavigate }: DocsPageProps) {
  return (
    <>
      <PageHeader
        eyebrow="GUIA"
        title="Como usar o Timescale Auditor"
        description={
          <>
            Fluxo recomendado para configurar os bancos auditados (somente via{" "}
            <code className="rounded bg-slate-800 px-1 text-slate-200">
              .env
            </code>{" "}
            / secrets), coletar inventário e triar findings. A aplicação nunca
            aplica mudanças automáticas nos ambientes Timescale.
          </>
        }
      />

      <ol className="mt-8 grid auto-rows-fr gap-4 lg:grid-cols-2">
        {steps.map((step) => {
          const target = step.goTo;
          return (
            <li key={step.n} className="h-full">
              <Card className="h-full">
                <div className="flex items-start gap-3">
                  <span
                    className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-emerald-500/15 text-sm font-bold text-emerald-300 ring-1 ring-emerald-500/30"
                    aria-hidden
                  >
                    {step.n}
                  </span>
                  <div className="min-w-0 flex-1">
                    <h2 className="text-base font-semibold text-slate-50">
                      {step.title}
                    </h2>
                    <p className="mt-2 text-sm leading-relaxed text-slate-300">
                      {step.body}
                    </p>
                    {step.tip ? (
                      <p className="mt-3 rounded-md border border-amber-500/20 bg-amber-500/5 px-3 py-2 text-xs leading-relaxed text-amber-200/90">
                        <span className="font-semibold text-amber-300">
                          Dica:{" "}
                        </span>
                        {step.tip}
                      </p>
                    ) : null}
                  </div>
                </div>
                <div className="mt-4 flex flex-1 items-end">
                  {target && onNavigate ? (
                    <Button type="button" onClick={() => onNavigate(target)}>
                      Ir para {target}
                    </Button>
                  ) : (
                    <span className="text-xs text-slate-500">
                      Configuração local
                    </span>
                  )}
                </div>
              </Card>
            </li>
          );
        })}
      </ol>

      <div className="mt-10">
        <Card title="Checklist rápido após make up">
          <ul className="mt-1 space-y-2 text-sm text-slate-300">
            <li className="flex gap-2">
              <span className="text-emerald-400">✓</span>
              <span>
                <code className="rounded bg-slate-800 px-1 text-slate-200">
                  make smoke
                </code>{" "}
                — health e ready
              </span>
            </li>
            <li className="flex gap-2">
              <span className="text-emerald-400">✓</span>
              <span>UI Status — API e snapshot store verdes</span>
            </li>
            <li className="flex gap-2">
              <span className="text-emerald-400">✓</span>
              <span>Ambientes listados (seed/config)</span>
            </li>
            <li className="flex gap-2">
              <span className="text-emerald-400">✓</span>
              <span>Primeira execução de auditoria concluída</span>
            </li>
            <li className="flex gap-2">
              <span className="text-emerald-400">✓</span>
              <span>
                Após mudar o baseline SQL:{" "}
                <code className="rounded bg-slate-800 px-1 text-slate-200">
                  make reset-volume && make up
                </code>
              </span>
            </li>
          </ul>
          {onNavigate ? (
            <div className="mt-4 flex flex-wrap gap-2">
              <Button onClick={() => onNavigate("Status")}>
                Ir para Status
              </Button>
              <Button
                variant="secondary"
                onClick={() => onNavigate("Execuções")}
              >
                Ir para Execuções
              </Button>
            </div>
          ) : null}
        </Card>
      </div>
    </>
  );
}

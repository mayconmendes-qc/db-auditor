import { PageHeader } from "../components/PageHeader";
import { Accordion, Button } from "../components/ui";
import type { NavigationSection } from "../types";

export interface DocsPageProps {
  onNavigate?: (section: NavigationSection) => void;
}

function Term({ name, children }: { name: string; children: string }) {
  return (
    <li className="text-sm text-slate-300">
      <span className="font-medium text-slate-100">{name}</span>
      {" — "}
      {children}
    </li>
  );
}

function SectionBody({
  howTo,
  terms,
  goTo,
  onNavigate,
}: {
  howTo: string[];
  terms: Array<{ name: string; def: string }>;
  goTo?: NavigationSection;
  onNavigate?: (section: NavigationSection) => void;
}) {
  return (
    <div className="space-y-4">
      <div>
        <h4 className="text-xs font-medium text-slate-400">Como usar</h4>
        <ol className="mt-2 list-decimal space-y-1.5 pl-4 text-sm text-slate-300">
          {howTo.map((step) => (
            <li key={step}>{step}</li>
          ))}
        </ol>
      </div>
      {terms.length > 0 ? (
        <div>
          <h4 className="text-xs font-medium text-slate-400">
            Termos desta tela
          </h4>
          <ul className="mt-2 space-y-1.5">
            {terms.map((t) => (
              <Term key={t.name} name={t.name}>
                {t.def}
              </Term>
            ))}
          </ul>
        </div>
      ) : null}
      {goTo && onNavigate ? (
        <Button type="button" onClick={() => onNavigate(goTo)}>
          Ir para {goTo}
        </Button>
      ) : null}
    </div>
  );
}

export function DocsPage({ onNavigate }: DocsPageProps) {
  const items = [
    {
      id: "dashboard",
      title: "Dashboard",
      content: (
        <SectionBody
          goTo="Dashboard"
          onNavigate={onNavigate}
          howTo={[
            "Use o seletor de Ambiente na barra lateral para filtrar KPIs e gráficos (ou deixe em Todos).",
            "Confira o status de conexão de cada ambiente no topo (Conectado / Indisponível / DSN ausente).",
            "Clique nos cards de KPI (Findings, Storage, Runs) para ir à tela correspondente.",
            "O gráfico de pizza mostra a distribuição de storage: a legenda traz o armazenamento em bytes; o percentual aparece só nas fatias do gráfico.",
          ]}
          terms={[
            {
              name: "KPI",
              def: "Indicador resumido (ex.: findings abertos, hypertables, runs ok/falha).",
            },
            {
              name: "DSN",
              def: "Connection string do banco auditado, configurada via variáveis de ambiente no backend.",
            },
            {
              name: "Storage por ambiente",
              def: "Soma do tamanho dos objetos coletados no último inventário de cada ambiente.",
            },
          ]}
        />
      ),
    },
    {
      id: "ambientes",
      title: "Ambientes",
      content: (
        <SectionBody
          goTo="Ambientes"
          onNavigate={onNavigate}
          howTo={[
            "Liste os ambientes registrados no store interno (seed ou configuração do backend).",
            "Cada ambiente representa um alvo Timescale (Tiger Cloud ou self-hosted).",
            "As credenciais nunca são editadas na UI — apenas via .env / secrets.",
            "Após alterar DSN no host, reinicie a API e valide em Status / Dashboard.",
          ]}
          terms={[
            {
              name: "Ambiente",
              def: "Alvo lógico de auditoria com nome, tipo e modo de discovery.",
            },
            {
              name: "Discovery",
              def: "Modo de descoberta de databases (único ou múltiplos) no servidor Postgres/Timescale.",
            },
            {
              name: "Somente leitura",
              def: "A aplicação só consulta metadados; não aplica DDL/DML nos bancos auditados.",
            },
          ]}
        />
      ),
    },
    {
      id: "execucoes",
      title: "Execuções",
      content: (
        <SectionBody
          goTo="Execuções"
          onNavigate={onNavigate}
          howTo={[
            "Em Disparo manual, escolha o ambiente e clique em Executar agora (confirmação via modal).",
            "Filtre a lista por Status e Perfil (largura total, lado a lado).",
            "Selecione uma run na tabela para ver detalhe, erros e progresso por collector.",
            "Enquanto houver runs em execução, a lista atualiza automaticamente.",
          ]}
          terms={[
            {
              name: "Audit run",
              def: "Uma execução de coleta que orquestra vários collectors e grava snapshots.",
            },
            {
              name: "Collector",
              def: "Unidade de coleta (ex.: tables, indexes, hypertables, policies) dentro de uma run.",
            },
            {
              name: "Perfil",
              def: "Agenda ou modo da run: manual, fast, daily, weekly, monthly.",
            },
            {
              name: "Sucesso parcial",
              def: "Alguns collectors falharam, mas outros concluíram e persistiram dados.",
            },
          ]}
        />
      ),
    },
    {
      id: "inventario",
      title: "Inventário",
      content: (
        <SectionBody
          goTo="Inventário"
          onNavigate={onNavigate}
          howTo={[
            "Selecione um ambiente na sidebar (obrigatório para carregar snapshots).",
            "Use o card Filtros: Database, Schema e busca por nome.",
            "Alterne o tipo de objeto (Tabelas, Hypertables, Índices, Views, Funções, CAGGs).",
            "Clique em uma linha para ver o JSON de detalhe do objeto à direita (em telas largas).",
          ]}
          terms={[
            {
              name: "Snapshot",
              def: "Cópia pontual dos metadados coletados em uma audit run.",
            },
            {
              name: "Hypertable",
              def: "Tabela Timescale particionada no tempo (ou outra dimensão).",
            },
            {
              name: "CAGG",
              def: "Continuous Aggregate — view materializada mantida pelo Timescale.",
            },
          ]}
        />
      ),
    },
    {
      id: "mapeamentos",
      title: "Mapeamentos",
      content: (
        <SectionBody
          goTo="Mapeamentos"
          onNavigate={onNavigate}
          howTo={[
            "Requer pelo menos dois ambientes com inventário coletado.",
            "Use a sugestão automática por fingerprint para propor correspondências.",
            "Revise e valide mapeamentos antes de comparar schema drift.",
            "Objetos sem par ficam disponíveis para associação manual quando aplicável.",
          ]}
          terms={[
            {
              name: "Fingerprint",
              def: "Assinatura normalizada do objeto (nome, tipo, colunas, etc.) usada para matching.",
            },
            {
              name: "Canonical object ID",
              def: "Identificador estável do objeto no domínio do auditor, independente do ambiente.",
            },
            {
              name: "Suggest mappings",
              def: "Algoritmo que propõe pares source/target com base em similaridade de fingerprints.",
            },
          ]}
        />
      ),
    },
    {
      id: "desvio",
      title: "Desvio de schema",
      content: (
        <SectionBody
          goTo="Desvio de schema"
          onNavigate={onNavigate}
          howTo={[
            "Escolha par de ambientes (origem e destino) já mapeados ou comparáveis.",
            "Analise categorias: match, drift, only_source e only_target.",
            "Use o resultado como evidência — a UI não aplica correções nos bancos.",
          ]}
          terms={[
            {
              name: "Match",
              def: "Objetos equivalentes nos dois lados, sem diferença relevante de schema.",
            },
            {
              name: "Drift",
              def: "Objetos mapeados com diferenças (colunas, tipos, índices, etc.).",
            },
            {
              name: "Only source / only target",
              def: "Objeto presente só no ambiente de origem ou só no de destino.",
            },
          ]}
        />
      ),
    },
    {
      id: "findings",
      title: "Findings",
      content: (
        <SectionBody
          goTo="Findings"
          onNavigate={onNavigate}
          howTo={[
            "Filtre por severidade, status e tipo de analyzer.",
            "Abra um finding para ler evidências e contexto do objeto.",
            "Triagem: reconheça, resolva ou suprima em lote quando fizer sentido.",
            "Exporte CSV/JSON para auditoria externa, se necessário.",
          ]}
          terms={[
            {
              name: "Finding",
              def: "Achado gerado por um analyzer com severidade, evidências e status de triagem.",
            },
            {
              name: "Analyzer",
              def: "Regra automatizada (CAGG, policy, vacuum, segurança, etc.) que produz findings.",
            },
            {
              name: "Severidade",
              def: "critical, high, medium, low — prioridade relativa do risco ou da inconsistência.",
            },
            {
              name: "Suprimir",
              def: "Marcar o finding como não acionável agora, sem apagar o histórico.",
            },
          ]}
        />
      ),
    },
    {
      id: "performance",
      title: "Performance",
      content: (
        <SectionBody
          goTo="Performance"
          onNavigate={onNavigate}
          howTo={[
            "Revise findings e métricas ligados a vacuum, bloat e consultas problemáticas.",
            "Correlacione com Inventário (tamanho de tabelas/índices) e Execuções recentes.",
            "Nada é executado no banco alvo — use as evidências para planejar manutenção.",
          ]}
          terms={[
            {
              name: "Vacuum",
              def: "Manutenção Postgres que recupera espaço e atualiza estatísticas.",
            },
            {
              name: "Query fingerprint",
              def: "Forma sanitizada da consulta (sem literais) para agrupar padrões semelhantes.",
            },
          ]}
        />
      ),
    },
    {
      id: "seguranca",
      title: "Segurança",
      content: (
        <SectionBody
          goTo="Segurança"
          onNavigate={onNavigate}
          howTo={[
            "Consulte findings de segurança (permissões amplas, roles, extensões sensíveis, etc.).",
            "Priorize critical/high e valide no ambiente de origem com a equipe de DBA.",
            "A auditoria é passiva: não revoga grants nem altera roles automaticamente.",
          ]}
          terms={[
            {
              name: "Finding de segurança",
              def: "Achado relacionado a privilégios, exposição ou configuração insegura.",
            },
            {
              name: "Evidência",
              def: "Dados coletados que sustentam o finding (objeto, grant, configuração).",
            },
          ]}
        />
      ),
    },
    {
      id: "status",
      title: "Status",
      content: (
        <SectionBody
          goTo="Status"
          onNavigate={onNavigate}
          howTo={[
            "Verifique saúde da API, store interno e runs recentes.",
            "Após make up ou mudança de .env, confirme que API e conexões estão verdes.",
            "Em produção, métricas Prometheus ficam em GET /metrics (fora desta UI).",
          ]}
          terms={[
            {
              name: "Health / Ready",
              def: "Endpoints de liveness e readiness da API do auditor.",
            },
            {
              name: "Snapshot store",
              def: "Banco interno onde inventário, runs e findings são persistidos.",
            },
          ]}
        />
      ),
    },
    {
      id: "setup",
      title: "Configuração inicial (ops)",
      content: (
        <SectionBody
          howTo={[
            "Instale Podman (ou Docker) com Compose. Copie .env.example para .env e defina POSTGRES_PASSWORD.",
            "Configure AUDITOR_TARGET_DSN_* e denylists de database/schema fora do repositório.",
            "Suba com make up; valide com make smoke e a tela Status.",
            "Após mudar baseline SQL: make reset-volume && make up.",
          ]}
          terms={[
            {
              name: "Denylist",
              def: "Lista de databases/schemas excluídos da coleta (templates, sistema).",
            },
            {
              name: "Seed demo",
              def: "Dados de exemplo carregados em volume vazio para demonstração local.",
            },
          ]}
        />
      ),
    },
  ];

  return (
    <>
      <PageHeader
        eyebrow="Guia"
        title="Como usar o DB Auditor"
        description={
          <>
            Documentação por tela: como operar cada página e o significado dos
            termos. A aplicação nunca aplica mudanças automáticas nos ambientes
            Timescale auditados.
          </>
        }
      />

      <div className="mt-8">
        <Accordion items={items} defaultOpenId="dashboard" />
      </div>
    </>
  );
}

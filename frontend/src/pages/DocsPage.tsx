import { PageHeader } from "../components/PageHeader";
import { Accordion, Button } from "../components/ui";
import type { NavigationSection } from "../types";

export interface DocsPageProps {
  onNavigate?: (section: NavigationSection) => void;
}

function Term({ name, children }: { name: string; children: string }) {
  return (
    <li className="text-sm leading-relaxed text-slate-300">
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
    <div className="space-y-8">
      <div>
        <h4 className="text-xs font-medium tracking-wide text-slate-400">
          Como usar
        </h4>
        <ol className="mt-3 list-decimal space-y-3 pl-5 text-sm leading-relaxed text-slate-300">
          {howTo.map((step) => (
            <li key={step} className="pl-1">
              {step}
            </li>
          ))}
        </ol>
      </div>
      {terms.length > 0 ? (
        <div>
          <h4 className="text-xs font-medium tracking-wide text-slate-400">
            Termos desta tela
          </h4>
          <ul className="mt-3 space-y-3">
            {terms.map((t) => (
              <Term key={t.name} name={t.name}>
                {t.def}
              </Term>
            ))}
          </ul>
        </div>
      ) : null}
      {goTo && onNavigate ? (
        <div className="pt-1">
          <Button type="button" onClick={() => onNavigate(goTo)}>
            Ir para {goTo}
          </Button>
        </div>
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
            "Total de Databases, Schemas e Tabelas vem da última execução coerente de cada ambiente. Todos soma os ambientes; aviso de cobertura parcial indica que algum inventário está ausente ou incompleto.",
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
            "A cobertura identifica databases coletados e falhas. Uma execução ativa pode ser cancelada por operador; o cancelamento é cooperativo e os dados já persistidos permanecem auditáveis.",
            "No detalhe, compare a execução com o baseline aprovado e acompanhe o score do escopo. Aprovar novo baseline exige confirmação e cobertura elegível.",
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
            "Clique em uma linha para abrir detalhes, assessment, histórico e métricas do objeto. A avaliação não modifica o banco auditado.",
            "Feche o painel de detalhes pelo botão, pelo fundo ou com Esc. O link volta para o inventário sem a tabela selecionada.",
            "Ajuste linhas por página entre 20, 50 e 100. O inventário carrega uma página por vez; refine os filtros em ambientes grandes.",
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
            "Filtre por severidade e status usando os seletores em português.",
            "Abra um finding para ler evidências e contexto do objeto.",
            "Triagem: reconheça, resolva ou suprima em lote quando fizer sentido.",
            "Consulte o histórico imutável antes de mudar o status. Supressões têm motivo e validade; uma reincidência após a validade volta a ser visível.",
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
              def: "Crítica, alta, média, baixa ou informativo — prioridade relativa do risco.",
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
            "Consulte findings de segurança (funções com privilégios elevados, roles e permissões amplas).",
            "Priorize critical/high e valide no ambiente de origem com a equipe de DBA.",
            "A auditoria é passiva: não revoga grants nem altera roles automaticamente.",
          ]}
          terms={[
            {
              name: "Funções com privilégios elevados",
              def: "Funções SECURITY DEFINER que executam com o dono da função, não com o chamador.",
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
      id: "relatorios",
      title: "Relatórios",
      content: (
        <SectionBody
          goTo="Relatórios"
          onNavigate={onNavigate}
          howTo={[
            "Escolha ambiente, execução concluída e tipo Executivo, Técnico ou Tabela.",
            "Um relatório de tabela exige database, schema e tabela. A geração é assíncrona; acompanhe a fila, cancele ou tente novamente quando permitido.",
            "Baixe o PDF somente enquanto o artefato estiver válido. O histórico mantém o solicitante e a versão das regras; o PDF expira após 30 dias.",
            "O PDF mostra os achados por severidade e termina com próximos passos. Confira a cobertura e valide as recomendações com a equipe antes de alterar o banco.",
          ]}
          terms={[
            {
              name: "Artefato",
              def: "PDF persistido com hash para validar a integridade do download.",
            },
            {
              name: "Redação de metadados",
              def: "A configuração do servidor pode ocultar ambiente, ID da execução, solicitante e versões do cabeçalho do PDF.",
            },
          ]}
        />
      ),
    },
    {
      id: "acesso",
      title: "Contas e permissões",
      content: (
        <SectionBody
          howTo={[
            "Entre com uma conta local; a sessão usa cookie protegido, é verificada ao recarregar a página e pode ser encerrada em Sair.",
            "Viewer consulta os ambientes atribuídos; auditor também faz triagem e solicita relatórios; operator gerencia execuções, baselines e contas.",
            "Um operator cria outras contas pela API administrativa, definindo role e IDs de ambientes. Segredos de conexão e senhas não são expostos na interface.",
          ]}
          terms={[
            {
              name: "Sessão",
              def: "Token temporário e revogável, válido por até 8 horas.",
            },
            {
              name: "Escopo",
              def: "Conjunto de ambientes explicitamente associado a uma conta não-operadora.",
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
            "Em produção, métricas Prometheus ficam na porta interna 9090, fora do site público.",
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
            "O usuário e a senha ficam no .env: AUDITOR_BOOTSTRAP_USER=admin e AUDITOR_BOOTSTRAP_PASSWORD=db-auditor-local-1. A senha precisa ter pelo menos 16 caracteres.",
            "Suba com make up e entre em http://localhost:5173. A conta operator só nasce se ainda não existir nenhum usuário. Mudar o .env depois não troca a senha já gravada.",
            "Configure os alvos com sslmode=verify-full e AUDITOR_TARGET_ALLOWED_HOSTS. Use conta somente leitura e denylists de database/schema. Host e senha dos Timescale ficam no .env, não numa migration.",
            "O schema é 01_baseline.sql mais 02_seed_demo.sql. Não use reset-volume em produção: ele apaga os dados.",
          ]}
          terms={[
            {
              name: "Denylist",
              def: "Lista de databases/schemas excluídos da coleta (templates, sistema).",
            },
            {
              name: "Seed demo",
              def: "Duas linhas locais de audit_environment, só em volume vazio. Não guarda host nem senha dos Timescale; isso vem do .env.",
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

      <div className="mt-10 w-full">
        <Accordion items={items} defaultOpenId="dashboard" />
      </div>
    </>
  );
}

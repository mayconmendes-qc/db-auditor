# 03 — Contratos de qualidade de dados (`dq-contracts`)

## 0. Contexto e problema, objetivo, fora de escopo

**Problema.** Bugs silenciosos de dados (nulos inesperados, duplicatas, órfãos, dados parados) chegam ao BI e ao cliente. Great Expectations e Soda são pesados e centrados em Python ou em data warehouse. O db-auditor já tem um **diagnóstico de qualidade sob demanda** (uma tabela, até 1000 linhas), mas não tem regra declarada, agenda, gate de CI nem tendência.

**Objetivo.** Declarar contratos versionados por tabela (arquivo YAML ou edição na UI), executá-los com amostragem segura por agenda ou em CI, guardar só contagens e mostrar histórico e tendência.

**Fora de escopo.** Correção de dados. Checagens com expressão livre do usuário (SQL arbitrário). Data warehouses e MongoDB. Profiling estatístico completo.

## 1. Requisitos funcionais

- **RF-01** Contrato por ambiente e tabela, versionado: cada alteração gera uma nova versão imutável, com autor e data. Formato de troca em YAML, com esquema publicado na documentação.
- **RF-02** Seis tipos de checagem:
  - (1) % de nulos máximo por coluna;
  - (2) unicidade de coluna ou chave composta;
  - (3) FK órfã em relação simples;
  - (4) domínio: lista permitida ou intervalo numérico/data;
  - (5) frescor: idade máxima do maior valor de uma coluna temporal;
  - (6) volume: variação de linhas estimadas entre coletas dentro de uma faixa.
- **RF-03** Gerar o contrato inicial a partir do schema já coletado:
  - colunas obrigatórias → nulos 0%;
  - PK e unicidade → unicidade;
  - FKs → órfãs;
  - coluna temporal de atualização reconhecida por nome → frescor.

  O resultado é uma proposta que um auditor aprova. Nunca entra em vigor sozinho.
- **RF-04** Execução por agenda (perfis existentes do agendador), sob demanda na UI e em modo CLI para CI, com gate por severidade.
- **RF-05** Amostragem por páginas com limite configurável por contrato. Frescor e volume usam agregado ou estatística, sem amostra.
- **RF-06** Resultado por checagem: aprovado, violado ou não executado (com motivo), contagens observadas, tamanho da amostra e método.
- **RF-07** Violação gera achado (`quality.contract_violation`) com dedup por contrato, checagem e versão. Violação persistente em duas execuções completas entra na regressão (AN-11) e na notificação existente.
- **RF-08** Histórico e tendência por checagem (série de % e contagens) na UI.
- **RF-09** Relatório Markdown (CLI e download) e seção no PDF técnico.

## 2. Requisitos não funcionais

- **RNF-01 Segurança e só leitura.** Mesmo pré-requisito do diagnóstico atual: recurso habilitado pelo operator (`AUDITOR_DATA_QUALITY_ENABLED`) e conta de leitura sem escrita. Toda leitura em transação só leitura, com timeout por checagem.
- **RNF-02 LGPD.** Persistir só contagens, proporções, tamanho da amostra e a idade em horas do frescor (nunca o valor máximo em si). Valores de linha nunca saem da memória do processo. A lista de domínio é escrita pelo usuário no contrato e é tratada como metadado, mas aparece só para auditor ou acima.
- **RNF-03 Desempenho e limites.** Amostra padrão de 10 000 linhas, máximo de 100 000 por checagem (**valor inicial, calibrar**). Timeout de 30 s por checagem. No máximo 50 checagens por contrato e 4 checagens simultâneas por ambiente. Tabelas acima de 1 GB exigem amostragem por páginas.
- **RNF-04 Multi-tenant.** Contratos, execuções e achados pertencem ao ambiente. Viewer lê, auditor cria, aprova e executa, operator habilita o recurso.
- **RNF-05 Observabilidade.** Métricas de checagens por resultado e duração. Log sem valores de linha.
- **RNF-06 Honestidade estatística.** Toda contagem amostral mostra o tamanho da amostra e o aviso de margem de erro desconhecida, mesmo padrão do diagnóstico atual.

## 3. Regras importantes

| Checagem | Violação | Severidade padrão | Observação |
|---|---|---|---|
| Nulos | % observado > máximo do contrato | média | alta se coluna marcada "crítica" no contrato |
| Unicidade | ≥ 1 duplicata na amostra | alta | amostra não prova unicidade; aprovado = "sem duplicata na amostra" |
| FK órfã | ≥ 1 órfã na amostra | média | só relação simples (uma coluna), como no diagnóstico atual |
| Domínio | % fora do domínio > 0% (tolerância configurável) | média | — |
| Frescor | idade > máximo do contrato | alta | padrão sugerido 24 h, **valor inicial, calibrar** |
| Volume | variação entre coletas fora de ±50% | baixa | **valor inicial, calibrar**; usa linhas estimadas, então tem confiança baixa |
| Não executado | timeout, sem permissão, tabela ausente | informativa | conta como dado insuficiente (código 3 na CLI) |

Gate de CI padrão: bloqueia a partir de alta.

## 4. Respostas esperadas

- **Entradas.** Contrato (YAML ou UI), ambiente, tabela, modo (agenda, sob demanda, CLI) e severidade de bloqueio.
- **UI.** Em Ações assistidas → Qualidade: lista de contratos por tabela, com estado da última execução, tendência, botão "gerar proposta" e diff entre versões do contrato.
- **API.** Criar, versionar e aprovar contratos (auditor). Executar (auditor). Ler resultados e histórico (viewer). Importar e exportar YAML.
- **CLI.** Executa um contrato ou diretório de contratos contra um ambiente. Sai em Markdown ou JSON, com os códigos do README.
- **Markdown.** Tabela por checagem com resultado, contagem, amostra e severidade.
- **PDF.** Seção "Contratos de qualidade" no técnico.
- **Erros.**
  - YAML inválido: 400 ou código 2, com linha e campo;
  - recurso desabilitado: 409 com mensagem;
  - coluna inexistente: checagem "não executada".

## 5. Critérios de aceite

- **CA-01** Dado uma tabela com uma coluna obrigatória e uma FK no snapshot, quando um auditor pede a proposta, então o contrato sugerido traz nulos 0% e FK órfã, e fica "proposto", sem vigorar.
- **CA-02** Dado um contrato aprovado com nulos máximo de 1% e uma amostra com 3% de nulos, quando a execução termina, então o resultado guarda 3%, o tamanho da amostra e um achado médio, e nenhum valor de linha é persistido.
- **CA-03** Dado o recurso desabilitado, quando alguém tenta executar um contrato, então a API responde 409 e a CLI sai com código 2.
- **CA-04** Dado uma checagem de frescor com máximo de 24 h e o último registro de 30 h atrás, quando executa, então o resultado guarda "30 h" e não o carimbo de data.
- **CA-05** Dado a CLI com gate em alta e uma violação de unicidade, quando roda em CI, então sai com código 1 e o Markdown lista a violação.
- **CA-06** Dado uma checagem que excede o timeout, quando a execução termina, então ela aparece "não executada — timeout" e a CLI sai com código 3.
- **CA-07** Dado a mesma violação em duas execuções completas seguidas, quando a regressão é avaliada, então um único alerta é criado (dedup existente).
- **CA-08** Dado um viewer, quando ele tenta aprovar um contrato, então recebe 403.

## 6. Dependências, reaproveitamento, riscos e perguntas em aberto

**Reaproveita.**
- `internal/quality`: checagens null, duplicate, orphan, date_range e distribuição, os textos de plano, as tabelas `quality_scan`, `quality_issue` e `quality_issue_event`, e as rotas `quality-scans` (papel auditor);
- a flag `AUDITOR_DATA_QUALITY_ENABLED`;
- `column_snapshot`, `constraint_snapshot`, `table_snapshot` (linhas estimadas para volume) e `column_stat_snapshot` (`null_fraction` de `pg_stats`, como pré-checagem de nulos sem ler linhas);
- `internal/scheduler`, regressão e notificação (AN-11, `internal/notify`);
- ações assistidas e medições, para validar correções;
- o PDF.

**Novo.**
- entidade contrato versionada;
- checagens de domínio, frescor e volume;
- geração de proposta;
- limite de amostra configurável acima de 1000;
- execução agendada;
- modo CLI com gate;
- tendência.

**Dependências.** 01 (modo CLI e credencial de CI). 05 (endurecimento da amostragem por páginas, compartilhada).

**Riscos.**
- Amostra pequena gera falso "aprovado".
- Carga de leitura em produção.
- Contrato desatualizado após migration. Mitigação: alerta de coluna ausente, com a timeline da 04.

**Perguntas em aberto.**
- P-01: o limite atual de 1000 linhas sobe para até 100 000 por contrato, ou contratos ficam com o mesmo teto?
- P-02: o contrato canônico fica no auditor (UI) ou no repositório da aplicação (YAML no Git, importado pela CLI)?

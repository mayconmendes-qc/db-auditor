# 01 — Risco de migrations com contexto do banco real (`migration-risk`)

## 0. Contexto e problema, objetivo, fora de escopo

**Problema.** Uma migration com lock forte (criar índice sem `CONCURRENTLY`, reescrita de tabela, validação de constraint) trava produção. Linters como squawk e strong_migrations olham só o texto. Eles não sabem o tamanho da tabela nem a taxa de escrita.

**Objetivo.** Antes do merge, classificar cada operação de uma migration PostgreSQL por lock adquirido, se há reescrita ou varredura e o impacto estimado, usando o tamanho e o tráfego já observados pelo db-auditor. Sugerir uma alternativa segura e publicar o resultado no PR.

**Fora de escopo.** Executar ou testar a migration. Gerar a migration corrigida. MongoDB e DDL específico de TimescaleDB (hypertable, compressão) no MVP. Ferramentas de migration não SQL (ORM em código).

## 1. Requisitos funcionais

- **RF-01** Analisar um ou mais arquivos `.sql`, ou um diretório, e identificar cada operação com arquivo e linha.
- **RF-02** Classificar cada operação pelas regras da seção 3, informando o modo de lock da documentação oficial do PostgreSQL, se reescreve ou varre a tabela e as operações bloqueadas (leitura, escrita).
- **RF-03** Considerar a versão major do PostgreSQL do alvo (14 a 18, mesma matriz da CI), porque o comportamento de lock muda entre versões. Origem da versão: snapshot do ambiente ou parâmetro explícito.
- **RF-04** Contexto, em três modos, nesta ordem de preferência:
  - (a) **snapshot**: última coleta completa de um ambiente, lida pela API do auditor, sem DSN no CI;
  - (b) **conexão direta só leitura**, só para uso local do operador, nunca no runner de CI (ADR-0001 D2);
  - (c) **sem contexto**: só a análise estática.
- **RF-05** Com contexto, estimar por tabela afetada: tamanho total, linhas estimadas, taxa de escrita (delta entre duas coletas completas) e duração estimada da reescrita ou varredura.
- **RF-06** Para cada risco, sugerir a alternativa segura em texto (por exemplo: índice concorrente, constraint criada como não validada e validada depois, `lock_timeout` explícito, separar em etapas).
- **RF-07** Detectar operações dentro de bloco transacional que não podem rodar em transação, e operações seguras que só ficam inseguras quando combinadas na mesma transação.
- **RF-08** Permitir supressão por linha com justificativa obrigatória no próprio arquivo de migration, registrada na saída.
- **RF-09** Integração GitHub Action: um passo que executa o mesmo binário em modo CLI e publica **um** comentário no PR, atualizado a cada push (não duplicar), com o resumo e a tabela de riscos.
- **RF-10** Opcionalmente, enviar o resultado ao auditor, vinculado ao ambiente, para histórico ("análises de migration" por ambiente na UI, com PR, commit e nível de risco).

## 2. Requisitos não funcionais

- **RNF-01 Segurança e só leitura.** O modo (b) usa a conexão só leitura existente, com allowlist de host e TLS. O modo (a) usa o token por ambiente com escopo mínimo (ADR-0001 D1). A GitHub Action só oferece o modo (a): o runner de CI nunca recebe DSN nem senha do banco (ADR-0001 D2). A CLI nunca executa o conteúdo da migration.
- **RNF-02 LGPD.** Migrations podem ter literais (seed, atualização de dados). Saída e histórico mostram tipo de operação, objeto, arquivo e linha. Nunca mostram literais nem o texto completo do comando. O texto é normalizado sem literais, como já faz a normalização de consultas do coletor de workload (`NormalizeQuery` em `collectors/postgres/stats_workload.go`). Essa normalização será extraída para `internal/fingerprint`, porque hoje `fingerprint.NormalizeSQL` não remove literais (ADR-0001 D16).
- **RNF-03 Desempenho.** Análise estática de 200 arquivos ou 2 MB em menos de 10 s. Com contexto, uma única leitura de metadados por tabela afetada. Limite de 500 operações por execução, com aviso de truncamento.
- **RNF-04 Multi-tenant.** O histórico no auditor pertence a um ambiente e segue as concessões por ambiente. A credencial de CI não lê outros ambientes.
- **RNF-05 Observabilidade.** Métricas de análises por resultado e erros de parse no `/metrics` existente. Log sem DSN, token nem SQL bruto (sanitização de `config`).
- **RNF-06 Determinismo.** Mesma entrada, mesma versão de regra e mesmo snapshot devem produzir a mesma saída (comparável em teste golden).

## 3. Regras importantes

| ID | Operação | Lock / efeito | Severidade base | Eleva para alta | Eleva para crítica |
|---|---|---|---|---|---|
| MR-01 | Criar índice sem modo concorrente | SHARE, bloqueia escrita durante o build | média | tabela ≥ 1 GB ou ≥ 5 M linhas | ≥ 10 GB ou ≥ 50 M linhas |
| MR-02 | Adicionar coluna com default volátil | ACCESS EXCLUSIVE + reescrita | alta | — | ≥ 10 GB |
| MR-03 | Alterar tipo de coluna que exige reescrita | ACCESS EXCLUSIVE + reescrita | alta | — | ≥ 10 GB |
| MR-04 | Tornar coluna existente obrigatória sem constraint de checagem validada antes | ACCESS EXCLUSIVE + varredura | média | ≥ 1 GB | ≥ 10 GB |
| MR-05 | Adicionar FK validada imediatamente | SHARE ROW EXCLUSIVE nas duas tabelas + varredura | média | ≥ 1 GB em qualquer lado | ≥ 10 GB |
| MR-06 | Adicionar constraint de checagem validada imediatamente | ACCESS EXCLUSIVE + varredura | média | ≥ 1 GB | ≥ 10 GB |
| MR-07 | Adicionar PK ou unicidade sem índice prévio | ACCESS EXCLUSIVE + build de índice | alta | — | ≥ 10 GB |
| MR-08 | Remover ou renomear coluna ou tabela | ACCESS EXCLUSIVE breve; quebra de compatibilidade com a aplicação | média | tabela com escrita observada | — |
| MR-09 | Reescrita total (VACUUM FULL, CLUSTER, refresh de visão materializada não concorrente) | ACCESS EXCLUSIVE | alta | — | ≥ 10 GB |
| MR-10 | Lock forte sem `lock_timeout` na migration | Fila de lock bloqueia o tráfego seguinte | baixa | taxa de escrita ≥ 100 escritas/s | — |
| MR-11 | Operação que não roda em transação, dentro de transação | Falha no deploy | alta | — | — |

- Limiares de tamanho, linhas e taxa: **valor inicial, calibrar**, configuráveis por ambiente.
- Duração estimada = tamanho ÷ vazão de referência. A vazão é configurável (padrão 50 MB/s, **valor inicial, calibrar**) e sempre aparece como estimativa, com confiança.
- Sem contexto: severidade base, confiança baixa e impacto "não estimado" com o motivo (regra AN-03).
- Tabela criada na própria migration: risco de lock desprezível, informativa.
- Gate de CI: bloqueia (código 1) a partir de **alta** por padrão. Configurável.

## 4. Respostas esperadas

- **Entradas.** Caminhos de arquivos ou diretório. Modo de contexto e ambiente. Versão do PostgreSQL (opcional se houver snapshot). Severidade de bloqueio. Formato de saída.
- **Saída Markdown (comentário do PR).** Resumo com contagem por severidade e o maior risco. Tabela com arquivo:linha, operação, objeto, lock, reescrita/varredura, tamanho, duração estimada, severidade, confiança e alternativa segura. Rodapé com execução de referência (id do `audit_run` e data) e versão das regras.
- **Saída JSON.** Mesmos campos, mais a versão do esquema de saída.
- **UI/API (opcional, RF-10).** Lista de análises por ambiente com PR, commit, maior severidade e data. Detalhe com a mesma tabela. Leitura exige viewer. Envio exige credencial de CI do ambiente.
- **Códigos de saída.** Os do README. 3 quando o contexto foi pedido, mas o snapshot é parcial, antigo além do limite (padrão 7 dias, **valor inicial, calibrar**) ou inexistente.
- **Erros.**
  - arquivo ilegível ou erro de parse: operação marcada "não analisada" com arquivo:linha, código 3;
  - ambiente sem coleta completa: código 3 com mensagem;
  - credencial inválida ou sem escopo: código 2;
  - conexão com conta que tem escrita: código 4.

## 5. Critérios de aceite

- **CA-01** Dado uma migration que cria índice sem modo concorrente numa tabela de 12 GB no snapshot, quando a CLI roda com contexto snapshot, então a saída marca MR-01 como crítica, mostra o tamanho, o lock SHARE e a alternativa concorrente, e sai com código 1.
- **CA-02** Dado a mesma migration sem contexto, quando a CLI roda, então MR-01 aparece como média, com confiança baixa e impacto "não estimado", e o código é 0 com o gate padrão.
- **CA-03** Dado uma migration com um literal de e-mail num comando de atualização de dados, quando a saída é gerada, então nem o Markdown, nem o JSON, nem o histórico contêm o literal.
- **CA-04** Dado um PR com comentário anterior do auditor, quando há novo push, então o comentário existente é atualizado e não surge um segundo comentário.
- **CA-05** Dado um snapshot `partial_success` como contexto, quando a CLI roda, então o código é 3 e a saída diz que o contexto é parcial.
- **CA-06** Dado a conexão direta (uso local do operador) com uma conta com privilégio de escrita, quando a CLI tenta o modo (b), então recusa com código 4 sem ler nada.
- **CA-07** Dado uma supressão por linha com justificativa, quando a regra dispara, então a operação aparece como "suprimida", com a justificativa, e não conta para o gate.
- **CA-08** Dado uma migration que torna uma coluna existente obrigatória (MR-04), quando a CLI roda com versão alvo 14, 15, 16, 17 e 18, então a saída de cada versão segue a documentação daquela versão: no PG 18, a alternativa segura sugerida é `NOT NULL ... NOT VALID`; no PG 14–17, é `CHECK (... IS NOT NULL) NOT VALID` seguida de `VALIDATE` (ADR-0001 D12; teste nas versões da matriz da CI).

## 6. Dependências, reaproveitamento, riscos e perguntas em aberto

**Reaproveita.**
- `table_snapshot` (tamanho, linhas estimadas, contadores de escrita) e a regra de comparabilidade de duas coletas completas de `maintenance.growth_trend`, para a taxa de escrita;
- conexão só leitura e sanitização de DSN (`collectors/postgres`, `config`);
- normalização sem literais do coletor de workload, extraída para `internal/fingerprint` (ADR-0001 D16);
- catálogo de regras versionado e esquema de impacto AN-03;
- matriz PostgreSQL 14–18 da CI (#134).

**Novo.**
- modo CLI e subcomando;
- parser de DDL PostgreSQL em Go puro (hoje não existe parser SQL no repositório, e a imagem é `CGO_ENABLED=0`; ADR-0001 D16);
- catálogo de regras MR-*;
- integração GitHub Action;
- credencial de CI com escopo de ambiente;
- histórico de análises de migration.

**Dependências.** Nenhuma spec bloqueia esta. A 04 é opcional (com a timeline, dá para mostrar se a tabela mudou desde o snapshot).

**Riscos.**
- Snapshot defasado subestima o tamanho.
- Parser divergente da versão do servidor.
- Falsa sensação de segurança quando falta contexto. Mitigação: confiança e motivo explícitos.

**Perguntas resolvidas** ([ADR-0001](../adr/0001-decisoes-v1-features.md)).
- P-01: como o CI se autentica no auditor? **Resolvida (D1):** token novo por ambiente, com escopo mínimo. `AUDITOR_REPORT_API_TOKEN` não é reutilizado.
- P-02: permitir o modo (b), DSN de produção no runner de CI, ou só o modo (a)? **Resolvida (D2):** CI só no modo (a). O modo (b) é só para uso local do operador.
- P-03: publicar SARIF? **Resolvida (D12):** fora do v1.

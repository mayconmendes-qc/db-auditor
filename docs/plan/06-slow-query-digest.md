# Plano 06 — Digest semanal de regressões de consultas (`slow-query-digest`)

- **Spec:** [../spec/06-slow-query-digest.md](../spec/06-slow-query-digest.md)
- **Base lida:** `main` em `b1a93cf`
- **Posição na ordem de implementação:** 1ª (ver [README](README.md))
- **Total:** 4 lotes, **13 d** (dias-pessoa, com testes e docs), 1 lote com gate R2, Trinity necessária (1 lote)

## 0. Resumo

O digest é calculado só a partir do snapshot store (`workload_snapshot`), sem nenhuma sessão nova no alvo. O plano cria um pacote de cálculo de janela (`internal/digest`), que também será usado pela spec 02 para o custo por consulta. Depois vêm a persistência do digest com histórico, a entrega pelos canais de alerta globais que já existem e a aba de UI.

### Decisões padrão aplicadas

- **decisão padrão (Q, nº 6):** só as sugestões de índice existentes (`index.candidate`, `performance.workload_scan`). **Sem HypoPG no v1.** O RF-05 fica limitado à primeira frase, e a P-01 da spec fica respondida como "não".
- **decisão padrão (Q, nº 6):** entrega pelos **canais globais atuais** (`AUDITOR_ALERT_WEBHOOK_URL`, `AUDITOR_ALERT_EMAIL_TO` e SMTP em `internal/notify`). Não haverá destinatários por ambiente no v1. A P-02 fica respondida como "canais globais". Por ambiente, o operator configura só janela, limiares e a liga/desliga da entrega.
- **decisão padrão (Q, nº 11):** tenant = ambiente (`audit_environment`). Cada digest pertence a um único ambiente e nunca mistura ambientes (RNF-04).

### Dependências

Nenhuma bloqueante. A 02 (lote 3) depende do lote 1 deste plano.

## 1. Módulos e arquivos afetados

| Área | Caminho | Mudança |
|---|---|---|
| Cálculo de janela | `backend/internal/digest/` (novo: `window.go`, `classify.go`, `markdown.go`, `*_test.go`) | delta por fingerprint, subjanela sem reset, classificação, Markdown |
| Regras de comparabilidade | `backend/internal/repository/regression_policy.go` | reaproveitar `statsResetChanged`, `PairComparable` e `RegressionMaxGap`, sem mudar a semântica (AN-11) |
| Leitura de workload | `backend/internal/repository/query_digest.go` (novo) | leitura paginada de `workload_snapshot` por janela e persistência do digest |
| Agendamento | `backend/internal/scheduler/scheduler.go`, `backend/internal/audit/runner.go` | gatilho após coleta completa que fecha a janela (mesmo ponto de `EvaluateRegressionAlerts`) |
| Entrega | `backend/internal/notify/notify.go` | novo `Event.Kind = "query_digest"` com resumo e link. O erro de transporte passa a ser registrado pelo chamador |
| Métricas | `backend/internal/observability/metrics.go` | `auditor_query_digests_total{result}`, `auditor_query_digest_deliveries_failed_total` |
| API | `backend/internal/api/query_digest.go` (novo), registro em `backend/internal/api/health.go` | rotas da seção 3 |
| Contrato | `backend/api/openapi.json`, `frontend/src/types/openapi.ts` (`bun run gen:openapi`) | novas rotas (o `openapi_test.go` exige) |
| Frontend | `frontend/src/pages/QueryDigestPage.tsx` (novo) ou aba em `MonitoringPage.tsx` (Trinity decide), `frontend/src/services/api.ts`, `frontend/src/components/layout/Shell.tsx`, `frontend/src/app.tsx` | aba **Digest** |
| Docs | `docs/rules.md` (nova seção "Digest de consultas"), `docs/ops.md` (configuração e calibração BA-15), `docs/guia-do-usuario.md` + `bun run sync:guide` | — |

## 2. Migrations (provisórias)

A numeração é **provisória**: os planos rodam na ordem do README, e cada número só é confirmado no merge. Hoje a última é `25_role_attributes_regression_workload.sql`. O número `02` está vago e **não** deve ser reaproveitado.

| Nº provisório | Arquivo | Lote | Conteúdo |
|---|---|---|---|
| 26 | `backend/migrations/26_query_digest.sql` | 2 | `query_digest_config` (ambiente, janela em dias, limiares em jsonb, entrega ligada), `query_digest` (ambiente, janelas, execuções usadas, N do top, estado `ok`/`no_comparable_window`/`not_applicable`, motivo, versão do cálculo), `query_digest_item` (classificação, fingerprint, `query_id`, números das duas janelas, variação, `finding_id`/`regression_alert_id` linkados, **sem texto SQL**), `query_digest_delivery` (canal, estado, erro sanitizado, tentativa). Só `CREATE ... IF NOT EXISTS`, idempotente. |

Checklist comum de migration: acrescentar o arquivo à lista explícita do passo "Prepare disposable snapshot store" do job `Backend (Go)` em `.github/workflows/ci.yml` (o job da matriz usa o glob), a `backend/migrations/README.md` e à faixa citada em `docs/ops.md`.

## 3. Endpoints e telas

| Método e rota | Papel | Observação |
|---|---|---|
| `GET /api/v1/environments/{id}/query-digests` | viewer | lista paginada (`parseListPage`) |
| `GET /api/v1/environments/{id}/query-digests/{digest}` | viewer | detalhe com seções do RF-04, ressalvas e entregas |
| `GET /api/v1/environments/{id}/query-digests/{digest}/markdown` | viewer | corpo Markdown (o mesmo do e-mail) |
| `POST /api/v1/environments/{id}/query-digests` | operator | geração sob demanda. A regra padrão de `authorizeRequest` (não-GET = operator) já cobre |
| `GET /api/v1/environments/{id}/query-digest-config` | viewer | — |
| `PUT /api/v1/environments/{id}/query-digest-config` | operator | janela, limiares, entrega ligada. Viewer recebe 403 (CA-07) |

**Tela.** A spec diz "Em Performance, nova aba **Digest**". Hoje não há uma seção "Performance" na navegação (`Shell.tsx` tem Início, Operação, Análise e Sistema). "Performance" existe só como categoria em `FindingsBrowser` e como aba em `InventoryDetailTabs`. **Trinity é necessária** para decidir onde o digest fica (proposta: item "Digest de consultas" no grupo Análise ou aba em Acompanhamento) e para desenhar lista, detalhe e ressalvas de cobertura.

## 4. Lotes (um PR por lote)

### Lote 1 — Motor de delta por janela e classificação

**Issue:** `feat(digest): delta de workload por janela e classificação regressão/nova/sumiu/melhora (spec 06)`

**Descrição.** Criar `backend/internal/digest` com funções puras sobre pontos de `workload_snapshot`. Para cada janela, usar a primeira e a última coleta **completa** (`success`, análise concluída). A coleta `partial_success` nunca serve de extremo. Calcular Δchamadas, Δtempo total, tempo médio da janela (Δtempo ÷ Δchamadas), Δlinhas e Δblocos. Invalidar a janela quando `stats_reset` mudar, inclusive NULL → valor, reaproveitando `statsResetChanged`. Nesse caso, procurar a maior subjanela contígua sem reset. Lacuna maior que `AUDITOR_REGRESSION_MAX_GAP` também invalida. Classificar pelos limiares da seção 3 da spec, parametrizáveis por ambiente. A leitura do store fica em `repository/query_digest.go`, paginada e com `query_id` quando estável.

**Critérios de aceite.**
- [ ] CA-01: 1000 chamadas a 20 ms → 1200 a 35 ms resulta em "regressão média, +75%".
- [ ] CA-02: reset na janela atual leva à maior subjanela sem reset, indicada no resultado. Sem subjanela, o estado é `no_comparable_window`.
- [ ] CA-03: coleta `partial_success` nunca é extremo de janela.
- [ ] RNF-03: 500 fingerprints × 42 coletas por janela em menos de 3 s (teste de escala com `AUDITOR_TEST_SCALE=1`).
- [ ] Mudança de fingerprint ("sumiu + nova") documentada como limitação em `docs/rules.md`.
- [ ] Nenhuma alteração de comportamento em `regression_policy.go` (os testes AN-11 continuam verdes).

**Arquivos.** `internal/digest/*`, `internal/repository/query_digest.go`, testes de integração em `internal/repository/`.
**Estimativa:** 3 d. **Gate:** CI verde. **Depende de:** —

### Lote 2 — Persistência, agendamento e API do digest

**Issue:** `feat(digest): entidade digest com histórico, geração agendada e sob demanda, API (spec 06)`

**Descrição.** Migration `26_query_digest.sql` (provisória). Gerar o digest depois da coleta completa que fecha a janela, no mesmo ponto do `audit/runner.go` em que `EvaluateRegressionAlerts` é chamado, por uma interface opcional (`DigestEvaluator`) para não acoplar o runner. Também sob demanda. Top 10 regressões, top 10 novas, top 5 sumiram e top 5 melhoras (RF-04). Anexar `index.candidate` e `performance.workload_scan` das mesmas tabelas por `referenced_objects` (RF-05, **decisão padrão** sem HypoPG). Linkar o alerta `workload` aberto do mesmo fingerprint sem criar alerta novo (RF-08). MongoDB e ausência de `pg_stat_statements` geram estado `not_applicable` com motivo (capacidade existente). Rotas da seção 3 e `openapi.json`.

**Critérios de aceite.**
- [ ] CA-04: com alerta `workload` aberto, o item linka o alerta e `regression_alert` não ganha linha nova.
- [ ] CA-06: ambiente MongoDB responde "não aplicável a este mecanismo".
- [ ] CA-07: viewer recebe 403 em `PUT .../query-digest-config` e em `POST .../query-digests`.
- [ ] RNF-04: teste com dois ambientes. O digest de A nunca contém fingerprint de B, e viewer sem concessão a B recebe 403 em B.
- [ ] RNF-06: o digest grava e expõe N (`AUDITOR_WORKLOAD_LIMIT`) e a ressalva de top N.
- [ ] Migration idempotente aplicada nos jobs PG 14–18 e listada em `ci.yml`.

**Arquivos.** `migrations/26_query_digest.sql`, `internal/repository/query_digest.go`, `internal/api/query_digest.go`, `internal/api/health.go`, `internal/audit/runner.go`, `internal/scheduler/scheduler.go`, `api/openapi.json`.
**Estimativa:** 4 d. **Gate:** CI verde (as rotas seguem o RBAC padrão de `authorizeRequest`, sem mudança nele). **Depende de:** lote 1.

### Lote 3 — Markdown, entrega e registro de falhas

**Issue:** `feat(digest): Markdown, e-mail e webhook pelos canais globais, com registro de falha e nova tentativa (spec 06)`

**Descrição.** Gerar o Markdown (cabeçalho com ambiente, janelas, coletas usadas e N; seções; rodapé com ressalvas). Corpo ≤ 100 KB, truncando com aviso. Enviar por `notify.Config` (**decisão padrão**: canais globais). O webhook leva só o evento resumido: contagens por classificação e link (spec §4). Hoje `internal/notify` devolve o erro e o chamador o ignora. Para o digest, registrar o resultado em `query_digest_delivery` com o erro sanitizado (`config.SanitizeError`) e tentar de novo no próximo ciclo. Usar `ClaimAlert`/`alert_delivery` com `dedup_key` = id do digest para não reenviar o que já foi entregue. Métricas `auditor_query_digests_total{result}` e `auditor_query_digest_deliveries_failed_total`.

**Critérios de aceite.**
- [ ] CA-05: teste com fingerprints que vieram de consultas com literais (e-mail, CPF, números). Nem o Markdown, nem o corpo do e-mail, nem o payload do webhook contêm os literais (nada de texto original de `pg_stat_statements`).
- [ ] Payload do webhook sem lista completa, sem DSN, senha ou SQL (contrato de `notify.Event`).
- [ ] Falha de SMTP ou webhook fica registrada no histórico do digest e é tentada de novo no ciclo seguinte, sem duplicar a entrega bem-sucedida.
- [ ] RNF-05: métricas expostas em `/metrics` (porta interna 9090).
- [ ] RNF-03: corpo ≤ 100 KB.

**Arquivos.** `internal/digest/markdown.go`, `internal/notify/notify.go`, `internal/repository/query_digest.go`, `internal/observability/metrics.go`, `docs/ops.md`.
**Estimativa:** 3 d. **Gate:** **R2** (saída de dados para fora do sistema por e-mail e webhook; risco de vazar literal de consulta, dado pessoal). **Depende de:** lote 2.

### Lote 4 — Aba Digest na UI

**Issue:** `feat(ui): aba Digest com histórico, detalhe e ressalvas de cobertura (spec 06)`

**Descrição.** Tela definida com a Trinity: lista de digests por ambiente (estado, janelas, contagens), detalhe com as quatro seções do RF-04, números das duas janelas, variação, link para o achado ou alerta, ressalvas de cobertura (N do top, subjanela usada, motivo de "sem janela comparável") e histórico de entregas. O botão "Gerar agora" e a configuração aparecem só para operator (`api.hasRole("operator")`), e o backend continua sendo a fonte do 403.

**Critérios de aceite.**
- [ ] Layout aprovado pela Trinity (estados vazio, sem janela comparável, não aplicável e falha de entrega).
- [ ] CA-06 visível: MongoDB mostra "não aplicável a este mecanismo".
- [ ] CA-07 na UI: viewer não vê os controles de configuração. Teste de componente cobre o caso.
- [ ] Biome, typecheck e testes do job `Frontend (Bun)` verdes. `openapi.ts` regenerado sem diff.
- [ ] `docs/guia-do-usuario.md` com a seção "Ajuda por tela" do digest (lida por `guideTopics.ts`) + `bun run sync:guide`.

**Arquivos.** `frontend/src/pages/QueryDigestPage.tsx` (ou aba), `frontend/src/services/api.ts`, `frontend/src/app.tsx`, `frontend/src/components/layout/Shell.tsx`, `docs/guia-do-usuario.md`, `frontend/public/guia-do-usuario.md`.
**Estimativa:** 3 d. **Gate:** CI verde (+ aprovação de layout da Trinity). **Depende de:** lote 2 (o lote 3 é opcional para a tela).

## 5. Estratégia de testes

| Teste | Tipo | Critério ligado |
|---|---|---|
| `TestDigestRegressionClassification` (20→35 ms, +75%, média) | unitário, `internal/digest` | CA-01 |
| `TestDigestStatsResetSubwindow` (reset no meio; NULL→valor; sem subjanela) | unitário + integração no store | CA-02, RF-03 |
| `TestDigestIgnoresPartialRunAsWindowEdge` | integração (`AUDITOR_TEST_DATABASE_URL`) | CA-03 |
| `TestDigestLinksOpenWorkloadAlert` (conta linhas de `regression_alert` antes e depois) | integração | CA-04, RF-08 |
| `TestDigestMarkdownHasNoLiterals` (golden + varredura de padrões de literal) | unitário + golden em `internal/digest/testdata` | CA-05, RNF-02 |
| `TestDigestMongoNotApplicable` | API (`httptest`) | CA-06 |
| `TestDigestConfigRequiresOperator` (viewer, auditor → 403; operator → 200) | API com `authMiddleware` | CA-07 |
| `TestDigestNeverMixesEnvironments` | integração | RNF-04 |
| `TestDigestScaleBudget` (500 × 42 < 3 s) | escala (`AUDITOR_TEST_SCALE=1`) | RNF-03 |
| `TestDigestDeliveryFailureRecordedAndRetried` (servidor webhook falso que falha e depois aceita) | unitário/integração | RNF-05, §4 Erros |
| Testes de componente da aba | `bun run test` | CA-06, CA-07 (UI) |

Calibração: os limiares começam como **valor inicial, calibrar**. O procedimento BA-15 (#104) é documentado em `docs/ops.md`, com taxa de falso positivo registrada.

## 6. Gates

| Lote | Gate | Motivo |
|---|---|---|
| 1 | CI verde | cálculo puro sobre o store |
| 2 | CI verde | rotas por ambiente no RBAC existente, sem mudança em `authorizeRequest` |
| 3 | **R2** | e-mail e webhook levam conteúdo derivado de consultas (risco de dado pessoal em literal) |
| 4 | CI verde + Trinity | UI |

## 7. Riscos

- O top N (`AUDITOR_WORKLOAD_LIMIT`, padrão 500) esconde consultas medianas que pioraram. Mitigação: ressalva explícita com N (RNF-06).
- Refatoração de SQL muda o fingerprint e vira "sumiu + nova". Mitigação: texto na ressalva e uso de `query_id` quando estável.
- Cadência baixa de coleta gera janelas pobres ou "sem janela comparável" frequente. Mitigação: o digest diz o motivo e o guia recomenda o perfil `daily`.
- Os canais globais (**decisão padrão**) mandam o digest de todos os ambientes para o mesmo destino. Risco de exposição entre times quando ambientes de clientes diferentes compartilham a instalação. Registrar em `docs/ops.md`.
- Mudar `internal/notify` para registrar falha não pode alterar o comportamento atual dos alertas (falha não falha o run).

## 8. Pontos para Q

- A spec pede destinatários por ambiente (RF-07, RNF-04 e §4 "configurar ... destinatários (operator)"), e o CA-07 fala em "mudar os destinatários". Pela **decisão padrão** (canais globais), o CA-07 foi reinterpretado como "mudar a configuração do digest do ambiente". Confirmar.

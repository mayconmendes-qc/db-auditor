# Plano 07 — Revisão periódica de acessos (`access-review`)

- **Spec:** [../spec/07-access-review.md](../spec/07-access-review.md)
- **Base lida:** `main` em `b1a93cf`
- **Posição na ordem de implementação:** 5ª
- **Total:** 5 lotes no v1, **28 d**, mais 1 lote condicional (2 d). 4 lotes com gate R2 no v1 (+1 condicional). Trinity necessária (1 lote)

## 0. Resumo

O plano estende a coleta de privilégios com a **origem** de cada privilégio (direto, via role X com a cadeia, via PUBLIC, via role predefinida) e com **privilégios por coluna**. Sobre a coleta de referência, cria campanhas de revisão com responsáveis, decisões imutáveis, diff entre campanhas, exportação CSV e PDF de evidência. "Revogar" vira ação assistida (`finding_action`), e o auditor nunca revoga nada.

### Decisões padrão aplicadas

- **decisão padrão (Q, nº 7):** o **responsável é texto livre (nome e e-mail)**, mantido no auditor por role ou conta, só para a planilha e o agrupamento. Ele **não** é uma conta do auditor. Responde à P-01.
- **decisão padrão (Q, nº 7):** **saída em CSV no v1. XLSX fica para depois.** Responde à P-02.
- **decisão padrão (Q, nº 7):** **o último login vem dos logs quando a spec 10 existir.** No v1, a fonte é a amostragem de sessões atual (`security.account_inactive_review`, `10_account_activity.sql`). O lote 6 é condicional. Responde à P-03.
- **decisão padrão (Q, nº 11):** tenant = ambiente. A campanha, a matriz, o mapeamento de responsáveis e as decisões pertencem ao ambiente.

### Dependências

Nenhuma bloqueante. Uso opcional da 04 (timeline: quando o privilégio apareceu) e da 10 (último login). É **pré-requisito** da 05 (lote 1: matriz de privilégio efetivo com origem e privilégio por coluna).

## 1. Módulos e arquivos afetados

| Área | Caminho | Mudança |
|---|---|---|
| Coleta de origem | `backend/sql/collectors/postgres/effective_grants.sql`, novos `grant_origins.sql` e `column_grants.sql`, `backend/internal/collectors/postgres/account_roles.go` (`membershipModes`, limiar `>= 160000`), novo `collectors/postgres/grant_origin.go`, `internal/audit/live_registry_postgres.go` | cadeia de membership por privilégio, PUBLIC, roles predefinidas; ACL de coluna (`pg_attribute.attacl`); sequences, funções (EXECUTE) e schemas (CREATE/USAGE, `schema_snapshot.nspacl`) |
| Store | `backend/internal/repository/account_roles.go`, novos `access_review.go` e `grant_origin_snapshots.go` | — |
| Regras | `backend/internal/analyzer/access_review.go` (novo), `analyzer/security.go` (reuso de `security.powerful_role`, `security.account_inactive_review`, `security.excessive_privilege`, `security.public_schema_create`), `backend/internal/actions/catalog.go` | — |
| Ações assistidas | `backend/internal/repository/actions.go`, `action_measurements.go` | decisão "revogar" cria `finding_action` com plano e medição na coleta seguinte |
| API e RBAC | `backend/internal/api/access_review.go` (novo), `backend/internal/api/auth.go` (`authorizeRequest`: decisões exigem auditor) | — |
| Exportação | padrão de streaming de `backend/internal/api/action_measurements.go` (`# export_complete`) | CSV |
| PDF | `backend/internal/report/` (novo tipo `access_review`), `backend/internal/reportworker/worker.go`, `report/redaction.go` | — |
| Frontend | `frontend/src/pages/AccessReviewPage.tsx` (novo), `pages/parts/AccessReviewMatrix.tsx` (virtualizada), `services/api.ts`, `components/layout/Shell.tsx`, `app.tsx` | — |
| Docs | `docs/rules.md` (origem por versão, ligada à seção "Roles poderosas e versão do PostgreSQL (#126, #134)"), `docs/ops.md`, `docs/guia-do-usuario.md` | — |

## 2. Migrations (provisórias)

| Nº provisório | Arquivo | Lote | Conteúdo |
|---|---|---|---|
| 33 | `backend/migrations/33_grant_origin_column_privilege.sql` | 1 | `grant_origin_snapshot` (execução, ambiente, banco, conta com LOGIN, tipo de objeto `table`/`view`/`sequence`/`function`/`schema`, objeto qualificado, privilégio, `origin_kind` `direct`/`role`/`public`/`predefined`, `via_chain text[]`, `inherit`/`set` quando PG 16+); `column_grant_snapshot` (execução, objeto, coluna, conta, privilégio, origem). Snapshots antigos ficam "não coletado" |
| 34 | `backend/migrations/34_access_review.sql` | 2 | `access_review_owner` (ambiente, role ou conta, nome e e-mail em texto livre, **decisão padrão**), `access_review_campaign` (ambiente, escopo, `reference_run_id` completa, prazo, estado `open`/`in_review`/`closed`/`overdue`, criador), `access_review_item` (matriz congelada da coleta de referência + destaques), `access_review_decision` (append-only: item, decisão `approve`/`revoke`/`keep`, prazo de revogação, justificativa, `decided_by`, `decided_at`, `supersedes_id`, `finding_action_id`); amplia o CHECK de `report_job.report_type` com `access_review` (padrão de `19_architecture_report.sql`) |
| 41 (condicional) | `backend/migrations/41_account_last_login.sql` | 6 | `account_last_login_observation` (ambiente, role, último horário, fonte `log`, lote de origem). Só com a fase 2 da spec 10 |

Os números supõem 26–32 já mergeados. O 41 fica depois do 40 do plano 10 (condicional). Lembrar `ci.yml`, `migrations/README.md` e `docs/ops.md`.

## 3. Endpoints e telas

| Método e rota | Papel | Observação |
|---|---|---|
| `GET`/`PUT /api/v1/environments/{id}/access-review-owners` | viewer / operator | mapeamento role/conta → nome e e-mail livres |
| `POST /api/v1/environments/{id}/access-reviews` | operator | 409 se a coleta de referência for parcial |
| `GET /api/v1/environments/{id}/access-reviews`, `GET .../{campaign}` | viewer | lista e progresso (% decidido, pendências) |
| `GET /api/v1/environments/{id}/access-reviews/{campaign}/matrix?account=&object=&privilege=&origin=&highlight=&pending=&owner=&limit=&offset=` | viewer | filtrada e paginada no servidor |
| `POST /api/v1/environments/{id}/access-reviews/{campaign}/decisions` | auditor | por linha ou em lote. 400 para "manter" sem justificativa. 409 em campanha concluída |
| `POST /api/v1/environments/{id}/access-reviews/{campaign}/close` | operator | — |
| `GET /api/v1/environments/{id}/access-reviews/{campaign}/diff` | viewer | contra a campanha anterior |
| `GET /api/v1/environments/{id}/access-reviews/{campaign}/export/download?format=csv` | auditor | o sufixo `/download` reaproveita a regra de download de `authorizeRequest` |
| `POST /api/v1/environments/{id}/reports` com `report_type=access_review` | auditor | rota existente de PDF |

**Tela (Trinity necessária).** A spec diz "Em Segurança, nova área". Hoje "Segurança" é só uma categoria de `FindingsBrowser` e uma aba de `InventoryDetailTabs`, sem item de navegação, e "Contas" é a gestão de contas do próprio auditor (fora de escopo). A Trinity define o lugar (proposta: item "Revisão de acessos" no grupo Análise), a matriz grande com filtros e lote, o diff e a visão "pendências por responsável".

## 4. Lotes

### Lote 1 — Origem por privilégio e privilégio por coluna na coleta

**Issue:** `feat(collector): origem do privilégio efetivo (direto, via role, PUBLIC, predefinida) e privilégios por coluna, PG 14–18 (spec 07 RF-02, RF-03)`

**Descrição.** Migration `33_grant_origin_column_privilege.sql` (provisória). Novas consultas de catálogo pela conexão só leitura, no orçamento dos coletores de catálogo: para cada conta com LOGIN, o objeto e o privilégio, com a cadeia de membership que concede, usando as regras por versão já documentadas (PG 14/15: `pg_has_role(...,'MEMBER')`; PG 16+: `SET`/`USAGE`, `INHERIT FALSE`, `ADMIN OPTION`), PUBLIC e roles predefinidas (`pg_read_all_data` etc.). A ACL de coluna vem de `pg_attribute.attacl`. Objetos: tabela, visão, sequence, função com EXECUTE e schema com CREATE/USAGE. Respeitar allowlist e denylist de schemas. `grant_snapshot` continua igual (compatibilidade com as regras atuais).

**Critérios de aceite.**
- [ ] CA-01: `app_report` membro de `g_leitura` mostra SELECT em `vendas.pedido` com origem "via g_leitura".
- [ ] CA-08: origem com e sem herança em PG 14 e 16 segue `docs/rules.md` (#126, #134). O teste entra no passo "Role privilege detection E2E (must run, not skip)" da matriz em `ci.yml`, com conferência da versão do servidor.
- [ ] Privilégio por coluna aparece como linha própria (RF-03).
- [ ] Um banco sem permissão de leitura de catálogo gera cobertura parcial, não falha da coleta.
- [ ] Orçamento: tempo de coleta medido no teste de escala, sem estourar o `statement_timeout` de catálogo.

**Arquivos.** `sql/collectors/postgres/grant_origins.sql`, `sql/collectors/postgres/column_grants.sql`, `internal/collectors/postgres/grant_origin.go`, `internal/collectors/postgres/account_roles.go`, `internal/audit/live_registry_postgres.go`, `internal/repository/grant_origin_snapshots.go`, `migrations/33_*.sql`, `.github/workflows/ci.yml`, `docs/rules.md`.
**Estimativa:** 6 d. **Gate:** **R2** (resolução de privilégio é sinal de segurança e base da 05; consultas novas no alvo). **Depende de:** —

### Lote 2 — Campanha, matriz congelada e responsáveis

**Issue:** `feat(access-review): campanhas por ambiente, matriz efetiva congelada da coleta de referência e mapeamento de responsáveis em texto livre`

**Descrição.** Migration `34_access_review.sql` (provisória). A criação da campanha (operator) exige coleta de referência **completa** (senão 409) e congela a matriz em `access_review_item` com os destaques do RF-04: superusuário e poderes de `security.powerful_role`, conta sem sessão observada ou com validade vencida (`security.account_inactive_review`), escrita em schema de outra aplicação e privilégio novo desde a campanha anterior. Responsável em texto livre (**decisão padrão**). Linha sem responsável vai para o operator (RF-05). Matriz paginada e filtrada no servidor. Estados e vencimento por prazo.

**Critérios de aceite.**
- [ ] Coleta de referência parcial: 409 ("campanha exige coleta completa").
- [ ] RNF-03: 500 contas × 5000 objetos, menos de 2 s por página (teste de escala).
- [ ] RNF-04: viewer lê. Operator cria e mapeia responsáveis. Auditor não cria campanha (403).
- [ ] E-mail do responsável validado em formato e tamanho. Ele é tratado como dado pessoal: não entra em logs nem métricas.
- [ ] RNF-06: a matriz congelada não muda quando chegam coletas novas.

**Arquivos.** `migrations/34_access_review.sql`, `internal/repository/access_review.go`, `internal/api/access_review.go`, `internal/api/health.go`, `api/openapi.json`.
**Estimativa:** 6 d. **Gate:** **R2** (dado pessoal: nomes de role e e-mail do responsável). **Depende de:** lote 1.

### Lote 3 — Decisões imutáveis, revogação como ação assistida e regras

**Issue:** `feat(access-review): decisões aprovar/revogar/manter imutáveis, revogação via ação assistida e regra de revogação não aplicada (RF-06, RF-07)`

**Descrição.** `POST .../decisions` (auditor; por linha ou em lote). "Manter" exige justificativa. A decisão é append-only, e uma correção é uma nova decisão com `supersedes_id`. Toda decisão vai para `auditor_operation_log`. "Revogar" com prazo cria uma `finding_action` (plano e execução externa), com medição na coleta seguinte. Regras em `analyzer/access_review.go`: revogação não aplicada após o prazo (alta, vinculada à ação), superusuário/BYPASSRLS sem "aprovar" (alta), conta sem sessão ≥ 90 dias com escrita (média), privilégio via PUBLIC fora de `public` (média), privilégio novo sem decisão (média), campanha vencida com ≥ 10% pendente (média). Todos os limiares são **valor inicial, calibrar**. `authorizeRequest` passa a exigir auditor para `/access-reviews/{campaign}/decisions`. Pela **decisão padrão** nº 7, qualquer auditor com acesso ao ambiente decide, e o responsável é só um rótulo. Métricas de campanhas por estado e linhas pendentes (RNF-05).

**Critérios de aceite.**
- [ ] CA-03: "revogar" com prazo vencido e privilégio presente na coleta completa seguinte gera achado alto de "revogação não aplicada", vinculado à ação assistida.
- [ ] CA-04: "manter" sem justificativa recebe 400 e nada é gravado (contagem de linhas igual antes e depois).
- [ ] CA-05: viewer recebe 403 ao decidir.
- [ ] Decisão em campanha concluída recebe 409.
- [ ] Toda decisão aparece em `auditor_operation_log`.
- [ ] As regras novas têm os cinco campos e consulta segura no catálogo.

**Arquivos.** `internal/api/access_review.go`, `internal/api/auth.go`, `internal/repository/access_review.go`, `internal/repository/actions.go`, `internal/analyzer/access_review.go`, `internal/actions/catalog.go`, `internal/observability/metrics.go`, `docs/rules.md`.
**Estimativa:** 5 d. **Gate:** **R2** (mudança em `authorizeRequest`). **Depende de:** lote 2.

### Lote 4 — Diff entre campanhas, exportação CSV e PDF de evidência

**Issue:** `feat(access-review): diff com a campanha anterior, export CSV streaming e PDF de evidência com redação (RF-08, RF-09)`

**Descrição.** Diff: privilégios novos e removidos, decisões que mudaram, contas novas e removidas. CSV com as colunas do §4 (conta, objeto qualificado, tipo, privilégio, origem, destaque, responsável, decisão, justificativa, decidido por, decidido em), em streaming e terminando em `# export_complete` com a contagem de linhas. Download exige auditor. **XLSX fica fora do v1 (decisão padrão).** PDF `access_review`: resumo, responsáveis, % decidido e pendências, respeitando a redação. No modo "identificadores", os nomes de conta somem e as contagens ficam.

**Critérios de aceite.**
- [ ] CA-02: `app_etl` ganha UPDATE em `vendas.pedido` entre campanhas, e o diff lista o privilégio como novo e destacado.
- [ ] CA-06: o CSV termina com o marcador, e o número de linhas confere com o total da matriz.
- [ ] CA-07: PDF com redação "identificadores" sem nomes de conta e com as contagens (golden).
- [ ] Viewer recebe 403 no download (regra `/download`).
- [ ] Teste de memória: o export de 500 × 5000 linhas é feito em streaming, sem carregar tudo.

**Arquivos.** `internal/api/access_review.go`, `internal/repository/access_review.go`, `internal/report/*`, `internal/reportworker/worker.go`, `internal/report/testdata/*`.
**Estimativa:** 5 d. **Gate:** **R2** (exportação de dados pessoais e correção da redação). **Depende de:** lote 3.

### Lote 5 — UI Revisão de acessos

**Issue:** `feat(ui): área Revisão de acessos com campanhas, matriz filtrável, decisão em lote, diff e pendências por responsável`

**Descrição.** Tela desenhada com a Trinity. Lista de campanhas. Matriz virtualizada (`docs/ui-virtualization.md`) com filtros de conta, objeto, privilégio, origem, destaque e pendente. Decisão por linha e em lote, com justificativa obrigatória em "manter". Diff com a anterior. Visão "pendências por responsável" filtrada pelo e-mail ou nome livre (**decisão padrão**). Botões de exportar e de PDF.

**Critérios de aceite.**
- [ ] CA-01 visível: origem "via g_leitura" na linha.
- [ ] CA-04 e CA-05 na UI: botão "manter" desabilitado sem justificativa. Viewer não vê os controles de decisão (o backend continua sendo a fonte do 403).
- [ ] Matriz de 500 × 5000 navegável sem travar (medição de heap como em `scripts/measure-table-heap.mjs`).
- [ ] Layout aprovado pela Trinity.

**Arquivos.** `frontend/src/pages/AccessReviewPage.tsx`, `frontend/src/pages/parts/AccessReviewMatrix.tsx`, `frontend/src/services/api.ts`, `frontend/src/components/layout/Shell.tsx`, `frontend/src/app.tsx`, `docs/guia-do-usuario.md`.
**Estimativa:** 6 d. **Gate:** CI verde + Trinity. **Depende de:** lotes 3 e 4.

### Lote 6 (condicional) — Último login vindo dos logs

**Issue:** `feat(access-review): último login por role a partir do insumo da spec 10 (fase 2)`

**Descrição.** Só entra quando a spec 10 enviar o insumo de último login ao auditor (lote 5 do plano 10, condicional pela **decisão padrão** nº 10). Migration `41_account_last_login.sql` (provisória). A matriz e a regra de conta sem uso passam a mostrar "último login (log)" ao lado de "sem sessão observada (amostra)", com a fonte explícita.

**Critérios de aceite.**
- [ ] Só role e horário são guardados (CA-07 da spec 10).
- [ ] Sem o insumo, o comportamento do v1 continua idêntico.

**Estimativa:** 2 d. **Gate:** **R2** (dado pessoal de autenticação). **Depende de:** plano 10, lote 5.

## 5. Estratégia de testes

| Teste | Tipo | Critério ligado |
|---|---|---|
| `TestGrantOriginViaGroupRole` (`app_report` → `g_leitura`) | integração no alvo descartável | CA-01, RF-02 |
| `TestAccessReviewDiffNewPrivilege` (`app_etl` UPDATE) | integração | CA-02, RF-08 |
| `TestRevocationNotAppliedFinding` | integração (campanha + coleta seguinte) | CA-03, RF-07 |
| `TestKeepWithoutJustificationRejected` | API | CA-04 |
| `TestViewerCannotDecide` | API com `authMiddleware` | CA-05 |
| `TestAccessReviewCSVMarkerAndCount` | API | CA-06 |
| `TestAccessReviewPDFIdentifierRedaction` | golden do relatório | CA-07, RNF-02 |
| `TestGrantOriginByVersion` (PG 14/15 × 16+, INHERIT/SET/ADMIN) | E2E na matriz PG 14–18 (passo "must run") | CA-08 |
| `TestAccessReviewMatrixScaleBudget` | escala | RNF-03 |
| `TestCampaignRequiresCompleteRun` (409) | API | §4 Erros |
| `TestClosedCampaignImmutable` | API | RNF-06 |

## 6. Gates

| Lote | Gate | Motivo |
|---|---|---|
| 1 | **R2** | resolução de privilégio (segurança) e novas consultas no alvo |
| 2 | **R2** | dados pessoais (role, e-mail do responsável) |
| 3 | **R2** | mudança em `authorizeRequest` |
| 4 | **R2** | exportação e redação de dados pessoais |
| 5 | CI verde + Trinity | UI |
| 6 (cond.) | **R2** | último login (dado pessoal) |

## 7. Riscos

- Uma matriz grande demais para revisão humana. Mitigação: lote, filtros por destaque e agrupamento por responsável.
- "Sem sessão observada" é amostral e não embasa revogação sozinho. A ressalva aparece na matriz e no CSV.
- Nomes de role e e-mails livres são dados pessoais. Mitigação: redação no PDF, download só para auditor e nada em log ou métrica.
- O responsável livre (**decisão padrão**) não autentica. Qualquer auditor do ambiente pode decidir em nome dele, então a trilha registra quem decidiu de fato.
- Consultas de origem por cadeia de membership podem ser caras em clusters com muitas roles. Mitigação: orçamento e cobertura parcial.

## 8. Pontos para Q

- A **decisão padrão** nº 7 (responsável em texto livre) conflita com a spec em vários pontos: RNF-04 ("responsável deve ter acesso ao ambiente, papel auditor ou acima"), §4 API ("Decidir (auditor responsável)"), o erro "responsável sem acesso ao ambiente: 400" e a visão "minhas pendências". O plano faz assim: qualquer auditor com acesso decide, o erro 400 sai, e "minhas pendências" vira filtro por responsável. Confirmar.
- RF-09 e RNF-03 (XLSX, 1 M linhas por aba) ficam para depois, pela **decisão padrão**.
- A **decisão padrão** nº 7 (último login dos logs) depende da 10 enviar dados ao auditor, e a **decisão padrão** nº 10 diz "processado só localmente no início". Sem a fase 2 da 10, não há caminho para o dado chegar. Q precisa decidir se o v1 inclui um envio mínimo (só role e horário) ou se o lote 6 espera.

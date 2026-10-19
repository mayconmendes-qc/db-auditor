# Plano 07 — Revisão periódica de acessos (`access-review`)

- **Spec:** [../spec/07-access-review.md](../spec/07-access-review.md)
- **Base lida:** `main` em `b1a93cf`
- **Posição na ordem de implementação:** 5ª
- **Decisões:** [ADR-0001](../adr/0001-decisoes-v1-features.md) (Aceita, 2026-10-18)
- **Total:** 6 lotes, **32 d**, 5 lotes com gate R2, Trinity necessária (1 lote)

## 0. Resumo

O plano estende a coleta de privilégios com a **origem** de cada privilégio (direto, via role X com a cadeia, via PUBLIC, via role predefinida) e com **privilégios por coluna**. Sobre a coleta de referência, cria campanhas de revisão com responsáveis (contas do auditor), decisões imutáveis, diff entre campanhas, exportação CSV e PDF de evidência. "Revogar" vira ação assistida (`finding_action`), e o auditor nunca revoga nada.

### Decisões aplicadas (ADR-0001)

- **ADR-0001 D7:** o **RNF-04 da spec prevalece**. O **responsável é uma conta do auditor** com papel auditor ou acima no ambiente, com **contato livre opcional** (nome e/ou e-mail) só para a planilha e o PDF.
  - Mapear um responsável sem esse acesso gera 400.
  - Decide a linha o responsável dela. Nas linhas sem responsável, decide o operator.
  - Outro auditor recebe 403.
  - A UI tem "minhas pendências". Responde à P-01.
- **ADR-0001 D7:** **saída em CSV no v1. XLSX fica para depois** (RF-09 e RNF-03 parciais). Responde à P-02.
- **ADR-0001 D7, D10:** o **último login vem do agregado de logs da spec 10** (só role e horário), recebido no lote 6, ao lado da amostragem de sessões atual (`security.account_inactive_review`, `10_account_activity.sql`), com a fonte explícita. Responde à P-03.
- **ADR-0001 D1:** o agregado chega com o token por ambiente, no escopo `last-login:write`.
- **ADR-0001 D11:** tenant = ambiente. A campanha, a matriz, o mapeamento de responsáveis, as decisões e o último login pertencem ao ambiente.

### Dependências

Os lotes 1–5 não têm dependência bloqueante. O lote 6 depende do plano 01, lote 3 (token) e do plano 10, lote 4 (formato do arquivo de último login), por isso é **implementado depois do lote 4 do plano 10**, junto com o lote 5 do plano 10. Uso opcional da 04 (timeline: quando o privilégio apareceu). É **pré-requisito** da 05 (lote 1: matriz de privilégio efetivo com origem e privilégio por coluna).

## 1. Módulos e arquivos afetados

| Área | Caminho | Mudança |
|---|---|---|
| Coleta de origem | `backend/sql/collectors/postgres/effective_grants.sql`, novos `grant_origins.sql` e `column_grants.sql`, `backend/internal/collectors/postgres/account_roles.go` (`membershipModes`, limiar `>= 160000`), novo `collectors/postgres/grant_origin.go`, `internal/audit/live_registry_postgres.go` | cadeia de membership por privilégio, PUBLIC, roles predefinidas; ACL de coluna (`pg_attribute.attacl`); sequences, funções (EXECUTE) e schemas (CREATE/USAGE, `schema_snapshot.nspacl`) |
| Store | `backend/internal/repository/account_roles.go`, novos `access_review.go` e `grant_origin_snapshots.go` | — |
| Regras | `backend/internal/analyzer/access_review.go` (novo), `analyzer/security.go` (reuso de `security.powerful_role`, `security.account_inactive_review`, `security.excessive_privilege`, `security.public_schema_create`), `backend/internal/actions/catalog.go` | — |
| Ações assistidas | `backend/internal/repository/actions.go`, `action_measurements.go` | decisão "revogar" cria `finding_action` com plano e medição na coleta seguinte |
| API e RBAC | `backend/internal/api/access_review.go` (novo), `backend/internal/api/auth.go` (`authorizeRequest`: decisões exigem auditor; o handler confere se quem decide é o responsável da linha ou operator) | — |
| Último login | novos `backend/internal/api/account_last_login.go` e `backend/internal/repository/account_last_login.go`, `backend/internal/api/auth.go` (escopo `last-login:write`) | ingestão do agregado da spec 10 |
| Exportação | padrão de streaming de `backend/internal/api/action_measurements.go` (`# export_complete`) | CSV |
| PDF | `backend/internal/report/` (novo tipo `access_review`), `backend/internal/reportworker/worker.go`, `report/redaction.go` | — |
| Frontend | `frontend/src/pages/AccessReviewPage.tsx` (novo), `pages/parts/AccessReviewMatrix.tsx` (virtualizada), `services/api.ts`, `components/layout/Shell.tsx`, `app.tsx` | — |
| Docs | `docs/rules.md` (origem por versão, ligada à seção "Roles poderosas e versão do PostgreSQL (#126, #134)"), `docs/ops.md`, `docs/guia-do-usuario.md` | — |

## 2. Migrations (provisórias)

| Nº provisório | Arquivo | Lote | Conteúdo |
|---|---|---|---|
| 33 | `backend/migrations/33_grant_origin_column_privilege.sql` | 1 | `grant_origin_snapshot` (execução, ambiente, banco, conta com LOGIN, tipo de objeto `table`/`view`/`sequence`/`function`/`schema`, objeto qualificado, privilégio, `origin_kind` `direct`/`role`/`public`/`predefined`, `via_chain text[]`, `inherit`/`set` quando PG 16+); `column_grant_snapshot` (execução, objeto, coluna, conta, privilégio, origem). Snapshots antigos ficam "não coletado" |
| 34 | `backend/migrations/34_access_review.sql` | 2 | `access_review_owner` (ambiente, role ou conta, `owner_user_id` → `auditor_user` com papel auditor ou acima no ambiente, `contact_name` e `contact_email` opcionais; ADR-0001 D7), `access_review_campaign` (ambiente, escopo, `reference_run_id` completa, prazo, estado `open`/`in_review`/`closed`/`overdue`, criador), `access_review_item` (matriz congelada da coleta de referência + destaques), `access_review_decision` (append-only: item, decisão `approve`/`revoke`/`keep`, prazo de revogação, justificativa, `decided_by`, `decided_at`, `supersedes_id`, `finding_action_id`); amplia o CHECK de `report_job.report_type` com `access_review` (padrão de `19_architecture_report.sql`) |
| 40 | `backend/migrations/40_account_last_login.sql` | 6 | `account_last_login_observation` (ambiente, role, último horário, fonte `log`, id do token, recebido em). Só role e horário (ADR-0001 D10) |

Os números supõem 26–32 já mergeados. O 40 vem depois do 39 do plano 08, porque o lote 6 é implementado depois do lote 4 do plano 10. Lembrar `ci.yml`, `migrations/README.md` e `docs/ops.md`.

## 3. Endpoints e telas

| Método e rota | Papel | Observação |
|---|---|---|
| `GET`/`PUT /api/v1/environments/{id}/access-review-owners` | viewer / operator | mapeamento role/conta → conta do auditor (auditor ou acima no ambiente) + contato livre opcional. Responsável sem acesso: 400 |
| `POST /api/v1/environments/{id}/access-reviews` | operator | 409 se a coleta de referência for parcial |
| `GET /api/v1/environments/{id}/access-reviews`, `GET .../{campaign}` | viewer | lista e progresso (% decidido, pendências) |
| `GET /api/v1/environments/{id}/access-reviews/{campaign}/matrix?account=&object=&privilege=&origin=&highlight=&pending=&owner=&mine=&limit=&offset=` | viewer | filtrada e paginada no servidor. `mine=true` = "minhas pendências" do usuário logado |
| `POST /api/v1/environments/{id}/access-reviews/{campaign}/decisions` | responsável da linha (auditor ou acima); operator nas linhas sem responsável | por linha ou em lote. 403 para quem não é o responsável. 400 para "manter" sem justificativa. 409 em campanha concluída |
| `POST /api/v1/environments/{id}/access-reviews/{campaign}/close` | operator | — |
| `GET /api/v1/environments/{id}/access-reviews/{campaign}/diff` | viewer | contra a campanha anterior |
| `GET /api/v1/environments/{id}/access-reviews/{campaign}/export/download?format=csv` | auditor | o sufixo `/download` reaproveita a regra de download de `authorizeRequest` |
| `POST /api/v1/environments/{id}/reports` com `report_type=access_review` | auditor | rota existente de PDF |
| `POST /api/v1/environments/{id}/account-last-logins` | token `last-login:write` | lote 6. Corpo só com `role` e `last_login_at`. Campos extras: 400. Token de outro ambiente: 403 |

**Tela (Trinity necessária).** A spec diz "Em Segurança, nova área". Hoje "Segurança" é só uma categoria de `FindingsBrowser` e uma aba de `InventoryDetailTabs`, sem item de navegação, e "Contas" é a gestão de contas do próprio auditor (fora de escopo). A posição na navegação fica adiada para o lote 5, quando Q chamará a Trinity (ADR-0001, Pendente). Este plano não decide o lugar. A Trinity também desenha a matriz grande com filtros e lote, o diff e a visão "minhas pendências".

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

**Issue:** `feat(access-review): campanhas por ambiente, matriz efetiva congelada da coleta de referência e mapeamento de responsáveis (contas do auditor com contato opcional)`

**Descrição.** Migration `34_access_review.sql` (provisória). A criação da campanha (operator) exige coleta de referência **completa** (senão 409) e congela a matriz em `access_review_item` com os destaques do RF-04: superusuário e poderes de `security.powerful_role`, conta sem sessão observada ou com validade vencida (`security.account_inactive_review`), escrita em schema de outra aplicação e privilégio novo desde a campanha anterior. Responsável = conta do auditor com papel auditor ou acima no ambiente (`auditor_user_environment`), com contato livre opcional (ADR-0001 D7). Mapear conta sem esse acesso gera 400. Linha sem responsável vai para o operator (RF-05). Matriz paginada e filtrada no servidor. Estados e vencimento por prazo.

**Critérios de aceite.**
- [ ] Coleta de referência parcial: 409 ("campanha exige coleta completa").
- [ ] RNF-03: 500 contas × 5000 objetos, menos de 2 s por página (teste de escala).
- [ ] RNF-04: viewer lê. Operator cria e mapeia responsáveis. Auditor não cria campanha (403).
- [ ] Responsável sem acesso ao ambiente, ou só com papel viewer: 400 ("responsável sem acesso ao ambiente"), e nada é gravado.
- [ ] O e-mail de contato opcional é validado em formato e tamanho. Ele é tratado como dado pessoal: não entra em logs nem métricas.
- [ ] RNF-06: a matriz congelada não muda quando chegam coletas novas.

**Arquivos.** `migrations/34_access_review.sql`, `internal/repository/access_review.go`, `internal/repository/accounts.go` (consulta de papel por ambiente em `auditor_user_environment`), `internal/api/access_review.go`, `internal/api/health.go`, `api/openapi.json`.
**Estimativa:** 6 d. **Gate:** **R2** (dado pessoal: nomes de role e contato do responsável). **Depende de:** lote 1.

### Lote 3 — Decisões imutáveis, revogação como ação assistida e regras

**Issue:** `feat(access-review): decisões aprovar/revogar/manter imutáveis, revogação via ação assistida e regra de revogação não aplicada (RF-06, RF-07)`

**Descrição.** `POST .../decisions` (auditor; por linha ou em lote). "Manter" exige justificativa. A decisão é append-only, e uma correção é uma nova decisão com `supersedes_id`. Toda decisão vai para `auditor_operation_log`. "Revogar" com prazo cria uma `finding_action` (plano e execução externa), com medição na coleta seguinte. Regras em `analyzer/access_review.go`: revogação não aplicada após o prazo (alta, vinculada à ação), superusuário/BYPASSRLS sem "aprovar" (alta), conta sem sessão ≥ 90 dias com escrita (média), privilégio via PUBLIC fora de `public` (média), privilégio novo sem decisão (média), campanha vencida com ≥ 10% pendente (média). Todos os limiares são **valor inicial, calibrar**. `authorizeRequest` passa a exigir auditor para `/access-reviews/{campaign}/decisions`. O handler confere, linha a linha, que quem decide é o responsável dela, ou operator quando a linha não tem responsável (ADR-0001 D7). Decisão em lote com uma linha de outro responsável é recusada inteira (403). Se o responsável perde acesso ao ambiente, as linhas pendentes dele voltam para o operator. Métricas de campanhas por estado e linhas pendentes (RNF-05).

**Critérios de aceite.**
- [ ] CA-03: "revogar" com prazo vencido e privilégio presente na coleta completa seguinte gera achado alto de "revogação não aplicada", vinculado à ação assistida.
- [ ] CA-04: "manter" sem justificativa recebe 400 e nada é gravado (contagem de linhas igual antes e depois).
- [ ] CA-05: viewer recebe 403 ao decidir.
- [ ] Auditor que não é o responsável da linha recebe 403. Operator decide as linhas sem responsável. Lote misto é recusado inteiro (contagem igual antes e depois).
- [ ] Decisão em campanha concluída recebe 409.
- [ ] Toda decisão aparece em `auditor_operation_log`.
- [ ] As regras novas têm os cinco campos e consulta segura no catálogo.

**Arquivos.** `internal/api/access_review.go`, `internal/api/auth.go`, `internal/repository/access_review.go`, `internal/repository/actions.go`, `internal/analyzer/access_review.go`, `internal/actions/catalog.go`, `internal/observability/metrics.go`, `docs/rules.md`.
**Estimativa:** 6 d. **Gate:** **R2** (mudança em `authorizeRequest` e autorização por responsável). **Depende de:** lote 2.

### Lote 4 — Diff entre campanhas, exportação CSV e PDF de evidência

**Issue:** `feat(access-review): diff com a campanha anterior, export CSV streaming e PDF de evidência com redação (RF-08, RF-09)`

**Descrição.** Diff: privilégios novos e removidos, decisões que mudaram, contas novas e removidas. CSV com as colunas do §4 (conta, objeto qualificado, tipo, privilégio, origem, destaque, responsável, decisão, justificativa, decidido por, decidido em), em streaming e terminando em `# export_complete` com a contagem de linhas. A coluna "responsável" traz a conta do auditor e o contato opcional. Download exige auditor. **XLSX fica fora do v1 (ADR-0001 D7).** PDF `access_review`: resumo, responsáveis, % decidido e pendências, respeitando a redação. No modo "identificadores", os nomes de conta somem e as contagens ficam.

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

**Descrição.** Tela desenhada com a Trinity (incluindo a posição na navegação, adiada até aqui). Lista de campanhas. Matriz virtualizada (`docs/ui-virtualization.md`) com filtros de conta, objeto, privilégio, origem, destaque e pendente. Decisão por linha e em lote, com justificativa obrigatória em "manter". Diff com a anterior. Visão **"minhas pendências"** (linhas do usuário logado, `mine=true`) e, para operator, pendências por responsável (ADR-0001 D7). Botões de exportar e de PDF.

**Critérios de aceite.**
- [ ] CA-01 visível: origem "via g_leitura" na linha.
- [ ] CA-04 e CA-05 na UI: botão "manter" desabilitado sem justificativa. Viewer, e auditor fora das próprias linhas, não veem os controles de decisão (o backend continua sendo a fonte do 403).
- [ ] "Minhas pendências" lista só as linhas do usuário logado.
- [ ] Matriz de 500 × 5000 navegável sem travar (medição de heap como em `scripts/measure-table-heap.mjs`).
- [ ] Layout aprovado pela Trinity.

**Arquivos.** `frontend/src/pages/AccessReviewPage.tsx`, `frontend/src/pages/parts/AccessReviewMatrix.tsx`, `frontend/src/services/api.ts`, `frontend/src/components/layout/Shell.tsx`, `frontend/src/app.tsx`, `docs/guia-do-usuario.md`.
**Estimativa:** 6 d. **Gate:** CI verde + Trinity. **Depende de:** lotes 3 e 4.

### Lote 6 — Último login vindo dos logs (agregado da spec 10)

**Issue:** `feat(access-review): receber o agregado de último login por role (só role e horário) via token last-login:write e exibir na matriz e na regra de conta sem uso`

**Descrição.** Implementa a ADR-0001 D10 do lado do auditor. Migration `40_account_last_login.sql` (provisória). Rota `POST .../account-last-logins` com o token por ambiente no escopo novo `last-login:write`, acrescentado à lista fechada do plano 01. O corpo aceita **só** `role` e `last_login_at`. Qualquer outro campo (IP, banco, aplicação) é recusado com 400, e o tamanho é limitado. Grava a observação mais recente por role, com o id do token. A matriz e a regra de conta sem uso passam a mostrar "último login (log)" ao lado de "sem sessão observada (amostra)", com a fonte explícita. Métricas de envios recebidos e rejeitados (RNF-05 da spec 10). Implementado depois do plano 10, lote 4, que define o formato do arquivo.

**Critérios de aceite.**
- [ ] Só role e horário são guardados (CA-07 da spec 10): o teste varre a tabela e o log da requisição.
- [ ] Corpo com campo extra recebe 400, e nada é gravado.
- [ ] Token de outro ambiente recebe 403 (CA-06 da spec 10, lado da API). Token sem o escopo recebe 403.
- [ ] Sem nenhum envio, o comportamento dos lotes 1–5 continua idêntico.
- [ ] A regra de conta sem uso cita a fonte (log ou amostra) na evidência.

**Arquivos.** `migrations/40_account_last_login.sql`, `internal/api/account_last_login.go`, `internal/repository/account_last_login.go`, `internal/api/auth.go`, `internal/analyzer/access_review.go`, `internal/observability/metrics.go`, `api/openapi.json`, `.github/workflows/ci.yml`, `docs/ops.md`.
**Estimativa:** 3 d. **Gate:** **R2** (dado pessoal de autenticação; novo escopo de token). **Depende de:** lote 3, plano 01 lote 3 e plano 10 lote 4.

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
| `TestOwnerWithoutEnvironmentAccess400` | API | RNF-04, §4 Erros |
| `TestOnlyOwnerOrOperatorDecides` (outro auditor 403; lote misto recusado) | API com `authMiddleware` | RNF-04, RF-06, ADR-0001 D7 |
| `TestMyPendingOnlyOwnRows` | API | §4 UI ("minhas pendências") |
| `TestLastLoginIngestOnlyRoleAndTime` (campo extra 400; varredura do store) | API + integração | CA-07 da spec 10, ADR-0001 D10 |
| `TestLastLoginTokenOtherEnvironment403` | API com `authMiddleware` | CA-06 da spec 10 |

## 6. Gates

| Lote | Gate | Motivo |
|---|---|---|
| 1 | **R2** | resolução de privilégio (segurança) e novas consultas no alvo |
| 2 | **R2** | dados pessoais (role, e-mail do responsável) |
| 3 | **R2** | mudança em `authorizeRequest` |
| 4 | **R2** | exportação e redação de dados pessoais |
| 5 | CI verde + Trinity | UI |
| 6 | **R2** | último login (dado pessoal) e novo escopo de token |

## 7. Riscos

- Uma matriz grande demais para revisão humana. Mitigação: lote, filtros por destaque e agrupamento por responsável.
- "Sem sessão observada" é amostral e não embasa revogação sozinho. A ressalva aparece na matriz e no CSV.
- Nomes de role e o contato livre do responsável são dados pessoais. Mitigação: redação no PDF, download só para auditor e nada em log ou métrica.
- Um responsável que perde acesso ao ambiente deixa linhas paradas. Mitigação: as linhas voltam para o operator, e a métrica de linhas pendentes mostra a situação por responsável.
- Consultas de origem por cadeia de membership podem ser caras em clusters com muitas roles. Mitigação: orçamento e cobertura parcial.

## 8. Pontos para Q (resolvidos)

- ~~Responsável em texto livre × RNF-04.~~ **Resolvido por ADR-0001 D7:** vale o RNF-04. O responsável é uma conta do auditor com contato livre opcional. Voltam o erro 400, a decisão pelo responsável e "minhas pendências".
- ~~XLSX.~~ **Resolvido por ADR-0001 D7:** CSV no v1, XLSX depois. RF-09 e RNF-03 da spec foram ajustados.
- ~~Caminho do último login.~~ **Resolvido por ADR-0001 D10:** no v1 sobe só o agregado de role e horário. O lote 6 deixou de ser condicional.
- **Pendente (fora do escopo de Q):** posição da área na navegação, com a Trinity no lote 5.

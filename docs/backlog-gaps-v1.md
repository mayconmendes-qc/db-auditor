# Backlog curto — gaps pós-merge do roadmap v1

Escopo: fechar CAs e DoD que ficaram abertos depois do PR #137 (`feat/roadmap-v1-plans` → `main`).  
**Não** reabre desenho de feature, ADR-0001 nem specs. Só contrato, testes e happy-path que o plano já pediu.

Origem: auditoria pós-merge em `main` @ `1249c1b`.  
Planos: [`docs/plan/`](plan/README.md). Specs: [`docs/spec/`](spec/README.md).

## Ordem sugerida

| # | Item | Plano | Prioridade | Esforço |
|---|---|---|---|---|
| G1 | OpenAPI dos restore drills | 08 | P0 | S |
| G2 | Happy-path `restore-verify` em container | 08 | P0 | M |
| G3 | `TestNoSampledValueAnywhere` (PII CA-02) | 05 | P0 | M |
| G4 | API tests DQ (CA-03, CA-08 + mínimos) | 03 | P1 | M |
| G5 | CAs access-review (auth/diff/imutável) | 07 | P1 | M |
| G6 | CAs digest (link alerta, destinatários, isolamento) | 06 | P1 | M |

Um PR por item (ou G1+G2 juntos se o OpenAPI for pré-requisito trivial do happy-path). Gate: CI verde; G1/G3/G5/G6 tocam auth, isolamento ou dado pessoal → preferir R2.

---

## G1 — OpenAPI restore drills (08)

**Problema.** Handlers em `backend/internal/api/restore_drills.go` existem; rotas **não** estão em `backend/api/openapi.json` (DoD do plano: rota documentada + `openapi_test.go` + `bun run gen:openapi`).

**Fazer**
- [ ] Documentar em `backend/api/openapi.json`:
  - `GET /api/v1/environments/{id}/restore-drill-context`
  - `POST /api/v1/environments/{id}/restore-drills`
  - `GET /api/v1/environments/{id}/restore-drills`
  - `GET /api/v1/environments/{id}/restore-drills/{drill}`
  - `GET /api/v1/environments/{id}/restore-drill-policy`
  - `PUT /api/v1/environments/{id}/restore-drill-policy`
- [ ] Incluir os paths na lista exigida por `backend/internal/api/openapi_test.go` (se aplicável).
- [ ] Regenerar `frontend/src/types/openapi.ts` (`bun run gen:openapi`) sem diff residual.

**Aceite.** `go test ./internal/api -run OpenAPI` e job Frontend OpenAPI sem diff. Nenhuma mudança de comportamento da API.

---

## G2 — Happy-path restore (08)

**Problema.** Pacote `restoreverify` cobre pré-validação/isolamento com `FakeRuntime`; falta o happy-path de container do plano (CA-01 / CA-08 / CA-09 conforme lote).

**Fazer** (sem mudar o desenho D8: só CLI, sem restore in-API)
- [ ] `TestRestoreVerifyHappyPath` (ou nome equivalente): dump válido + confirmação de isolamento + **sem** `--sample-hash` → inventário/contagens/integridade, `(c) desabilitada`, container destruído, exit 0.
- [ ] Assertar ausência de container/volume órfão com o rótulo da execução após falha injetada no meio (CA-08).
- [ ] Opt-in `--sample-hash`: sem DSN → exit 2; DSN de escrita → exit 4; DSN só leitura → caminho CA-09 sem vazar DSN em log/evidência/`ps` (sanitização já existente).
- [ ] Rodar sob a mesma restrição de major `pg_dump`/`pg_restore` já documentada em `docs/ops.md` (matriz PG ou job dedicado descartável).

**Aceite.** Teste verde na CI (ou job de integração marcado); zero valor de linha na evidência quando sample-hash divergir (CA-05, se coberto neste PR).

**Fora de escopo.** Introduzir lib `age` ou mudar fingerprints/política — só se um CA bloqueante exigir e com ADR nova (não neste backlog).

---

## G3 — PII CA-02: nenhum valor amostrado (05)

**Problema.** Há scrub unitário; falta a varredura forte do plano (`TestNoSampledValueAnywhere`).

**Fazer**
- [ ] Integração no padrão dos testes de redação de `internal/quality`: após scan com CPFs sintéticos conhecidos, procurar cada CPF (completo, só dígitos, máscara `***.***.***-**` com dígitos parciais, sha256/md5) em:
  - snapshot store (todas as tabelas tocadas pelo PII),
  - logs capturados do processo,
  - evidência dos achados,
  - CSV de export,
  - mensagens de erro,
  - `/metrics`.
- [ ] Zero ocorrências (CA-02, RNF-02, RNF-05).
- [ ] Ideal no mesmo PR ou follow-up curto: `TestDiscardPersistsAcrossScans` (CA-07).

**Aceite.** Teste falha se qualquer forma do valor vazar; passa no store descartável da CI.

---

## G4 — API tests DQ (03)

**Problema.** Lógica e rotas existem; falta `quality_contracts_test.go` (ou equivalente) cobrindo CAs de API/auth do plano.

**Fazer** (mínimo P1)
- [ ] `TestViewerCannotApprove` → viewer 403 em `POST .../versions/{version}/approve` (CA-08).
- [ ] `TestContractRunDisabled409AndCLIExit2` → flag/processo desligado → API 409 e CLI exit 2 (CA-03).
- [ ] `TestCLIGateHighUniqueViolationExit1` ou API equivalente do gate (CA-05) se couber sem reabrir o motor.

**Desejável no mesmo eixo (P2, mesmo PR se barato)**
- [ ] `TestNullCheckStoresOnlyCounts` / `TestFreshnessStoresHoursNotTimestamp` (CA-02 / CA-04 — só contagens/idade, sem valor de linha nem carimbo).
- [ ] `TestPersistentViolationSingleAlert` (CA-07 / D13).

**Aceite.** `go test ./internal/api -run QualityContract` (e CLI se aplicável) verde; viewer nunca aprova.

**Fora de escopo.** Editor/diff YAML na UI (Trinity do lote 6) — backlog de UX separado, se necessário.

---

## G5 — CAs access-review (07)

**Problema.** Campanhas/matriz/L6 ok; faltam CAs de autorização e imutabilidade do plano.

**Fazer**
- [ ] `TestOnlyOwnerOrOperatorDecides` — responsável decide a própria linha; operator nas sem responsável; outro auditor 403 (D7).
- [ ] `TestMyPendingOnlyOwnRows` — filtro `mine` só do responsável autenticado.
- [ ] `TestAccessReviewDiffNewPrivilege` — diff marca privilégio novo (CA-02 do plano 07).
- [ ] `TestClosedCampaignImmutable` — campanha fechada rejeita nova decisão.

**Desejável**
- [ ] Smoke de componente ou API da matriz com `last_login_at` preenchido via `POST .../account-last-logins` (só role+time, D10).
- [ ] `TestAccessReviewMatrixScaleBudget` se já houver harness de escala.

**Aceite.** Testes API com `authMiddleware`; tenant = ambiente (D11) preservado.

---

## G6 — CAs digest (06)

**Problema.** Motor/UI ok; faltam CAs de vínculo de alerta, destinatários e isolamento.

**Fazer**
- [ ] `TestDigestLinksOpenWorkloadAlert` — item linka alerta `workload` aberto; `regression_alert` não ganha linha nova (CA-04 / RF-08).
- [ ] `TestDigestRecipientsPerEnvironmentWithGlobalFallback` — destinos do ambiente com fallback global (D6).
- [ ] `TestDigestNeverMixesEnvironments` — digest/entrega de A nunca usa destinos/dados de B (RNF-04).
- [ ] `TestDigestDeliveryFailureRecordedAndRetried` — falha de webhook registrada e retentável (RNF-05).

Já cobertos em grande parte (não reabrir): classificação CA-01, subjanela CA-02, partial edge CA-03, Mongo N/A CA-06, config operator CA-07 / `TestNotificationDestinationsRequireOperator`.

**Aceite.** Integração com store + HTTP fake; literais de SQL ausentes no Markdown/webhook (CA-05 já existente — não regredir).

---

## Checklist de PR (todos os itens)

- [ ] Não altera ADR-0001 nem texto de spec (só testes/contrato/OpenAPI/tipos).
- [ ] `gofmt`, `sqlc` sem diff se tocar SQL, staticcheck/golangci verdes.
- [ ] OpenAPI + `gen:openapi` quando houver rota nova ou path antes omitido.
- [ ] Nada de DSN, token, SQL bruto ou valor de linha em log/evidência/export (`SanitizeError` / padrões PII/DQ).
- [ ] Título do PR cita plano + CAs (ex.: `test(05): CA-02 no sampled value anywhere`).

## Fora deste backlog

- Editor/diff YAML de contratos na UI (03 L6 Trinity).
- Benchmark RNF-03 de `log-anomaly` (≥50 MB/s).
- Testes nomeados menores de 01/02/04/09 (Action comment, cost series gaps, etc.).
- Lib `age` no restore verifier.
- Qualquer mudança de produto, regra de negócio ou ADR.

# Plano 01 — Risco de migrations com contexto do banco real (`migration-risk`)

- **Spec:** [../spec/01-migration-risk.md](../spec/01-migration-risk.md)
- **Base lida:** `main` em `b1a93cf`
- **Posição na ordem de implementação:** 3ª
- **Total:** 6 lotes, **25 d**, 5 lotes com gate R2, Trinity necessária (2 lotes, escopo pequeno)

## 0. Resumo

Este plano cria o **modo CLI** do binário `backend/cmd/auditor`, que hoje só tem `recover-operator`. Esse modo é a base comum das specs 03, 08 e 10. O plano também cria a **credencial de API por ambiente** que todas as CLIs usam, um parser de DDL PostgreSQL em Go puro, o catálogo MR-01…MR-11, a integração com GitHub Actions (um único comentário por PR) e o histórico de análises por ambiente.

### Decisões padrão aplicadas

- **decisão padrão (Q, nº 1):** a CLI usa um **token novo por ambiente, com escopo mínimo**. `AUDITOR_REPORT_API_TOKEN` (`internal/api/pdf_reports.go`, `authorizeReport`) **não** é reutilizado. Responde à P-01.
- **decisão padrão (Q, nº 2):** **o CI usa só o último snapshot, via API** (modo a). **O runner nunca recebe a senha do banco.** A GitHub Action não expõe o modo (b). Responde à P-02.
- **decisão padrão (Q, nº 11):** tenant = ambiente. Um token pertence a um único ambiente e não lê nem escreve em outro (RNF-04).

### Dependências

Nenhuma bloqueante. A 04 é opcional: o contexto pode trazer "tabela mudou desde o snapshot" a partir da timeline (`schema_change_event`). Este plano é **pré-requisito** da 03 (lote 5), da 08 (lotes 1 e 3) e da 10 (lote 5 condicional).

## 1. Módulos e arquivos afetados

| Área | Caminho | Mudança |
|---|---|---|
| Entrada do binário | `backend/cmd/auditor/main.go` | despacho de subcomandos (`migration-risk`, e depois `dq-contracts`, `restore-verify`, `log-anomaly`) antes de `config.Load()`, mantendo `recover-operator` |
| Base de CLI | `backend/internal/cli/` (novo: `exit.go` com os códigos 0–4, `output.go` Markdown/JSON com versão de esquema, `secrets.go` só variável ou arquivo `*_FILE`, `apiclient.go` com token e TLS) | comum a 01, 03, 08 e 10 |
| Parser e regras | `backend/internal/migrationrisk/` (novo: `lexer.go`, `parser.go`, `locks.go` com tabela por major 14–18, `rules.go` MR-01…MR-11, `suppress.go`, `context.go`, `report.go`, `testdata/` golden) | Go puro: a imagem é compilada com `CGO_ENABLED=0` (`backend/Containerfile`), então não dá para usar libpg_query/cgo |
| Normalização sem literais | `backend/internal/fingerprint/` (novo `literals.go`), extraído de `collectors/postgres/stats_workload.go` (`NormalizeQuery`: `quotedPattern`, `dollarPattern`, `numberPattern`) | hoje `fingerprint.NormalizeSQL` **não** remove literais; a remoção mora no coletor de workload |
| Token por ambiente | `backend/internal/repository/environment_tokens.go` (novo), `backend/internal/api/auth.go` (`authMiddleware`, `authorizeRequest`), `backend/internal/api/environment_tokens.go` (novo) | principal "token" separado do usuário |
| Contexto | `backend/internal/api/migration_context.go` (novo), `backend/internal/repository/migration_context.go` (novo) sobre `table_snapshot` e a regra de comparabilidade de `analyzer/maintenance_trend.go` | — |
| Conexão direta | `backend/internal/collectors/postgres/readonly_connection.go` (`ConnectReadOnly`), `backend/internal/config/target_security.go`, `config/sanitize.go` | reuso sem mudança |
| Histórico | `backend/internal/repository/migration_analysis.go`, `backend/internal/api/migration_analysis.go` (novos) | — |
| Métricas | `backend/internal/observability/metrics.go` | `auditor_migration_analyses_total{result}`, `auditor_migration_parse_errors_total`, `auditor_env_token_requests_total{scope,result}` |
| GitHub | `.github/actions/migration-risk/action.yml` (novo, composite), `.github/workflows/release-cli.yml` (novo), job de autoteste em `.github/workflows/ci.yml` | — |
| Frontend | `frontend/src/pages/EnvironmentsPage.tsx` (cartão de tokens), `frontend/src/pages/MigrationAnalysesPage.tsx` (novo) ou aba em Acompanhamento, `frontend/src/services/api.ts` | — |
| Docs | `docs/migration-risk.md` (novo: uso da CLI e da Action, supressão, exemplos), `docs/rules.md` (catálogo MR-*), `docs/ops.md` (tokens: criação, rotação, revogação), `docs/hardening.md` (principal token) | — |

## 2. Migrations (provisórias)

| Nº provisório | Arquivo | Lote | Conteúdo |
|---|---|---|---|
| 29 | `backend/migrations/29_environment_api_token.sql` | 3 | `environment_api_token`: id, `environment_id`, nome, prefixo público, `token_sha256 bytea UNIQUE`, `scopes text[]` com CHECK no conjunto conhecido, `created_by`, `created_at`, `expires_at` obrigatório, `revoked_at`, `last_used_at`. Nunca guarda o token em claro |
| 30 | `backend/migrations/30_migration_analysis.sql` | 6 | `migration_analysis` (ambiente, repositório, PR, commit, maior severidade, versão das regras, versão do PG alvo, modo de contexto, `audit_run_id` de referência, truncado), `migration_analysis_item` (arquivo, linha, código MR, tipo de operação, objeto qualificado, lock, reescrita/varredura, tamanho, duração estimada, severidade, confiança, suprimida + justificativa). **Sem texto SQL nem literais** |

Os números supõem 26 (plano 06) e 27–28 (plano 04) já mergeados. Lembrar `ci.yml`, `migrations/README.md` e `docs/ops.md`.

## 3. Endpoints e telas

| Método e rota | Quem | Observação |
|---|---|---|
| `POST /api/v1/environments/{id}/api-tokens` | operator (sessão) | devolve o token **uma vez**. Escopos escolhidos de uma lista fechada |
| `GET /api/v1/environments/{id}/api-tokens` | operator | só metadados (prefixo, escopos, validade, último uso) |
| `DELETE /api/v1/environments/{id}/api-tokens/{token}` | operator | revogação imediata |
| `GET /api/v1/environments/{id}/migration-context?tables=s.t,...` | token `snapshot-context:read` ou viewer | tamanho, linhas estimadas, taxa de escrita, `server_version_num`, execução de referência, `partial`, idade |
| `POST /api/v1/environments/{id}/migration-analyses` | token `migration-analysis:write` | corpo validado: nenhum campo de texto livre além de PR, commit e justificativa de supressão (limitados em tamanho) |
| `GET /api/v1/environments/{id}/migration-analyses`, `GET .../{analysis}` | viewer | — |
| `POST /api/v1/environments/{id}/annotations` | **também** token `deploy-annotation:write` | rota existente (hoje exige auditor). Atende a mitigação de risco da spec 04 |

**Telas (Trinity necessária, escopo pequeno).** (1) Cartão "Tokens de API" em Ambientes: criar com escopos, exibir uma vez, revogar. (2) Lista "Análises de migration" por ambiente, com PR, commit, maior severidade e data, e detalhe com a mesma tabela do Markdown.

## 4. Lotes

### Lote 1 — Fundação do modo CLI

**Issue:** `feat(cli): subcomandos no binário auditor, códigos de saída 0–4, saída Markdown/JSON e segredos só por env/arquivo`

**Descrição.** `main.go` passa a despachar subcomandos antes de carregar a configuração da API. `recover-operator` continua igual. Criar `internal/cli` com: os códigos do README das specs (0 nada acima do limiar, 1 bloqueio, 2 uso ou configuração, 3 dados insuficientes ou parcial, 4 recusa de segurança); o escritor Markdown/JSON com `schema_version`; a leitura de segredos **só** por variável ou arquivo (`AUDITOR_CLI_TOKEN` / `AUDITOR_CLI_TOKEN_FILE`), com recusa de flags como `--token`/`--password`; e um cliente HTTP com TLS verificado e timeout. Logs sanitizados (`config.SanitizeError`).

**Critérios de aceite.**
- [ ] `auditor help` lista os subcomandos. `auditor recover-operator` mantém o comportamento (teste existente verde).
- [ ] Um segredo passado como argumento sai com código 2 e mensagem sem o valor.
- [ ] Teste de tabela dos códigos 0–4.
- [ ] A imagem distroless (`backend/Containerfile`) continua com `ENTRYPOINT ["/auditor"]`, e a API sobe sem argumentos.

**Arquivos.** `cmd/auditor/main.go`, `internal/cli/*`.
**Estimativa:** 2 d. **Gate:** **R2** (base de manuseio de segredos de todas as CLIs). **Depende de:** —

### Lote 2 — Parser de DDL e catálogo MR-01…MR-11 (modo sem contexto)

**Issue:** `feat(migration-risk): parser DDL PostgreSQL, regras MR-01..MR-11 por versão 14–18, supressão por linha e saída sem literais`

**Descrição.** Um lexer em Go puro (comentários, `$tag$`, aspas, `E''`) e um parser do subconjunto de DDL da §3: `CREATE INDEX` sem `CONCURRENTLY`, `ALTER TABLE ... ADD COLUMN ... DEFAULT` volátil, `ALTER COLUMN TYPE`, `SET NOT NULL`, `ADD CONSTRAINT ... FOREIGN KEY/CHECK` sem `NOT VALID`, `PRIMARY KEY/UNIQUE` sem `USING INDEX`, `DROP`/`RENAME`, `VACUUM FULL`, `CLUSTER`, `REFRESH MATERIALIZED VIEW` sem `CONCURRENTLY`, `SET lock_timeout`, `BEGIN`/`COMMIT`. Cada operação sai com arquivo e linha. A tabela de locks fica em `locks.go`, por major 14–18, com referência à página da documentação oficial de cada versão (RF-03). Exemplo de diferença por versão a cobrir: o PG 18 aceita `NOT NULL ... NOT VALID`, o que muda a alternativa segura do MR-04. Operação que não roda em transação dentro de `BEGIN` (MR-11) e combinações inseguras na mesma transação (RF-07). Supressão por linha `-- db-auditor:ignore MR-01 motivo: <texto>`, com justificativa obrigatória (RF-08). Normalização sem literais extraída para `internal/fingerprint/literals.go`. Tabela criada na própria migration gera severidade informativa. Limite de 500 operações, com aviso. Subcomando `auditor migration-risk --path <arq|dir>... --pg-version N --fail-on high --format md|json`.

**Critérios de aceite.**
- [ ] CA-02: MR-01 sem contexto sai média, com confiança baixa e impacto "não estimado" (com motivo, AN-03), código 0 com gate padrão.
- [ ] CA-03: um literal de e-mail num `UPDATE` não aparece no Markdown nem no JSON.
- [ ] CA-07: operação suprimida aparece "suprimida" com a justificativa e não conta para o gate. Supressão sem justificativa resulta em código 2.
- [ ] CA-08: golden por versão (14, 15, 16, 17, 18) da mesma migration. A saída segue a tabela daquela versão.
- [ ] RNF-03: 200 arquivos / 2 MB em menos de 10 s (benchmark no CI).
- [ ] RNF-06: a mesma entrada produz saída byte a byte igual (golden).
- [ ] Erro de parse marca a operação "não analisada" com arquivo:linha e código 3.
- [ ] Fuzz test do lexer (`go test -fuzz` curto no CI) sem pânico.

**Arquivos.** `internal/migrationrisk/*`, `internal/fingerprint/literals.go`, `internal/collectors/postgres/stats_workload.go` (passa a chamar a função extraída, sem mudar a saída: golden de workload verde), `docs/rules.md`, `docs/migration-risk.md`.
**Estimativa:** 6 d. **Gate:** CI verde (análise estática local, sem dado de banco nem credencial). **Depende de:** lote 1.

### Lote 3 — Token de API por ambiente com escopo mínimo

**Issue:** `feat(auth): token de API por ambiente com escopos mínimos para CLIs; não reutiliza AUDITOR_REPORT_API_TOKEN`

**Descrição.** Implementa a **decisão padrão** nº 1. Migration `29_environment_api_token.sql` (provisória). Formato `dba_<prefixo>_<segredo>` com menos de 128 caracteres, porque `authMiddleware` recusa `Bearer` maior. `authMiddleware` reconhece o prefixo e resolve um principal **token** (ambiente, escopos), distinto de `AuditorUser`. `authorizeRequest` ganha um ramo para token: uma lista fechada de pares (método, rota, escopo), ambiente do caminho igual ao do token, e nenhum acesso a `auth/*`, `environments` global, downloads ou rotas de operator. Escopos iniciais: `snapshot-context:read`, `migration-analysis:write`, `deploy-annotation:write`. A lista é extensível pelos planos 03 (`quality-contract:run`), 08 (`restore-evidence:write`) e 10 (`log-aggregate:write`, condicional). Validade obrigatória (padrão 90 dias, **valor inicial, calibrar**) e `last_used_at`. Todo uso vai para `auditor_operation_log` com o id do token, nunca o segredo. CSRF não se aplica (sem cookie). Rate limit por token. Cartão de tokens na UI (Trinity).

**Critérios de aceite.**
- [ ] O token de A usado em `/environments/B/...` recebe 403, e o log registra "denied" com o id do token (base do RNF-04 da 01, CA-06 da 10 e RNF-04 da 08).
- [ ] Token expirado ou revogado recebe 401. A CLI traduz para código 2 ("credencial inválida ou sem escopo", §4).
- [ ] Escopo ausente para a rota recebe 403.
- [ ] `AUDITOR_REPORT_API_TOKEN` não autentica nenhuma rota nova (teste negativo).
- [ ] O banco guarda só `sha256`. O token aparece uma única vez na resposta de criação e nunca em log.
- [ ] Viewer e auditor não criam token (403). Operator cria.
- [ ] `docs/ops.md` e `docs/hardening.md` atualizados (criação, rotação, revogação, onde guardar no GitHub Secrets).

**Arquivos.** `migrations/29_environment_api_token.sql`, `internal/repository/environment_tokens.go`, `internal/api/auth.go`, `internal/api/environment_tokens.go`, `internal/api/health.go`, `api/openapi.json`, `frontend/src/pages/EnvironmentsPage.tsx`, `frontend/src/services/api.ts`.
**Estimativa:** 5 d. **Gate:** **R2** (autenticação nova e isolamento de tenant). **Depende de:** lote 1.

### Lote 4 — Contexto do banco: snapshot via API (modo a) e conexão direta só leitura (modo b)

**Issue:** `feat(migration-risk): contexto por snapshot via API e por conexão direta só leitura; escalonamento de severidade e duração estimada`

**Descrição.** Rota `migration-context`: por tabela afetada, tamanho total, linhas estimadas, taxa de escrita (delta de `n_tup_ins/upd/del` entre duas coletas completas e comparáveis, regra de `maintenance.growth_trend`), `server_version_num` e execução de referência (id e data), com `partial` e idade. Uma única leitura por tabela (RNF-03). A CLI escala a severidade pelos limiares da §3 (por ambiente, **valor inicial, calibrar**) e estima a duração = tamanho ÷ 50 MB/s, sempre com confiança. Código 3 quando o snapshot é parcial, mais velho que 7 dias (**valor inicial, calibrar**) ou inexistente. Modo (b) `--direct` com DSN em `AUDITOR_MIGRATION_RISK_DSN`/`_FILE`, via `ConnectReadOnly`: allowlist, TLS `verify-full` e recusa de conta com escrita (código 4). Pela **decisão padrão** nº 2, o modo (b) existe só para uso local do operador. A Action do lote 5 não o oferece, e a documentação proíbe DSN no runner de CI.

**Critérios de aceite.**
- [ ] CA-01: índice sem `CONCURRENTLY` numa tabela de 12 GB no snapshot sai MR-01 crítica, com tamanho, lock SHARE e alternativa concorrente, código 1.
- [ ] CA-05: snapshot `partial_success` resulta em código 3 e a saída diz "contexto parcial".
- [ ] CA-06: modo (b) com conta com escrita é recusado com código 4 antes de qualquer leitura (teste com role de escrita no alvo descartável, matriz PG 14–18).
- [ ] O rodapé do Markdown traz o `audit_run_id`, a data e a versão das regras.
- [ ] Token sem `snapshot-context:read` recebe 403 → código 2.
- [ ] A resposta do contexto não traz nada além de metadados das tabelas pedidas (máximo de tabelas por chamada definido e testado).

**Arquivos.** `internal/api/migration_context.go`, `internal/repository/migration_context.go`, `internal/migrationrisk/context.go`, `internal/cli/apiclient.go`, `api/openapi.json`.
**Estimativa:** 5 d. **Gate:** **R2** (nova leitura por token e conexão direta ao alvo). **Depende de:** lotes 2 e 3.

### Lote 5 — GitHub Action e distribuição do binário

**Issue:** `feat(ci): GitHub Action migration-risk com comentário único atualizado por push e release do binário estático`

**Descrição.** Action composite `.github/actions/migration-risk/action.yml`: baixa o binário da release (checksum sha256 verificado), roda `auditor migration-risk` em modo (a) com `AUDITOR_CLI_TOKEN` vindo de `secrets`, e publica **um** comentário no PR com o marcador oculto `<!-- db-auditor:migration-risk -->`. Se o comentário existe, é atualizado. Se não existe, é criado (RF-09). Permissões mínimas: `contents: read`, `pull-requests: write`. A Action não aceita DSN (**decisão padrão** nº 2). Workflow `release-cli.yml`: build `CGO_ENABLED=0` linux/amd64 e arm64, `SHA256SUMS`, gatilho por tag. Um job de autoteste no `ci.yml` roda a CLI sobre fixtures, sem token. A Action pode opcionalmente registrar a anotação `deployment` no merge (escopo `deploy-annotation:write`), mitigando o falso "fora do pipeline" da 04.

**Critérios de aceite.**
- [ ] CA-04: dois pushes seguidos resultam em um único comentário, atualizado (teste com repositório de exemplo ou mock da API do GitHub).
- [ ] O token nunca aparece no log do job (mascarado por `secrets` e nunca impresso pela CLI).
- [ ] Gate padrão: a Action falha o job com código 1 a partir de alta. Os códigos 3 e 4 também falham, com mensagem distinta.
- [ ] Release reproduzível, com checksums publicados. A Action recusa binário com checksum divergente.
- [ ] `docs/migration-risk.md` com exemplo de workflow do repositório da aplicação.

**Arquivos.** `.github/actions/migration-risk/action.yml`, `.github/workflows/release-cli.yml`, `.github/workflows/ci.yml`, `docs/migration-risk.md`.
**Estimativa:** 3 d. **Gate:** **R2** (cadeia de suprimentos, permissão de escrita em PR, segredo no runner). **Depende de:** lote 4.

### Lote 6 — Histórico de análises de migration por ambiente

**Issue:** `feat(migration-risk): envio opcional do resultado ao auditor e lista "Análises de migration" por ambiente (RF-10)`

**Descrição.** Migration `30_migration_analysis.sql` (provisória). `--submit` na CLI envia o JSON (sem SQL nem literais) para `POST .../migration-analyses` com escopo `migration-analysis:write`. O servidor revalida o esquema e recusa campos fora do contrato. Lista e detalhe para viewer. Tela com Trinity. Métricas de análises por resultado e de erros de parse (RNF-05).

**Critérios de aceite.**
- [ ] CA-03 (histórico): o literal de e-mail da migration de teste não está em `migration_analysis_item` (varredura da tabela no teste de integração).
- [ ] RNF-04: o token de A não envia para B (403), e viewer sem concessão a A não lê o histórico de A.
- [ ] RNF-05: `auditor_migration_analyses_total{result}` e `auditor_migration_parse_errors_total` em `/metrics`.
- [ ] Payload acima do limite recebe 413 (`maxAPIBody` de 1 MiB respeitado).
- [ ] Layout da lista aprovado pela Trinity.

**Arquivos.** `migrations/30_migration_analysis.sql`, `internal/repository/migration_analysis.go`, `internal/api/migration_analysis.go`, `internal/observability/metrics.go`, `frontend/src/pages/MigrationAnalysesPage.tsx`, `frontend/src/services/api.ts`.
**Estimativa:** 4 d. **Gate:** **R2** (caminho de escrita por token; garantia de que nenhum dado pessoal de literal persiste). **Depende de:** lotes 3 e 4.

## 5. Estratégia de testes

| Teste | Tipo | Critério ligado |
|---|---|---|
| `TestMR01CriticalWithSnapshotContext` (12 GB) | integração CLI + API fake | CA-01 |
| `TestMR01NoContextMediumLowConfidence` | unitário/golden | CA-02 |
| `TestMigrationOutputHasNoLiterals` (Markdown, JSON e linhas de `migration_analysis_item`) | golden + integração | CA-03, RNF-02 |
| `TestActionUpdatesSingleComment` | teste da Action com mock da API do GitHub | CA-04, RF-09 |
| `TestPartialSnapshotExit3` | integração | CA-05 |
| `TestDirectModeRefusesWriteAccount` | integração no alvo descartável, matriz PG 14–18 | CA-06, RNF-01 |
| `TestInlineSuppressionWithJustification` | unitário | CA-07, RF-08 |
| `TestLockTablePerMajor` (golden por versão 14–18) | unitário + job da matriz | CA-08, RF-03 |
| `TestEnvTokenCrossEnvironmentForbidden`, `TestEnvTokenExpiredRevoked`, `TestReportTokenNotAccepted` | API com `authMiddleware` | RNF-04, decisão padrão nº 1 |
| `TestStaticAnalysisBudget` (200 arquivos / 2 MB < 10 s) | benchmark | RNF-03 |
| `TestDeterministicOutput` | golden | RNF-06 |
| `FuzzMigrationLexer` | fuzz curto | robustez do RF-01 |

## 6. Gates

| Lote | Gate | Motivo |
|---|---|---|
| 1 | **R2** | segredos da CLI |
| 2 | CI verde | análise estática local |
| 3 | **R2** | nova autenticação e isolamento por ambiente |
| 4 | **R2** | leitura por token e conexão ao alvo |
| 5 | **R2** | cadeia de suprimentos e permissões no GitHub |
| 6 | **R2** | escrita por token; ausência de dado pessoal no histórico |

## 7. Riscos

- Snapshot defasado subestima o tamanho. Mitigação: idade explícita, código 3 acima de 7 dias e, com a 04, aviso de mudança desde o snapshot.
- Parser divergente do servidor. Mitigação: subconjunto explícito, "não analisada" no que não for reconhecido, golden por versão.
- Falsa sensação de segurança sem contexto. Mitigação: confiança e motivo explícitos (AN-03).
- O token por ambiente é um novo segredo de longa duração em CI. Mitigação: validade obrigatória, escopo mínimo, revogação e `last_used_at` visível.
- O ramo de token em `authorizeRequest` é uma superfície nova de bypass de RBAC. Mitigação: lista fechada, deny-by-default e testes negativos por rota.
- Sem cgo, um parser completo de SQL fica fora de alcance. O escopo é o subconjunto de DDL da §3.

## 8. Pontos para Q

- A **decisão padrão** nº 2 proíbe DSN no CI, mas o RF-04(b) e o CA-06 exigem o modo (b). O plano mantém o modo (b) só para uso local. Confirmar se ele deve existir no v1 ou sair do binário.
- O CA-08 cita "PostgreSQL 14 e 16". O plano testa as cinco versões. A diferença de lock mais clara dentro da matriz é a do PG 18 (`NOT NULL ... NOT VALID`). Confirmar a operação de referência com o PO.
- A P-03 (SARIF) não foi decidida e fica fora do v1.

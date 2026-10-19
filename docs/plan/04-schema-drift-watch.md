# Plano 04 — Linha do tempo e alerta de drift de schema (`schema-drift-watch`)

- **Spec:** [../spec/04-schema-drift-watch.md](../spec/04-schema-drift-watch.md)
- **Base lida:** `main` em `b1a93cf`
- **Posição na ordem de implementação:** 2ª
- **Total:** 6 lotes, **23 d**, 3 lotes com gate R2, Trinity necessária (2 lotes)

## 0. Resumo

Fecha a lacuna verificada: `frontend/src/pages/SchemaDriftPage.tsx` ainda envia `demoSource`/`demoTarget` fixos para `POST /api/v1/compare` (botão "Comparar (demo payloads)"). O plano cria um diff real entre execuções, lido do snapshot store, e uma linha do tempo gravada na ingestão, no fim de cada coleta completa. Cada evento é correlacionado com anotações `deployment` (`audit_annotation`) e com o registro de migrations da ferramenta da aplicação. Os alertas saem pelos canais globais, em formato genérico ou nativo do Slack.

### Decisões padrão aplicadas

- **decisão padrão (Q, nº 5):** ler as **tabelas de histórico das ferramentas de migration conhecidas** (golang-migrate, goose, Flyway, Alembic, Rails, Prisma: as seis da P-01) e, **opcionalmente, as anotações de implantação**. Responde à P-01.
- **decisão padrão (Q, nº 5):** **webhook genérico** (JSON atual de `notify.Event`) **mais formato nativo do Slack**. Responde à P-02.
- **decisão padrão (Q, nº 6, por extensão):** os alertas de drift usam os **canais globais atuais** (`internal/notify`). Não há canal por ambiente.
- **decisão padrão (Q, nº 11):** tenant = ambiente. O diff entre ambientes exige concessão aos dois lados (RNF-04).

### Dependências

Nenhuma bloqueante. Este plano é base opcional para a 01 (mostrar se a tabela mudou desde o snapshot) e para a 07 (quando o privilégio apareceu). A anotação automática de deploy pela CLI, mitigação de risco desta spec, depende do token por ambiente da 01 (lote 3 do plano 01, escopo `deploy-annotation:write`).

## 1. Módulos e arquivos afetados

| Área | Caminho | Mudança |
|---|---|---|
| Montagem do diff | `backend/internal/schemadiff/` (novo: `build.go`, `objects.go`, `timeline.go`) | converte os snapshots em `comparison.ObjectItem` e aplica `comparison.CompareSets`/`diffFields` |
| Comparação | `backend/internal/comparison/compare.go` | sem mudança de semântica; no máximo novos tipos de objeto em `KeyFor` |
| Leitura de snapshots | `backend/internal/repository/schema_diff.go`, `schema_timeline.go` (novos), `structural_snapshots.go`, `object_snapshots.go`, `extended_object_snapshots.go`, `timescale_policy_snapshots.go`, `object_mapping.go` | leitura por execução, paginada |
| Ingestão | `backend/internal/audit/runner.go` (interface opcional `SchemaTimelineRecorder`, chamada junto de `ReconcileCompletedRun` em `repository/baseline.go`) | eventos gravados na ingestão (RNF-03) |
| Regras | `backend/internal/analyzer/schema_drift.go` (novo), `analyzer/registry.go`, `backend/internal/actions/catalog.go`, `actions/text_pt.go` | `drift.out_of_pipeline`, `drift.environment_divergence`. Os `config.*_drift` de `analyzer/drift.go` ficam como estão |
| Registro de migrations | `backend/internal/collectors/postgres/migration_registry.go` (novo), registro em `backend/internal/audit/live_registry_postgres.go` | leitura só leitura de versão e carimbo |
| Alertas | `backend/internal/notify/notify.go` (formato Slack), `backend/internal/repository/alerts.go` (`ClaimAlert`) | — |
| API | `backend/internal/api/schema_drift.go` (novo), `api/health.go`, `api/auth.go` (só se preciso, ver lote 1) | — |
| Frontend | `frontend/src/pages/SchemaDriftPage.tsx`, `frontend/src/pages/SchemaTimelinePage.tsx` (novo) ou aba, `frontend/src/pages/EnvironmentsPage.tsx` (atributo produção), `frontend/src/services/api.ts`, `frontend/src/lib/export.ts` | — |
| Docs | `docs/rules.md` (seção "Drift de schema": timeline × regressão `structure`), `docs/ops.md` (registro de migrations, Slack), `docs/guia-do-usuario.md` | — |

## 2. Migrations (provisórias)

| Nº provisório | Arquivo | Lote | Conteúdo |
|---|---|---|---|
| 27 | `backend/migrations/27_schema_timeline.sql` | 3 | `audit_environment.is_production boolean NOT NULL DEFAULT false`; `schema_change_event` (ambiente, execução anterior e atual, intervalo, tipo de objeto, objeto qualificado, tipo de mudança, campos alterados com fingerprints e **sem definição SQL**, correlação `deployment`/`migration_registry`/`none`/`not_verified`, id da anotação, versão do registro); `schema_timeline_gap` (intervalos com coleta parcial ou incomparável); índice `(environment_id, detected_at DESC, id)` |
| 28 | `backend/migrations/28_migration_registry.sql` | 4 | `environment_migration_registry` (ambiente, banco, ferramenta com CHECK nas seis da **decisão padrão**, schema e tabela opcionais, ligado); `migration_registry_snapshot` (execução, ferramenta, versão mais recente, `applied_at` nulo quando a ferramenta não grava, estado de acesso) |

Os números dependem do merge do plano 06 (26). Cada migration entra em `.github/workflows/ci.yml`, `backend/migrations/README.md` e `docs/ops.md`.

## 3. Endpoints e telas

| Método e rota | Papel | Observação |
|---|---|---|
| `GET /api/v1/environments/{id}/schema-diff?source_run=&target_environment_id=&target_run=&object_type=&status=&limit=&offset=` | viewer com acesso aos dois lados | `authorizeRequest` já recusa um `target_environment_id` sem concessão (laço de `source_environment_id`/`target_environment_id`). O lote 1 prova isso com teste |
| `GET /api/v1/environments/{id}/schema-diff/export?format=csv\|md&...` | viewer com acesso aos dois lados | CSV com `# export_complete` no fim (padrão de `actions/export`) |
| `GET /api/v1/environments/{id}/schema-timeline?from=&to=&limit=&offset=` | viewer | janela padrão de 90 dias |
| `GET /api/v1/environments/{id}/schema-timeline/export?format=csv\|md` | viewer | — |
| `PATCH /api/v1/environments/{id}` (`is_production`) | operator | regra padrão de não-GET |
| `GET`/`PUT /api/v1/environments/{id}/migration-registry` | viewer / operator | configuração do RF-05(b) |

**Telas (Trinity necessária).** (1) **Desvio de schema** com seletores reais de ambiente e execução (padrão: última completa de cada lado), estados igual/diferente/só origem/só destino, campos alterados, fingerprints antigo e novo, aviso "parcial" e link para o Inventário. (2) Nova aba **Linha do tempo** por ambiente, com evento, execução, correlação, selo "fora do pipeline" e lacunas. (3) Chave "ambiente de produção" em Ambientes.

## 4. Lotes

### Lote 1 — Diff real entre execuções (backend)

**Issue:** `feat(schema-drift): diff entre execuções reais do mesmo ambiente ou de ambientes diferentes (spec 04 RF-02, RF-03)`

**Descrição.** Montar os conjuntos de objetos a partir de `table_snapshot`, `column_snapshot` (tipo, nulidade, fingerprint do default), `index_snapshot`, `constraint_snapshot`, `view_snapshot`, `function_snapshot` (assinatura e fingerprint do corpo), `trigger_snapshot`, `sequence_snapshot`, `hypertable_snapshot` e `policy_snapshot`, com `object_mapping` pareando nomes diferentes. Comparar com `comparison.CompareSets`. Uma execução parcial marca o diff como "parcial" e não gera achado. Mecanismos diferentes (PostgreSQL × MongoDB) respondem 400. A resposta é paginada.

**Critérios de aceite.**
- [ ] CA-01 (backend): o resultado vem dos snapshots das execuções escolhidas. O padrão é a última completa de cada lado.
- [ ] CA-05: função com corpo alterado mostra fingerprint antigo e novo, nunca o corpo.
- [ ] CA-06: viewer com acesso só a A recebe 403 no diff A × B, e a resposta não revela a existência de objetos de B.
- [ ] RNF-03: 5000 tabelas e 50 000 colunas em menos de 5 s (teste de escala e orçamento em `scale_budget_integration_test.go`).
- [ ] 400 para PostgreSQL × MongoDB. Execução parcial resulta em `partial: true`.
- [ ] `openapi.json` atualizado.

**Arquivos.** `internal/schemadiff/*`, `internal/repository/schema_diff.go`, `internal/api/schema_drift.go`, `internal/api/health.go`, `api/openapi.json`.
**Estimativa:** 4 d. **Gate:** **R2** (isolamento de tenant: leitura cruzada entre ambientes). **Depende de:** —

### Lote 2 — Tela Desvio de schema com dados reais

**Issue:** `feat(ui): Desvio de schema com execuções reais e remoção dos dados de demonstração (spec 04 RF-01)`

**Descrição.** Remover `demoSource`, `demoTarget` e o botão "Comparar (demo payloads)" de `SchemaDriftPage.tsx`. Adicionar seletores de ambiente e execução, filtros de tipo e estado, campos alterados por objeto, aviso de parcial e link para o Inventário (`lib/inventoryTarget.ts`). O layout é desenhado com a Trinity.

**Critérios de aceite.**
- [ ] CA-01: nenhum dado de demonstração aparece. Um teste de componente garante que a tela chama `schema-diff` e não `POST /compare` com payload fixo.
- [ ] CA-05 na UI: só fingerprints.
- [ ] CA-06 na UI: 403 gera mensagem genérica, sem detalhes de B.
- [ ] Estados vazio, parcial e mecanismos diferentes aprovados pela Trinity.

**Arquivos.** `frontend/src/pages/SchemaDriftPage.tsx`, `frontend/src/services/api.ts`, `frontend/src/types/openapi.ts`, `docs/guia-do-usuario.md`.
**Estimativa:** 3 d. **Gate:** CI verde + Trinity. **Depende de:** lote 1.

### Lote 3 — Linha do tempo gravada na ingestão e regras `drift.*`

**Issue:** `feat(schema-drift): eventos de timeline por coleta completa, correlação com anotação de deploy e achados drift.* (spec 04 RF-04–RF-06)`

**Descrição.** Migration `27_schema_timeline.sql` (provisória). Ao fim de uma coleta completa (`success` e análise concluída), comparar com a coleta completa anterior do mesmo perfil e da mesma versão de coletor, gravando eventos por objeto. Uma coleta parcial no meio grava lacuna, não evento (RNF-06, AN-11). Correlacionar cada evento com `audit_annotation` `kind='deployment'` no intervalo ±30 min (**valor inicial, calibrar**, parâmetro de `rule_policy`). Achados `drift.out_of_pipeline`, com severidades da §3 rebaixadas um nível sem `is_production`, e `drift.environment_divergence` (pares definidos em `object_mapping` ou na configuração de par, persistente em duas completas de cada lado). As entradas novas de catálogo têm os cinco campos e consulta segura (`TestEveryCatalogRuleHasFiveFieldsAndASafeQuery`). Rota de timeline e `PATCH` do atributo produção.

**Critérios de aceite.**
- [ ] CA-02: índice criado entre duas completas sem anotação gera evento "fora do pipeline" e `drift.out_of_pipeline` média.
- [ ] CA-03: com anotação `deployment` no intervalo, o evento aparece correlacionado e não gera achado.
- [ ] CA-04: coleta parcial entre duas completas vira lacuna, sem eventos.
- [ ] `docs/rules.md` explica a diferença: a regressão `structure` compara com o baseline, a timeline compara coleta a coleta.
- [ ] RNF-03: o evento é gravado na ingestão. A leitura não recalcula nada (teste confere que `GET` não executa diff).
- [ ] RNF-05: `auditor_schema_change_events_total{kind}`.

**Arquivos.** `migrations/27_schema_timeline.sql`, `internal/schemadiff/timeline.go`, `internal/repository/schema_timeline.go`, `internal/repository/baseline.go`, `internal/audit/runner.go`, `internal/analyzer/schema_drift.go`, `internal/actions/catalog.go`, `internal/api/schema_drift.go`, `docs/rules.md`.
**Estimativa:** 5 d. **Gate:** CI verde. **Depende de:** lote 1.

### Lote 4 — Registro de migrations da ferramenta da aplicação

**Issue:** `feat(schema-drift): leitura opcional do registro de migrations (golang-migrate, goose, Flyway, Alembic, Rails, Prisma) para correlação (spec 04 RF-05b)`

**Descrição.** Migration `28_migration_registry.sql` (provisória). Implementa a **decisão padrão**: o operator configura por ambiente a ferramenta e, opcionalmente, schema e tabela. Um novo coletor, pela conexão só leitura (`collectors/postgres/readonly_connection.go`), lê **só** a versão mais recente e o carimbo de aplicação:

| Ferramenta | Tabela padrão | Versão | Carimbo |
|---|---|---|---|
| golang-migrate | `schema_migrations` | `version` (+ `dirty`) | **não existe** |
| goose | `goose_db_version` | `version_id` | `tstamp` |
| Flyway | `flyway_schema_history` | `version` | `installed_on` |
| Alembic | `alembic_version` | `version_num` | **não existe** |
| Rails | `schema_migrations` | `version` | **não existe** (o nome da versão costuma ser um timestamp, mas não é carimbo de aplicação) |
| Prisma | `_prisma_migrations` | `migration_name` | `finished_at` |

Sem carimbo, a correlação é "a versão mudou entre as duas coletas", com confiança menor. Os identificadores de schema e tabela são validados (regex de identificador e `pgx.Identifier.Sanitize`), nunca interpolados crus. Se a tabela estiver configurada mas inacessível, a correlação fica "não verificada" e a cobertura parcial (§4 Erros). Nada além de versão e carimbo é lido (RNF-02). As anotações de implantação continuam valendo em conjunto (**decisão padrão**: anotação opcional).

**Critérios de aceite.**
- [ ] Teste de integração por ferramenta, com tabelas criadas num alvo descartável, nos jobs PG 14–18.
- [ ] Evento correlacionado por versão quando a ferramenta não tem carimbo, com confiança registrada.
- [ ] Tabela inexistente ou sem permissão resulta em "não verificada" e cobertura parcial, sem falhar a coleta.
- [ ] Identificador malicioso na configuração (`x"; drop ...`) é recusado com 400.
- [ ] O coletor respeita os timeouts de catálogo e `AUDITOR_TARGET_ALLOWED_HOSTS`.

**Arquivos.** `migrations/28_migration_registry.sql`, `internal/collectors/postgres/migration_registry.go`, `internal/audit/live_registry_postgres.go`, `internal/repository/schema_timeline.go`, `internal/api/schema_drift.go`, `docs/ops.md`.
**Estimativa:** 4 d. **Gate:** **R2** (nova leitura no banco auditado a partir de identificadores configuráveis). **Depende de:** lote 3.

### Lote 5 — Alertas: webhook genérico, Slack nativo e e-mail

**Issue:** `feat(notify): alerta drift.out_of_pipeline com payload sanitizado, formato Slack e métrica de falha (spec 04 RF-07)`

**Descrição.** Um evento por episódio, com dedup pelo `ClaimAlert`/`alert_delivery` (chave = ambiente + objeto + tipo de mudança + execução do episódio). O payload leva tipo, ambiente, objeto qualificado, tipo de mudança e link, sem definição SQL. Novo `AUDITOR_ALERT_WEBHOOK_FORMAT=generic|slack` (padrão `generic`). O formato `slack` monta `text`/`blocks` a partir do mesmo `notify.Event`. Criar métrica de falha `auditor_alert_deliveries_failed_total{kind}` (RNF-05: hoje `alert_delivery` só registra entregas).

**Critérios de aceite.**
- [ ] CA-07: um único POST com payload sanitizado. A repetição na coleta seguinte não reenvia.
- [ ] Golden do payload genérico e do Slack, ambos sem SQL, DSN ou senha.
- [ ] Falha de entrega incrementa a métrica e não falha a coleta.
- [ ] O formato Slack vale para todos os eventos já existentes sem quebrar o JSON genérico (os testes atuais de `notify` continuam verdes).

**Arquivos.** `internal/notify/notify.go`, `internal/repository/alerts.go`, `internal/observability/metrics.go`, `.env.example`, `deploy/env.prod.example`, `docs/ops.md`.
**Estimativa:** 3 d. **Gate:** **R2** (saída de dados para um serviço externo; sanitização do payload). **Depende de:** lote 3.

### Lote 6 — UI Linha do tempo, atributo produção e exportações

**Issue:** `feat(ui): aba Linha do tempo, marcação de produção e export Markdown/CSV do diff e da timeline (spec 04 RF-04, RF-08)`

**Descrição.** Nova aba **Linha do tempo**, desenhada com a Trinity: eventos com execução, correlação (anotação ou migration com versão), selo "fora do pipeline", lacunas e link para o objeto no Inventário. Chave "produção" em Ambientes (operator). Rotas de exportação CSV/Markdown do diff e da timeline, com CSV terminando em `# export_complete`.

**Critérios de aceite.**
- [ ] CA-02, CA-03 e CA-04 visíveis na UI (evento fora do pipeline, correlacionado, lacuna).
- [ ] RF-08: CSV termina no marcador de exportação completa e não contém definição SQL.
- [ ] Viewer não vê a chave de produção. Operator altera e o registro vai para `auditor_operation_log`.
- [ ] Layout aprovado pela Trinity.

**Arquivos.** `frontend/src/pages/SchemaTimelinePage.tsx`, `frontend/src/pages/EnvironmentsPage.tsx`, `frontend/src/pages/parts/useEnvironmentsPage.tsx`, `frontend/src/services/api.ts`, `backend/internal/api/schema_drift.go`, `docs/guia-do-usuario.md`.
**Estimativa:** 4 d. **Gate:** CI verde + Trinity. **Depende de:** lotes 2 e 3.

## 5. Estratégia de testes

| Teste | Tipo | Critério ligado |
|---|---|---|
| `TestSchemaDriftPageUsesRealRuns` (sem `demoSource`) | componente (Bun) + API | CA-01 |
| `TestSchemaDiffFromSnapshots` (execuções completas de dois ambientes) | integração | CA-01, RF-02, RF-03 |
| `TestTimelineManualIndexOutOfPipeline` | integração (duas coletas sintéticas) | CA-02 |
| `TestTimelineDeploymentAnnotationCorrelates` (±30 min) | integração | CA-03 |
| `TestTimelinePartialRunIsGap` | integração | CA-04, RNF-06 |
| `TestSchemaDiffFunctionShowsFingerprintsOnly` | unitário + API | CA-05, RNF-02 |
| `TestSchemaDiffCrossEnvironmentRequiresBothGrants` (viewer A → A × B = 403, corpo sem objetos) | API com `authMiddleware` | CA-06, RNF-04 |
| `TestDriftWebhookOnceAndSanitized` (servidor falso conta POSTs; Slack e genérico) | unitário/integração | CA-07, RF-07 |
| `TestMigrationRegistryPerTool` (6 ferramentas) | integração no alvo descartável, matriz PG 14–18 | RF-05(b) |
| `TestSchemaDiffScaleBudget` (5000/50 000 < 5 s) | escala | RNF-03 |
| `TestEveryCatalogRuleHasFiveFieldsAndASafeQuery` (existente) | unitário | regras `drift.*` |

## 6. Gates

| Lote | Gate | Motivo |
|---|---|---|
| 1 | **R2** | leitura entre ambientes (isolamento de tenant) |
| 2 | CI verde + Trinity | UI |
| 3 | CI verde | eventos e regras sobre o store |
| 4 | **R2** | nova leitura no alvo com identificadores configuráveis |
| 5 | **R2** | dados saem para webhook, Slack ou e-mail |
| 6 | CI verde + Trinity | UI e exportação de metadados |

## 7. Riscos

- Intervalo longo entre coletas mascara mudança revertida. Mitigação: o guia recomenda cadência e mostra o intervalo do evento.
- Anotação esquecida gera falso "fora do pipeline". Mitigação: registro de migrations (lote 4) e anotação automática pela CLI do plano 01 (escopo `deploy-annotation:write`).
- golang-migrate, Alembic e Rails não guardam carimbo. A correlação por versão é mais fraca, e a confiança fica rebaixada.
- Sobreposição com a regressão `structure`: textos e `docs/rules.md` deixam a diferença explícita.
- O formato Slack global afeta todos os alertas existentes. Ele só é ativado por variável explícita.

## 8. Pontos para Q

- A **decisão padrão** nº 5 fala em "ler as tabelas de histórico ... e, opcionalmente, anotações de implantação", invertendo a ordem do RF-05 (anotação = (a), registro = (b) opcional). O plano trata as duas fontes como válidas: a anotação vale sempre que existir, e o registro vale quando configurado. Confirmar se o registro deve ser obrigatório para marcar "fora do pipeline".
- Para as ferramentas sem carimbo de aplicação (golang-migrate, Alembic, Rails), basta correlacionar por "versão mudou entre as coletas"?
- O par "produção × homologação" do RF-06 não tem entidade hoje. O plano usa o atributo `is_production` e um par configurado por `object_mapping`. Confirmar se é preciso uma entidade de par de ambientes.

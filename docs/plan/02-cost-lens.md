# Plano 02 — Custo por tabela, índice e consulta (`cost-lens`)

- **Spec:** [../spec/02-cost-lens.md](../spec/02-cost-lens.md)
- **Base lida:** `main` em `b1a93cf`
- **Posição na ordem de implementação:** 4ª
- **Decisões:** [ADR-0001](../adr/0001-decisoes-v1-features.md) (Aceita, 2026-10-18)
- **Total:** 6 lotes, **21 d**, 0 lote com gate R2, Trinity necessária (1 lote)

## 0. Resumo

O plano traduz tamanhos, índices sem uso, bloat estimado e workload em custo mensal estimado. Tudo é calculado sobre o snapshot store, sem nova sessão no alvo (RNF-01, RNF-03), a partir de uma tabela de preços versionada por ambiente. O custo por consulta reaproveita o motor de janela da spec 06 (`internal/digest`).

### Decisões aplicadas (ADR-0001)

- **ADR-0001 D3:** **uma moeda por ambiente** (`BRL` ou `USD`), com preço configurável. A moeda fica **travada depois da primeira versão** da tabela de preços do ambiente, e não existe câmbio. Responde à P-01.
- **ADR-0001 D3:** **bloat só por estimativa**, sem extensão de medição física no alvo. **O bloat exato fica para depois.** Responde à P-02.
- **ADR-0001 D3:** modelos iniciais só RDS PostgreSQL, Cloud SQL e self-hosted. Responde à P-03.
- **ADR-0001 D11:** tenant = ambiente. Preços, ranking e série pertencem ao ambiente.

### Dependências

O lote 3 depende do lote 1 do plano 06 (deltas por janela). Sem ele, o RF-05 fica de fora, e os demais lotes entregam (§6 da spec). É base opcional da 09 (tabela de preços).

## 1. Módulos e arquivos afetados

| Área | Caminho | Mudança |
|---|---|---|
| Modelo de custo | `backend/internal/cost/` (novo: `prices.go`, `templates.go` com RDS PostgreSQL, Cloud SQL e self-hosted editáveis, `storage.go`, `bloat.go`, `query.go`, `ranking.go`) | — |
| Impacto e prioridade | `backend/internal/impact/impact.go`, `impact/gain.go`, `backend/internal/priority/priority.go` | `impact_estimate` em moeda (valor, unidade `BRL/mês` ou `USD/mês`, denominador, período, AN-03) |
| Regras | `backend/internal/analyzer/cost.go` (novo), `analyzer/registry.go`, `backend/internal/actions/catalog.go`, `actions/text_pt.go` | `cost.unused_index_waste`, `cost.bloat_recoverable`, `cost.query_expensive` |
| Store | `backend/internal/repository/cost.go` (novo), `storage_footprint.go`, `analytics_aggregate.go` (série com lacunas, padrão da série `analytics/storage`) | — |
| Janela de consulta | `backend/internal/digest/window.go` (plano 06) | reuso |
| API | `backend/internal/api/cost.go` (novo), `api/health.go`, `api/openapi.json` | — |
| PDF | `backend/internal/report/sections.go`, `report/model.go`, `report/redaction.go`, `report/testdata/*.golden.txt`, `backend/internal/repository/report_document.go` | seção "Custo estimado" |
| Frontend | `frontend/src/pages/DashboardPage.tsx` + `pages/parts/DashboardCost.tsx` (novo), `pages/parts/InventoryTablesTable.tsx` (coluna de custo), `pages/parts/CostPricesCard.tsx` (novo) em Ambientes, `services/api.ts` | — |
| Docs | `docs/rules.md` (custo e bloat estimado × `maintenance.dead_tuple_pressure`), `docs/guia-do-usuario.md`, `docs/ops.md` | — |

## 2. Migrations (provisórias)

| Nº provisório | Arquivo | Lote | Conteúdo |
|---|---|---|---|
| 31 | `backend/migrations/31_cost_price_table.sql` | 1 | `cost_price_version`: ambiente, versão, `currency` (CHECK `BRL`/`USD`; igual em todas as versões do ambiente, garantido por trigger ou validação na escrita), preço de armazenamento por GB/mês, I/O por milhão de requisições **ou** por GB lido, computação por vCPU-hora **ou** custo mensal da instância, modelo de origem, `effective_from`, `created_by`, `created_at`. Todos `numeric >= 0`, `UNIQUE (environment_id, version)` |
| 32 | `backend/migrations/32_cost_estimate_series.sql` | 4 | `cost_estimate_snapshot`: ambiente, `audit_run_id` (só completas), versão de preço usada, totais por tipo (armazenamento, índice sem uso, bloat estimado, consulta), moeda, confiança. Alimenta a série do RF-09 sem recalcular 90 dias na leitura |

Os números supõem 26–30 já mergeados. Lembrar `ci.yml`, `migrations/README.md` e `docs/ops.md`.

## 3. Endpoints e telas

| Método e rota | Papel | Observação |
|---|---|---|
| `GET /api/v1/environments/{id}/cost/prices` | viewer | versão vigente e histórico |
| `PUT /api/v1/environments/{id}/cost/prices` | operator | cria uma nova versão. 400 para valor negativo ou moeda inválida/diferente. Registro em `auditor_operation_log` (já feito pelo `authMiddleware` em não-GET) |
| `GET /api/v1/environments/{id}/cost/templates` | viewer | modelos iniciais editáveis |
| `GET /api/v1/environments/{id}/cost/ranking?run=&database=&schema=&type=` | viewer | top 10 desperdícios e itens por tipo |
| `GET /api/v1/environments/{id}/cost/series?from=&to=` | viewer | teto de 90 dias, com lacunas |
| `GET /api/v1/environments/{id}/cost/ranking/export?format=csv` | viewer | uma linha por item, com moeda e vigência, terminando em `# export_complete` |

**Telas (Trinity necessária).** Aba **Custo** no Dashboard (ranking com tipo, objeto, valor/mês, base de cálculo, confiança, link para o achado; série; estados "não estimado — preço não configurado" e "janela inválida"). Coluna de custo no Inventário. Edição da tabela de preços em Ambientes (operator). Rótulos "estimado/mês" e "estimativa" em todo valor (RNF-06). A posição na navegação citada aqui é a da spec. A decisão final fica adiada para o lote de tela, quando Q chamará a Trinity (ADR-0001, Pendente).

## 4. Lotes

### Lote 1 — Tabela de preços versionada por ambiente

**Issue:** `feat(cost): tabela de preços por ambiente, versionada, uma moeda por ambiente, com modelos RDS/Cloud SQL/self-hosted`

**Descrição.** Migration `31_cost_price_table.sql` (provisória). Validação: números ≥ 0, moeda `BRL` ou `USD` e igual à da primeira versão (ADR-0001 D3), vigência obrigatória. Os modelos iniciais são presets editáveis, **sem** busca online de preço (fora de escopo). Leitura para viewer, escrita para operator.

**Critérios de aceite.**
- [ ] CA-05: viewer e auditor recebem 403 no `PUT`. Operator grava uma nova versão.
- [ ] 400 para preço negativo, moeda inválida ou moeda diferente da do ambiente.
- [ ] RNF-04/RNF-05: a alteração aparece em `auditor_operation_log`, com quem e quando.
- [ ] Versões são imutáveis: não há `UPDATE` de uma versão existente.

**Arquivos.** `migrations/31_cost_price_table.sql`, `internal/cost/prices.go`, `internal/cost/templates.go`, `internal/repository/cost.go`, `internal/api/cost.go`, `api/openapi.json`.
**Estimativa:** 3 d. **Gate:** CI verde (RBAC padrão de não-GET, sem mudança em `authorizeRequest`). **Depende de:** —

### Lote 2 — Custo de armazenamento, índice sem uso e bloat estimado

**Issue:** `feat(cost): custo de armazenamento, desperdício de índice sem uso e bloat recuperável estimado (RF-02–RF-04, RF-07)`

**Descrição.** Armazenamento = `total_size_bytes` do último snapshot completo × preço por GB/mês (RF-02). Desperdício de índice = índices com `index.unused` aberto × tamanho × preço, respeitando a janela de estatísticas ≥ 14 dias sem reset (RF-03). Bloat estimado por método estatístico sobre catálogo e estatísticas, sem ler linhas: largura média de `column_stat_snapshot.average_width` e `null_fraction`, mais linhas estimadas e tamanho de `table_snapshot`. É **sempre** rotulado "estimativa", com confiança no máximo média (ADR-0001 D3: só estimativa). As regras `cost.unused_index_waste` e `cost.bloat_recoverable` (limiares da §3, **valor inicial, calibrar**) entram em `analyzer/cost.go`, com textos e consulta de confirmação em `actions/catalog.go`. Nunca recomendam `DROP` automático. Sem preço, o valor é "não estimado — preço não configurado", e bytes e tempo continuam visíveis.

**Critérios de aceite.**
- [ ] CA-01: índice de 2 GB com `index.unused` aberto e R$ 0,50/GB-mês resulta em R$ 1,00/mês, com fórmula e vigência.
- [ ] CA-04: tabela com bloat estimado de 35% e 3 GB gera `cost.bloat_recoverable` média, rotulada "estimativa".
- [ ] CA-02 (parte de regra): sem preço, os achados ficam com impacto "não estimado" e motivo.
- [ ] RNF-06: todo valor guarda fórmula, preço usado (versão e vigência), denominador e período em `evidence.impact_basis`, campo que não entra na assinatura de descarte (padrão AN-03).
- [ ] `docs/rules.md` deixa claro que o bloat estimado não é `maintenance.dead_tuple_pressure`.
- [ ] `TestEveryCatalogRuleHasFiveFieldsAndASafeQuery` verde.

**Arquivos.** `internal/cost/storage.go`, `internal/cost/bloat.go`, `internal/analyzer/cost.go`, `internal/analyzer/registry.go`, `internal/actions/catalog.go`, `internal/actions/text_pt.go`, `internal/impact/impact.go`, `docs/rules.md`.
**Estimativa:** 5 d. **Gate:** CI verde. **Depende de:** lote 1.

### Lote 3 — Custo de consulta com os deltas da spec 06

**Issue:** `feat(cost): custo por fingerprint na janela entre coletas completas e regra cost.query_expensive (RF-05)`

**Descrição.** Usa `internal/digest` (plano 06, lote 1) para o delta entre duas coletas completas: participação no tempo total e em blocos lidos × custo de computação e I/O. Janela com `stats_reset` diferente é inválida: nenhum `cost.query_expensive` é criado e o motivo aparece. Só fingerprints normalizados, nunca texto SQL (RNF-02).

**Critérios de aceite.**
- [ ] CA-03: duas coletas com `stats_reset` diferente não geram `cost.query_expensive` e expõem o motivo "janela inválida".
- [ ] Regra com ≥ 10% do tempo total e ≥ 1000 chamadas resulta em baixa, e ≥ 25% em média (**valor inicial, calibrar**).
- [ ] Coleta parcial em qualquer lado: não calcula delta (§3).

**Arquivos.** `internal/cost/query.go`, `internal/analyzer/cost.go`, `internal/actions/catalog.go`.
**Estimativa:** 3 d. **Gate:** CI verde. **Depende de:** lote 2 e plano 06 lote 1.

### Lote 4 — Ranking, série e exportação CSV

**Issue:** `feat(cost): ranking top 10 desperdícios, série por coleta completa com lacunas e export CSV (RF-06, RF-08, RF-09)`

**Descrição.** Migration `32_cost_estimate_series.sql` (provisória). Gravar o total por tipo no fim da análise de cada coleta **completa** (nunca de parcial: lacuna). Ranking que combina os três tipos, com valor mensal, base de cálculo, confiança e próximo passo. CSV com moeda e vigência, terminando em `# export_complete`, sem texto SQL.

**Critérios de aceite.**
- [ ] CA-02: ambiente sem preço mostra itens com bytes e tempo e valor "não estimado — preço não configurado".
- [ ] CA-07: o CSV termina com o marcador de exportação completa, e nenhuma célula contém texto SQL (varredura no teste).
- [ ] RNF-03: ranking com 5000 tabelas em menos de 2 s (teste de escala).
- [ ] RF-09: a série respeita os 90 dias e mostra lacuna em coleta parcial (mesma regra de `analytics/storage`, `frontend/src/lib/trend-gaps.ts`).
- [ ] RNF-05: `auditor_cost_calculations_total{priced}`.
- [ ] Ambiente sem coleta completa devolve lista vazia com motivo.

**Arquivos.** `migrations/32_cost_estimate_series.sql`, `internal/cost/ranking.go`, `internal/repository/cost.go`, `internal/api/cost.go`, `internal/observability/metrics.go`, `api/openapi.json`.
**Estimativa:** 3 d. **Gate:** CI verde. **Depende de:** lotes 2 e 3 (o 3 é opcional; sem ele o ranking sai sem o tipo consulta).

### Lote 5 — Aba Custo, coluna no Inventário e edição de preços

**Issue:** `feat(ui): aba Custo no Dashboard, coluna de custo no Inventário e edição da tabela de preços (spec 02 §4 UI)`

**Descrição.** Telas desenhadas com a Trinity. Ranking com tipo, objeto, valor/mês, base, confiança e link para o achado. Série com lacunas. Faixa de confiança e rótulos "estimado". Formulário de preços para operator, com o modelo inicial como ponto de partida e moeda bloqueada depois da primeira versão.

**Critérios de aceite.**
- [ ] CA-01, CA-02 e CA-03 visíveis na UI (fórmula e vigência, "não estimado", "janela inválida").
- [ ] CA-05 na UI: viewer não vê o formulário.
- [ ] A coluna de custo no Inventário não degrada a virtualização (`docs/ui-virtualization.md`; medição com `scripts/measure-table-heap.mjs`).
- [ ] Layout aprovado pela Trinity, com contraste (`styles.contrast.test.ts`) e acessibilidade dos gráficos (padrão #122).

**Arquivos.** `frontend/src/pages/DashboardPage.tsx`, `frontend/src/pages/parts/DashboardCost.tsx`, `frontend/src/pages/parts/InventoryTablesTable.tsx`, `frontend/src/pages/parts/CostPricesCard.tsx`, `frontend/src/pages/EnvironmentsPage.tsx`, `frontend/src/services/api.ts`, `docs/guia-do-usuario.md`.
**Estimativa:** 4 d. **Gate:** CI verde + Trinity. **Depende de:** lote 4.

### Lote 6 — Seção "Custo estimado" no PDF técnico e executivo

**Issue:** `feat(report): seção Custo estimado nos PDFs técnico e executivo, com redação (RF-08)`

**Descrição.** Nova seção em `internal/report/sections.go`, com a separação hipótese / ganho previsto / ganho medido (BA-21). Na redação estrita sai só o total por tipo, sem nomes de objetos. Golden files novos ou atualizados (`go test ./internal/report -run Golden -update`, revisado no PR). Respeitar os limites do BA-11 (páginas, bytes, tempo).

**Critérios de aceite.**
- [ ] CA-06: PDF com redação estrita não lista nomes de objetos na seção de custo (golden + busca textual no PDF gerado).
- [ ] Sem preço, a seção diz "não estimado — preço não configurado".
- [ ] O teste de relatório sob limite de memória do CI (PF-07) continua verde.

**Arquivos.** `internal/report/sections.go`, `internal/report/model.go`, `internal/report/redaction.go`, `internal/report/testdata/*`, `internal/repository/report_document.go`, `docs/rules.md` (seção Relatórios PDF).
**Estimativa:** 3 d. **Gate:** CI verde. **Depende de:** lote 4.

## 5. Estratégia de testes

| Teste | Tipo | Critério ligado |
|---|---|---|
| `TestUnusedIndexWasteCost` (2 GB × R$ 0,50 = R$ 1,00) | unitário `internal/cost` + integração | CA-01 |
| `TestRankingWithoutPrices` | API | CA-02 |
| `TestQueryCostInvalidWindowOnStatsReset` | integração (reuso de `internal/digest`) | CA-03 |
| `TestBloatEstimateRule` (35%, 3 GB → média, "estimativa") | unitário | CA-04 |
| `TestPriceTableRequiresOperator` | API com `authMiddleware` | CA-05 |
| `TestCostSectionStrictRedaction` | golden do relatório | CA-06 |
| `TestCostCSVCompleteMarkerNoSQL` | API | CA-07 |
| `TestPriceValidation` (negativo, moeda inválida, troca de moeda) | unitário/API | §4 Erros, ADR-0001 D3 |
| `TestCostRankingScaleBudget` (5000 tabelas < 2 s) | escala | RNF-03 |
| `TestCostSeriesGapsOnPartialRuns` | integração | RF-09 |

## 6. Gates

Nenhum lote exige R2: só metadados e valores monetários, com rotas por ambiente no RBAC padrão. Todos os lotes ficam em **CI verde**. O lote 5 também precisa da aprovação de layout da Trinity.

## 7. Riscos

- Preço público ≠ fatura real (descontos, reservas). Mitigação: rótulo "estimado/mês", vigência e modelo de origem visíveis.
- O bloat estatístico erra com tipos de largura variável (TOAST, `text`, `jsonb`). Mitigação: confiança no máximo média, e bloat exato fica para depois (ADR-0001 D3).
- A gestão pode ler a estimativa como promessa. Mitigação: faixa de confiança e separação hipótese/ganho (BA-21).
- Mudar a moeda depois de criar a série quebraria a comparabilidade. Por isso a moeda fica travada por ambiente.

## 8. Pontos para Q (resolvidos)

- ~~Troca de moeda.~~ **Resolvido por ADR-0001 D3:** a moeda fica travada depois da primeira versão. Uma troca exige procedimento manual documentado em `docs/ops.md` e corta a série.
- ~~Provedores (P-03).~~ **Resolvido por ADR-0001 D3:** só RDS PostgreSQL, Cloud SQL e self-hosted no v1.
- **Pendente (fora do escopo de Q):** posição da aba Custo e do cartão de preços, com a Trinity no lote 5.

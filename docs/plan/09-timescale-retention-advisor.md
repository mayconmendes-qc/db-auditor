# Plano 09 — Simulação de retenção e compressão no TimescaleDB (`timescale-retention-advisor`)

- **Spec:** [../spec/09-timescale-retention-advisor.md](../spec/09-timescale-retention-advisor.md)
- **Base lida:** `main` em `b1a93cf`
- **Posição na ordem de implementação:** 8ª
- **Decisões:** [ADR-0001](../adr/0001-decisoes-v1-features.md) (Aceita, 2026-10-18)
- **Total:** 5 lotes, **18 d**, 1 lote com gate R2, Trinity necessária (1 lote)

## 0. Resumo

Um motor de simulação puro sobre `chunk_snapshot` (intervalos, `is_compressed`, `total_size_bytes`, `before_compression_bytes`/`after_compression_bytes`), `policy_snapshot`, `job_snapshot` e `continuous_aggregate_snapshot`. Ele calcula economia de retenção e compressão por hypertable, cenários combinados, projeção e riscos de CAGG. Entram também contadores de varredura por chunk, como proxy de "chunk sem consulta", e cenários salvos com achados `timescale.*`.

### Decisões aplicadas (ADR-0001)

- **ADR-0001 D9:** **chunks nunca consultados entram no v1**, usando os **contadores de varredura como proxy**, **rotulados como aproximados** ("aproximado — contadores zeram com reset"). Responde à P-01.
- **ADR-0001 D9:** o achado de cenário do CA-04 é criado **no salvamento do cenário**, vinculado à última coleta completa.
- **ADR-0001 D3 (via 02):** a conversão para moeda usa a tabela de preços do ambiente, com **uma moeda por ambiente**. Sem preço, só GB.
- **ADR-0001 D11:** tenant = ambiente. Cenários salvos pertencem ao ambiente.

### Dependências

Plano 02, lote 1 (tabela de preços), opcional. Sem ele, o resultado sai só em GB (CA-06 fica pendente).

## 1. Módulos e arquivos afetados

| Área | Caminho | Mudança |
|---|---|---|
| Simulação | `backend/internal/tsretention/` (novo: `state.go`, `retention.go`, `compression.go`, `scenario.go`, `projection.go`, `risks.go`) | cálculo puro |
| Store | `backend/internal/repository/timescale_snapshots.go`, `timescale_pages.go`, `timescale_policy_snapshots.go`, `hypertable_detail.go`, novo `repository/retention_scenarios.go` | — |
| Crescimento | `backend/internal/analyzer/maintenance_trend.go` | reuso da regra de comparabilidade (duas completas, ≤ 14 dias) para a projeção |
| Coleta de contadores | `backend/sql/collectors/timescale/chunks.sql`, `backend/internal/collectors/timescale/chunk.go`, `collectors/timescale/compat.go` | `seq_scan`, `idx_scan` e `n_tup_ins/upd/del` do chunk via `pg_stat_all_tables` |
| Regras | `backend/internal/analyzer/timescale_retention.go` (novo), `analyzer/chunk.go`, `analyzer/cagg.go`, `backend/internal/actions/catalog.go` | `timescale.retention_missing`, `timescale.compression_missing`, `timescale.retention_cagg_conflict`, `timescale.compression_ratio_low` |
| Custo | `backend/internal/cost/prices.go` (plano 02) | reuso |
| API e RBAC | `backend/internal/api/hypertable_detail.go`, novo `api/timescale_simulation.go`, `api/auth.go` (salvar cenário exige auditor) | — |
| PDF e CSV | `backend/internal/report/sections.go`, `report/testdata/*` | — |
| Frontend | `frontend/src/pages/HypertableAssessmentPanel.tsx`, `pages/parts/HypertableAssessmentTabs.tsx`, `pages/parts/useHypertableAssessment.tsx`, novo `pages/parts/HypertableSimulation.tsx`, `components/ui/chart.tsx` | — |
| Docs | `docs/rules.md` (seção BA-21: hipótese × ganho), `docs/guia-do-usuario.md` | — |

## 2. Migrations (provisórias)

| Nº provisório | Arquivo | Lote | Conteúdo |
|---|---|---|---|
| 37 | `backend/migrations/37_retention_scenarios.sql` | 2 | `retention_scenario` (ambiente, banco, schema, hypertable, retenção em dias e/ou compressão após N dias, rótulo, `saved_by`, `saved_at`, `audit_run_id` de referência, resultado resumido em GB e moeda com a versão do preço) |
| 38 | `backend/migrations/38_chunk_scan_counters.sql` | 3 | `chunk_snapshot` + `seq_scan bigint`, `idx_scan bigint`, `n_tup_ins bigint`, `n_tup_upd bigint`, `n_tup_del bigint` (nulos = "não coletado" em snapshots antigos) e `stats_reset timestamptz` |

Os números supõem 26–36 já mergeados. Lembrar `ci.yml`, `migrations/README.md` e `docs/ops.md`.

## 3. Endpoints e telas

| Método e rota | Papel | Observação |
|---|---|---|
| `GET /api/v1/environments/{id}/runs/{run}/databases/{database}/schemas/{schema}/hypertables/{hypertable}/simulation?retention_days=&compress_after_days=&scenario=...&horizon=` | viewer | simular sem gravar. Usa GET de propósito, para continuar no RBAC padrão (POST exige operator). Até 5 cenários. N ≤ 0 recebe 400 |
| `GET /api/v1/environments/{id}/timescale-scenarios` | viewer | lista de cenários salvos |
| `POST /api/v1/environments/{id}/timescale-scenarios` | auditor | salvar. Viewer recebe 403 (CA-07) |
| `GET /api/v1/environments/{id}/runs/{run}/.../hypertables/{hypertable}/unscanned-chunks` | viewer | chunks possivelmente sem consulta (aproximado) |
| `GET /api/v1/environments/{id}/timescale-scenarios/export?format=csv` | viewer | uma linha por hypertable × cenário, com `# export_complete` |

**Tela (Trinity necessária).** No painel da hypertable (`HypertableAssessmentPanel`), uma nova seção **Simulação** com a tabela de cenários (GB atuais, GB após, economia em GB e em moeda, riscos, confiança), gráfico de projeção 30/90/180 dias, faixas de idade e a separação hipótese / ganho previsto / ganho medido (RNF-06, BA-21). A spec cita o painel "Tabelas temporais", rótulo usado em `DashboardKpis` e no Inventário. A posição fica adiada para o lote 4, quando Q chamará a Trinity (ADR-0001, Pendente). Este plano não decide o lugar.

## 4. Lotes

### Lote 1 — Motor de simulação de retenção, compressão e projeção

**Issue:** `feat(tsretention): motor de simulação por hypertable — retenção N dias, compressão após N dias, cenários combinados, projeção e riscos de CAGG (RF-01–RF-04, RF-06)`

**Descrição.** Estado atual (RF-01): políticas vigentes, chunks por faixa de idade, % comprimido e taxa observada (Σantes ÷ Σdepois nos chunks comprimidos). Retenção de N dias: chunks com `range_end` mais antigo que N. Compressão após N dias: chunks não comprimidos mais antigos que N, divididos pela taxa observada. Sem chunk comprimido, usar a taxa padrão de 5× com "sem taxa observada, confiança baixa". Até 5 cenários combinados e projeção 30/90/180 com o crescimento entre duas completas comparáveis (regra de `maintenance.growth_trend`). Riscos: (a) retenção menor que a janela de refresh de um CAGG dependente; (b) retenção sobre dados de CAGG sem política de refresh; (c) compressão de chunk que ainda recebe escrita (tuplas mortas de `chunk_vacuum_sample`). Dimensão inteira sem função de "agora" fica "não aplicável". Inventário truncado (`chunk.inventory_truncated`) resulta em "parcial".

**Critérios de aceite.**
- [ ] CA-01: 400 chunks diários de 1 GB, retenção de 90 dias: economia de cerca de 310 GB, com as faixas de idade.
- [ ] CA-02: antes 10 GB e depois 2 GB resultam em taxa de 5× observada, com confiança não baixa.
- [ ] CA-03: sem chunk comprimido, taxa padrão com "confiança baixa".
- [ ] RNF-03: menos de 1 s por hypertable com 10 000 chunks (benchmark).
- [ ] Inventário truncado marca "parcial". Dimensão inteira: retenção "não aplicável".

**Arquivos.** `internal/tsretention/*`.
**Estimativa:** 4 d. **Gate:** CI verde (cálculo puro sobre metadados). **Depende de:** —

### Lote 2 — API de simulação, cenários salvos, regras e custo

**Issue:** `feat(tsretention): simulação via API (viewer), cenários salvos (auditor), achados timescale.* e conversão para moeda (RF-05, RF-08)`

**Descrição.** Migration `37_retention_scenarios.sql` (provisória). Simulação por GET (viewer, sem gravar). Salvar exige auditor, o que pede uma regra em `authorizeRequest` para `/timescale-scenarios`. Regras com os limiares da §3 (**valor inicial, calibrar**): `retention_missing` (baixa), `compression_missing` (média), `retention_cagg_conflict` (alta, avaliada contra a política vigente e **no momento em que o cenário é salvo**, vinculada à última coleta completa) e `compression_ratio_low` (baixa). O achado no salvamento segue a ADR-0001 D9. Nenhuma recomendação sugere exclusão automática, e o texto BA-21 continua: retenção exige aprovação de negócio e backup restaurável. Conversão para moeda pela tabela de preços vigente (plano 02), com a vigência. Ambiente sem Timescale: "não aplicável". Métricas de simulações e de hypertables sem taxa observada (RNF-05).

**Critérios de aceite.**
- [ ] CA-04: CAGG com refresh de 120 dias e cenário de 90 dias salvo gera `timescale.retention_cagg_conflict` alta.
- [ ] CA-05 (API): PostgreSQL sem Timescale responde "não aplicável" sem erro.
- [ ] CA-06: com preços configurados, a economia aparece em moeda, com a vigência do preço.
- [ ] CA-07: viewer recebe 403 ao salvar.
- [ ] N ≤ 0 recebe 400.
- [ ] Regras com os cinco campos e consulta segura (`timescaledb_information`) no catálogo.

**Arquivos.** `migrations/37_retention_scenarios.sql`, `internal/api/timescale_simulation.go`, `internal/api/hypertable_detail.go`, `internal/api/auth.go`, `internal/repository/retention_scenarios.go`, `internal/analyzer/timescale_retention.go`, `internal/actions/catalog.go`, `internal/observability/metrics.go`, `api/openapi.json`, `docs/rules.md`.
**Estimativa:** 4 d. **Gate:** **R2** (mudança em `authorizeRequest`). **Depende de:** lote 1 (plano 02 lote 1 para o CA-06).

### Lote 3 — Contadores de varredura por chunk (proxy aproximado)

**Issue:** `feat(collector): contadores de varredura e escrita por chunk e lista de chunks possivelmente sem consulta, rotulada aproximada (RF-07)`

**Descrição.** Migration `38_chunk_scan_counters.sql` (provisória). `chunks.sql` faz junção com `pg_stat_all_tables` pelo relid do chunk (pela conexão só leitura, dentro do orçamento de 120 s local já usado para chunks em `docs/ops.md`). Chunk sem varredura entre duas coletas completas e comparáveis (sem `stats_reset`, regra AN-11) vira "possivelmente sem consulta", **sempre** como hipótese **aproximada** (ADR-0001 D9). Contadores zerados por reset invalidam a janela daquele chunk. Também alimenta o risco (c) do lote 1 (chunk recente com escrita).

**Critérios de aceite.**
- [ ] CA-05: ambiente PostgreSQL sem Timescale não faz nenhum coletor falhar. A regressão do #125 continua verde (`TestFullCollectionWithTimescaleIsCompleteIntegration` nos jobs PG 16–18 com TimescaleDB 2.30.2; em PG 14/15 o teste só de Timescale é pulado, como hoje).
- [ ] O rótulo "aproximado" aparece na API e em toda saída.
- [ ] Reset entre as coletas resulta em "não comparável", não em "sem consulta".
- [ ] Snapshots antigos mostram "não coletado".

**Arquivos.** `migrations/38_chunk_scan_counters.sql`, `sql/collectors/timescale/chunks.sql`, `internal/collectors/timescale/chunk.go`, `internal/collectors/timescale/compat.go`, `internal/repository/timescale_snapshots.go`, `internal/tsretention/risks.go`, `internal/api/timescale_simulation.go`.
**Estimativa:** 4 d. **Gate:** CI verde (leitura de estatística de catálogo, sem dado de hypertable). **Depende de:** lote 1.

### Lote 4 — Seção Simulação no painel da hypertable

**Issue:** `feat(ui): seção Simulação no painel da hypertable com cenários lado a lado, projeção e riscos`

**Descrição.** Tela desenhada com a Trinity. Até 5 cenários lado a lado, economia em GB e moeda, riscos, confiança, gráfico de projeção acessível (padrão #122), chunks possivelmente sem consulta com o selo "aproximado" e "salvar cenário" (auditor).

**Critérios de aceite.**
- [ ] CA-05 na UI: "não aplicável" sem Timescale.
- [ ] CA-07 na UI: viewer simula e não vê "salvar".
- [ ] RNF-06: hipótese, ganho previsto e ganho medido separados.
- [ ] Layout aprovado pela Trinity.

**Arquivos.** `frontend/src/pages/HypertableAssessmentPanel.tsx`, `frontend/src/pages/parts/HypertableAssessmentTabs.tsx`, `frontend/src/pages/parts/HypertableSimulation.tsx`, `frontend/src/pages/parts/useHypertableAssessment.tsx`, `frontend/src/services/api.ts`, `docs/guia-do-usuario.md`.
**Estimativa:** 4 d. **Gate:** CI verde + Trinity. **Depende de:** lotes 2 e 3.

### Lote 5 — CSV dos cenários e seção no PDF técnico

**Issue:** `feat(report): CSV de cenários e seção Retenção e compressão (simulação) no PDF técnico (RF-09)`

**Descrição.** CSV com uma linha por hypertable × cenário, terminando em `# export_complete`. Seção do PDF com a ressalva de hipótese e os limites do BA-11. Golden.

**Critérios de aceite.**
- [ ] CSV com o marcador final, e a contagem de linhas confere.
- [ ] Golden do PDF com e sem Timescale ("não aplicável").

**Arquivos.** `internal/api/timescale_simulation.go`, `internal/report/sections.go`, `internal/report/testdata/*`.
**Estimativa:** 2 d. **Gate:** CI verde. **Depende de:** lote 2.

## 5. Estratégia de testes

| Teste | Tipo | Critério ligado |
|---|---|---|
| `TestRetention90DaysOn400DailyChunks` (≈ 310 GB) | unitário `internal/tsretention` | CA-01 |
| `TestCompressionObservedRatio5x` | unitário | CA-02 |
| `TestCompressionDefaultRatioLowConfidence` | unitário | CA-03 |
| `TestSavedScenarioCaggConflictHigh` (120 × 90 dias) | integração | CA-04 |
| `TestNoTimescaleNotApplicableAndCollectorsOK` + regressão #125 | integração / matriz CI | CA-05 |
| `TestSimulationCurrencyWithPriceVersion` | integração (com o plano 02) | CA-06 |
| `TestViewerCannotSaveScenario` | API com `authMiddleware` | CA-07 |
| `TestSimulationBudget10kChunks` | benchmark | RNF-03 |
| `TestTruncatedInventoryPartial`, `TestIntegerDimensionNotApplicable`, `TestInvalidScenario400` | unitário/API | §4 Erros |
| `TestUnscannedChunksApproximateAndResetAware` | integração | RF-07, ADR-0001 D9 |

## 6. Gates

| Lote | Gate | Motivo |
|---|---|---|
| 1 | CI verde | cálculo puro |
| 2 | **R2** | mudança em `authorizeRequest` |
| 3 | CI verde | estatística de catálogo |
| 4 | CI verde + Trinity | UI |
| 5 | CI verde | metadados |

## 7. Riscos

- A taxa de compressão muda com o tempo (cardinalidade). Mitigação: mostrar a taxa e a quantidade de chunks que a sustentam.
- Contadores por chunk zeram com reset, então "sem consulta" é fraco. Mitigação: rótulo "aproximado" (ADR-0001 D9) e invalidação por reset.
- Retenção é decisão de compliance. Mitigação: texto BA-21 e nenhuma recomendação de exclusão automática.
- TimescaleDB sem imagem 2.30.2 para PG 14/15: os testes só de Timescale são pulados nessas versões (comportamento atual da matriz).

## 8. Pontos para Q (resolvidos)

- ~~Achado no salvamento do cenário (CA-04).~~ **Resolvido por ADR-0001 D9:** criado no salvamento, vinculado à última coleta completa. O CA-04 da spec foi ajustado.
- **Pendente (fora do escopo de Q):** posição da seção Simulação (painel da hypertable ou "Tabelas temporais"), com a Trinity no lote 4.

# Plano 03 — Contratos de qualidade de dados (`dq-contracts`)

- **Spec:** [../spec/03-dq-contracts.md](../spec/03-dq-contracts.md)
- **Base lida:** `main` em `b1a93cf`
- **Posição na ordem de implementação:** 7ª
- **Decisões:** [ADR-0001](../adr/0001-decisoes-v1-features.md) (Aceita, 2026-10-18)
- **Total:** 7 lotes, **27 d**, 3 lotes com gate R2, Trinity necessária (1 lote)

## 0. Resumo

O plano estende o diagnóstico de qualidade sob demanda que já existe (`internal/quality`, `POST .../quality-scans`, tabelas `quality_scan`, `quality_issue` e `quality_issue_event`, flag `AUDITOR_DATA_QUALITY_ENABLED`). Entram contratos versionados por tabela (YAML ou UI), seis tipos de checagem, proposta gerada do schema, execução por agenda, sob demanda e pela CLI de CI, achados `quality.contract_violation` com dedup e tendência. Só contagens são persistidas.

### Decisões aplicadas (ADR-0001)

- **ADR-0001 D4 (emendada):** a **amostra padrão dos contratos é de 10 000 linhas** por checagem (o RNF-03 da spec prevalece). Ela vai **até 100 000 por opt-in**:
  - o operator sobe o teto do processo com `AUDITOR_DATA_QUALITY_MAX_SAMPLE` (padrão 10 000, máximo 100 000);
  - cada contrato declara o próprio `sample_limit` dentro desse teto.

  O diagnóstico sob demanda existente (`POST .../quality-scans`, `quality.Request.Validate`) **mantém o teto de 1000**. Responde à P-01.
- **ADR-0001 D4:** a varredura de PII (spec 05) tem **flag própria, mais restrita**, e amostra própria. Aqui continua valendo `AUDITOR_DATA_QUALITY_ENABLED`.
- **ADR-0001 D13:** o **contrato canônico fica no auditor**. O YAML importado cria uma versão `proposed`. O YAML passado à CLI é **execução ad hoc** registrada como `cli`, que não cria nem aprova versão. Violação persistente usa a **dedup e a notificação de achados**, sem categoria nova em `regression_alert`. Responde à P-02.
- **ADR-0001 D1:** a CLI usa o **token por ambiente** (plano 01, lote 3), com o novo escopo `quality-contract:run`.
- **ADR-0001 D2 (por analogia):** a CLI de CI **não recebe DSN**. Ela pede a execução à API, e quem se conecta ao alvo é a API, com a conta só leitura que já existe.
- **ADR-0001 D11:** tenant = ambiente.

### Dependências

Plano 01 (modo CLI, lote 1; token, lote 3). Plano 05, lote 3 (amostragem por páginas endurecida em `internal/quality/sample.go`). Uso opcional da 04 (alerta de coluna ausente após migration, a partir da timeline).

## 1. Módulos e arquivos afetados

| Área | Caminho | Mudança |
|---|---|---|
| Contratos | `backend/internal/dqcontract/` (novo: `model.go`, `yaml.go` com erro por linha e campo, `propose.go`, `checks.go`, `run.go`, `markdown.go`) | — |
| Dependência YAML | `backend/go.mod`, `backend/go.sum` | nova dependência de parser YAML (por exemplo `go.yaml.in/yaml/v3`). Hoje o módulo só tem pgx, o driver Mongo e `x/term`. Passa pelo Trivy do CI |
| Amostragem | `backend/internal/quality/probe.go`, `quality/sample.go` (plano 05), `quality/plan.go` | reuso das checagens null, duplicate, orphan e date_range. Domínio, frescor e volume são novos |
| Store | `backend/internal/repository/quality.go`, novo `repository/quality_contract.go` | — |
| Agendamento | `backend/internal/scheduler/scheduler.go`, `backend/internal/api/schedules.go` | contratos ligados aos perfis existentes (`fast`/`daily`/`weekly`/`monthly`) |
| Regras e alertas | `backend/internal/analyzer/quality_contract.go` (novo), `backend/internal/actions/catalog.go`, `backend/internal/repository/alerts.go` (`NotifyFindings`/`ClaimAlert`) | `quality.contract_violation` |
| API e RBAC | `backend/internal/api/quality_contracts.go` (novo), `backend/internal/api/auth.go` (rotas `/quality-contracts` exigem auditor nas escritas; ramo de token com `quality-contract:run`) | — |
| CLI | `backend/cmd/auditor/main.go`, `backend/internal/cli/`, novo `backend/internal/dqcontract/cli.go` | `auditor dq-contracts run` |
| PDF | `backend/internal/report/sections.go`, `report/testdata/*` | seção "Contratos de qualidade" |
| Frontend | `frontend/src/pages/AssistedActionsPage.tsx`, `pages/parts/AssistedScanResults.tsx`, novos `pages/parts/QualityContracts*.tsx`, `services/api.ts` | — |
| Docs | `docs/dq-contracts.md` (novo: esquema YAML publicado, RF-01), `docs/rules.md`, `docs/ops.md`, `docs/guia-do-usuario.md` | — |

## 2. Migrations (provisórias)

| Nº provisório | Arquivo | Lote | Conteúdo |
|---|---|---|---|
| 36 | `backend/migrations/36_dq_contracts.sql` | 1 | `quality_contract` (ambiente, banco, schema, tabela, criador), `quality_contract_version` (imutável: número, documento canônico em jsonb, YAML de origem normalizado, `sample_limit`, perfil de agenda, estado `proposed`/`approved`/`superseded`, `authored_by`, `approved_by`, datas), `quality_contract_run` (versão, modo `schedule`/`on_demand`/`cli`, estado, cobertura, origem do token quando CLI), `quality_contract_check_result` (checagem, resultado `passed`/`violated`/`not_executed`, motivo, contagens, proporção, amostra, método, `freshness_age_hours`; **nunca valores**). `quality_scan.check_kind` (CHECK em `07_third_stage.sql`) não muda: as tabelas de contrato são novas |

Os números supõem 26–35 já mergeados. Lembrar `ci.yml`, `migrations/README.md` e `docs/ops.md`.

## 3. Endpoints e telas

| Método e rota | Quem | Observação |
|---|---|---|
| `GET /api/v1/environments/{id}/quality-contracts` | viewer | lista com o estado da última execução |
| `POST /api/v1/environments/{id}/quality-contracts` | auditor | cria a versão 1 `proposed` |
| `POST /api/v1/environments/{id}/quality-contracts/{contract}/versions` | auditor | nova versão imutável |
| `POST /api/v1/environments/{id}/quality-contracts/{contract}/versions/{version}/approve` | auditor | viewer recebe 403 (CA-08) |
| `POST /api/v1/environments/{id}/quality-contracts/proposals` | auditor | gera proposta do schema (RF-03) |
| `POST /api/v1/environments/{id}/quality-contracts/{contract}/runs` | auditor **ou** token `quality-contract:run` | 409 se o recurso estiver desabilitado |
| `GET /api/v1/environments/{id}/quality-contracts/{contract}/runs`, `GET .../runs/{run}` | viewer | — |
| `GET /api/v1/environments/{id}/quality-contracts/{contract}/trend` | viewer | série por checagem |
| `GET .../quality-contracts/{contract}/versions/{version}/yaml` e `POST .../quality-contracts/import` | viewer (export) / auditor (import) | YAML inválido: 400 com linha e campo |
| `GET .../quality-contracts/{contract}/runs/{run}/markdown` | viewer | — |

Lista de domínio (RNF-02): só aparece para auditor ou acima. A resposta para viewer omite o campo.

**Tela (Trinity necessária).** Em Ações assistidas → Qualidade (`AssistedActionsPage`/`AssistedScanResults`): lista de contratos por tabela com o estado da última execução, tendência, botão "gerar proposta", diff entre versões e aprovação. A posição na navegação citada aqui é a da spec. A decisão final fica adiada para o lote de tela, quando Q chamará a Trinity (ADR-0001, Pendente).

## 4. Lotes

### Lote 1 — Contrato versionado, YAML e RBAC

**Issue:** `feat(dq-contracts): entidade contrato versionado e imutável, formato YAML com esquema publicado, importar/exportar e aprovação (spec 03 RF-01)`

**Descrição.** Migration `36_dq_contracts.sql` (provisória). Modelo canônico com as seis checagens do RF-02, no máximo 50 checagens por contrato, coluna "crítica" para nulos e tolerância de domínio. Parser YAML com mensagens de linha e campo (400). Cada alteração gera versão nova. A aprovação é uma transição única (`proposed` → `approved`, e a anterior vira `superseded`). `authorizeRequest`: escritas em `/quality-contracts` exigem auditor (como já acontece com `/quality-scans`). `docs/dq-contracts.md` publica o esquema.

**Critérios de aceite.**
- [ ] CA-08: viewer recebe 403 ao aprovar.
- [ ] YAML inválido recebe 400 com linha e campo. Teste de tabela com erros comuns.
- [ ] A versão aprovada nunca é alterada (o teste tenta `UPDATE` pela API e recebe 409 ou 405).
- [ ] RNF-02: lista de domínio omitida na resposta para viewer.
- [ ] Nova dependência YAML aprovada pelo Trivy e sem cgo (`CGO_ENABLED=0` no `Containerfile`).

**Arquivos.** `migrations/36_dq_contracts.sql`, `internal/dqcontract/model.go`, `internal/dqcontract/yaml.go`, `internal/repository/quality_contract.go`, `internal/api/quality_contracts.go`, `internal/api/auth.go`, `go.mod`, `go.sum`, `api/openapi.json`, `docs/dq-contracts.md`.
**Estimativa:** 5 d. **Gate:** **R2** (mudança em `authorizeRequest`; nova dependência externa). **Depende de:** —

### Lote 2 — Proposta de contrato a partir do schema coletado

**Issue:** `feat(dq-contracts): gerar contrato proposto do snapshot (obrigatórias → nulos 0%, PK/unique → unicidade, FK → órfãs, coluna de atualização → frescor) (RF-03)`

**Descrição.** A partir de `column_snapshot`, `constraint_snapshot` e `table_snapshot` da última coleta completa: colunas obrigatórias → nulos 0%, PK e unicidade → unicidade, FK simples → órfãs, coluna temporal de atualização reconhecida por nome (`updated_at`, `atualizado_em`, `modified_at`…) → frescor de 24 h (**valor inicial, calibrar**). O resultado é sempre `proposed`, nunca vigora sozinho.

**Critérios de aceite.**
- [ ] CA-01: tabela com coluna obrigatória e FK gera contrato com nulos 0% e FK órfã, no estado "proposto", sem execução agendada.
- [ ] Proposta sem coleta completa: 409 com motivo.

**Arquivos.** `internal/dqcontract/propose.go`, `internal/api/quality_contracts.go`.
**Estimativa:** 2 d. **Gate:** CI verde. **Depende de:** lote 1.

### Lote 3 — Execução das seis checagens com amostragem segura

**Issue:** `feat(dq-contracts): executor de nulos, unicidade, FK órfã, domínio, frescor e volume com amostra 10 000 por padrão e opt-in até 100 000 (RF-02, RF-05–RF-07)`

**Descrição.** Usa a amostragem compartilhada do plano 05 (`quality/sample.go`): transação só leitura, conta sem escrita, páginas obrigatórias em tabelas acima de 1 GB, 30 s de timeout por checagem e no máximo 4 checagens simultâneas por ambiente. Amostra padrão de 10 000, com opt-in até 100 000 (ADR-0001 D4; `AUDITOR_DATA_QUALITY_MAX_SAMPLE`, padrão 10 000). O teto de 1000 de `quality.Request.Validate` continua valendo para `quality-scans`: a amostragem compartilhada recebe o teto do chamador. Nulos: pré-checagem por `column_stat_snapshot.null_fraction` e confirmação na amostra. Frescor: `max(coluna)` agregado no alvo, e o processo guarda **só a idade em horas**, nunca o carimbo (RNF-02). Volume: variação de `table_snapshot` (linhas estimadas) entre coletas, ±50%, com confiança baixa. Domínio: lista ou intervalo, com tolerância. O resultado é `passed`/`violated`/`not_executed` com motivo (timeout, sem permissão, tabela ou coluna ausente). Toda contagem amostral traz o tamanho da amostra e o aviso de margem de erro (RNF-06). Violação gera `quality.contract_violation`, com dedup por contrato + checagem + versão e severidades da §3. A violação persistente entra no fluxo de dedup e notificação de achados existente (`NotifyFindings`/`ClaimAlert`, regra de lacuna AN-11), sem categoria nova em `regression_alert` (ADR-0001 D13). Recurso desligado: 409.

**Critérios de aceite.**
- [ ] CA-02: máximo de 1%, amostra com 3% de nulos. Grava 3%, a amostra e um achado médio. Nenhum valor de linha é persistido (varredura do store como no plano 05).
- [ ] CA-03 (API): recurso desabilitado gera 409.
- [ ] CA-04: frescor de 24 h com último registro de 30 h guarda "30 h", e o carimbo não está no store nem nos logs.
- [ ] CA-06: checagem que estoura o timeout fica "não executada — timeout".
- [ ] CA-07: a mesma violação em duas execuções completas seguidas cria um único alerta (dedup existente).
- [ ] `sample_limit` acima do teto do processo recebe 400. Sem opt-in, o teto é 10 000. `quality-scans` continua recusando mais de 1000 (testes existentes verdes).
- [ ] RNF-05: `auditor_quality_contract_checks_total{result}` e duração. Logs sem valores.

**Arquivos.** `internal/dqcontract/checks.go`, `internal/dqcontract/run.go`, `internal/quality/sample.go`, `internal/quality/probe.go`, `internal/analyzer/quality_contract.go`, `internal/actions/catalog.go`, `internal/repository/quality_contract.go`, `internal/repository/alerts.go`, `internal/observability/metrics.go`, `docs/rules.md`.
**Estimativa:** 6 d. **Gate:** **R2** (lê linhas reais do alvo; dado pessoal). **Depende de:** lote 1 e plano 05 lote 3.

### Lote 4 — Execução agendada, sob demanda e tendência

**Issue:** `feat(dq-contracts): execução por perfil do agendador e sob demanda, histórico e tendência por checagem (RF-04, RF-08)`

**Descrição.** O contrato aprovado se liga a um perfil do agendador. A execução acontece depois da coleta do perfil, para que volume e pré-checagens usem o snapshot fresco. Sob demanda pela API (auditor). Série por checagem (% e contagens) para a tendência, paginada e limitada a 90 dias (padrão das séries).

**Critérios de aceite.**
- [ ] A execução agendada respeita o limite de 4 checagens simultâneas por ambiente.
- [ ] A tendência mostra lacuna em execução parcial.
- [ ] Desligar `AUDITOR_DATA_QUALITY_ENABLED` suspende as execuções agendadas, com motivo registrado.

**Arquivos.** `internal/scheduler/scheduler.go`, `internal/dqcontract/run.go`, `internal/api/quality_contracts.go`, `internal/repository/quality_contract.go`.
**Estimativa:** 3 d. **Gate:** CI verde. **Depende de:** lote 3.

### Lote 5 — CLI de CI com gate por severidade

**Issue:** `feat(cli): auditor dq-contracts run com gate por severidade, saída Markdown/JSON e códigos 0–4 (RF-04, RF-09)`

**Descrição.** `auditor dq-contracts run --environment <id> (--contract <id> | --file c.yaml | --dir contracts/) --fail-on high --format md|json`. Autentica com o token por ambiente (escopo `quality-contract:run`, adicionado à lista fechada do plano 01). A CLI **não** se conecta ao alvo: pede a execução à API e acompanha o resultado. Com `--file`/`--dir`, o YAML vai como execução *ad hoc* de uma versão não aprovada, registrada como `cli` (ADR-0001 D13). Códigos: 1 a partir de alta, 2 para recurso desabilitado (409) ou YAML inválido, 3 para checagem não executada.

**Critérios de aceite.**
- [ ] CA-03 (CLI): recurso desabilitado sai com código 2.
- [ ] CA-05: gate em alta com violação de unicidade sai com código 1, e o Markdown lista a violação.
- [ ] CA-06 (CLI): timeout sai com código 3.
- [ ] Token de outro ambiente gera 403 → código 2.

**Arquivos.** `cmd/auditor/main.go`, `internal/dqcontract/cli.go`, `internal/dqcontract/markdown.go`, `internal/api/auth.go` (escopo novo), `docs/dq-contracts.md`.
**Estimativa:** 4 d. **Gate:** **R2** (novo escopo de token que dispara leitura no alvo). **Depende de:** lote 3 e plano 01 lotes 1 e 3.

### Lote 6 — UI de contratos em Ações assistidas → Qualidade

**Issue:** `feat(ui): contratos de qualidade por tabela com proposta, diff de versões, aprovação, execução e tendência`

**Descrição.** Tela desenhada com a Trinity. Lista por tabela, estado da última execução, tendência, "gerar proposta", editor com validação e diff entre versões, aprovar (auditor) e executar. Aviso de margem de erro e tamanho da amostra em toda contagem (RNF-06).

**Critérios de aceite.**
- [ ] CA-01 visível: a proposta aparece como "proposto", sem vigorar.
- [ ] CA-08 na UI: viewer não vê "aprovar".
- [ ] Recurso desabilitado: mensagem do 409 na tela.
- [ ] Layout aprovado pela Trinity.

**Arquivos.** `frontend/src/pages/AssistedActionsPage.tsx`, `frontend/src/pages/parts/AssistedScanResults.tsx`, `frontend/src/pages/parts/QualityContracts*.tsx`, `frontend/src/services/api.ts`, `docs/guia-do-usuario.md`.
**Estimativa:** 5 d. **Gate:** CI verde + Trinity. **Depende de:** lote 4.

### Lote 7 — Markdown para download e seção no PDF técnico

**Issue:** `feat(report): relatório Markdown de execução de contrato e seção Contratos de qualidade no PDF técnico (RF-09)`

**Descrição.** Tabela por checagem com resultado, contagem, amostra e severidade. Seção no PDF técnico com redação, sem lista de domínio nos modos "identificadores" e "estrita". Golden files.

**Critérios de aceite.**
- [ ] Golden do Markdown e do PDF.
- [ ] Nenhum valor de linha nem carimbo de frescor no PDF (CA-02 e CA-04 aplicados ao relatório).

**Arquivos.** `internal/dqcontract/markdown.go`, `internal/report/sections.go`, `internal/report/testdata/*`.
**Estimativa:** 2 d. **Gate:** CI verde. **Depende de:** lote 4.

## 5. Estratégia de testes

| Teste | Tipo | Critério ligado |
|---|---|---|
| `TestProposalFromSnapshot` | integração | CA-01, RF-03 |
| `TestNullCheckStoresOnlyCounts` (3% > 1%, achado médio, varredura do store) | integração no alvo descartável | CA-02, RNF-02 |
| `TestContractRunDisabled409AndCLIExit2` | API + CLI | CA-03 |
| `TestFreshnessStoresHoursNotTimestamp` | integração | CA-04 |
| `TestCLIGateHighUniqueViolationExit1` | CLI + API | CA-05 |
| `TestCheckTimeoutNotExecutedExit3` | integração + CLI | CA-06 |
| `TestPersistentViolationSingleAlert` | integração (duas execuções completas) | CA-07 |
| `TestViewerCannotApprove` | API com `authMiddleware` | CA-08 |
| `TestYAMLErrorsLineAndField` | unitário | §4 Erros, RF-01 |
| `TestSampleLimitOptIn` (10 000 padrão; 100 000 só com o teto do processo; `quality-scans` mantém 1000) | unitário/API | ADR-0001 D4, RNF-03 |
| `TestCLIAdHocYAMLDoesNotCreateOrApproveVersion` | API + CLI | ADR-0001 D13 |
| `TestConcurrentChecksPerEnvironmentCap` | integração | RNF-03 |
| Testes existentes de `internal/quality` | regressão | RNF-01 |

## 6. Gates

| Lote | Gate | Motivo |
|---|---|---|
| 1 | **R2** | RBAC e dependência externa |
| 2 | CI verde | só metadados |
| 3 | **R2** | leitura de linhas reais |
| 4 | CI verde | agenda sobre o executor já revisado |
| 5 | **R2** | escopo de token que dispara leitura no alvo |
| 6 | CI verde + Trinity | UI |
| 7 | CI verde | relatório de contagens |

## 7. Riscos

- Amostra ainda limitada (10 000 por padrão) gera falso "aprovado", principalmente em unicidade. Mitigação: texto "sem duplicata na amostra", tamanho da amostra sempre visível e opt-in de amostra maior.
- Carga de leitura em produção: o padrão de 10 000 (ADR-0001 D4) já é 10× o diagnóstico atual, e o opt-in vai até 100 000. Mitigação: páginas obrigatórias acima de 1 GB, timeout de 30 s por checagem, 4 checagens simultâneas por ambiente e opt-in do operator acima de 10 000.
- Contrato desatualizado depois de uma migration. Mitigação: checagem "não executada — coluna ausente" e, com a 04, um alerta pela timeline.
- A nova dependência YAML aumenta a superfície (parser de entrada não confiável vinda da CLI). Mitigação: limites de tamanho (1 MiB da API) e de profundidade.

## 8. Pontos para Q (resolvidos)

- ~~Amostra padrão 10 000 (spec) × 1000 (decisão).~~ **Resolvido por ADR-0001 D4 (emendada):** vale a spec, com 10 000 de padrão e opt-in até 100 000.
- ~~Contrato canônico (P-02).~~ **Resolvido por ADR-0001 D13:** auditor canônico, YAML via CLI como execução *ad hoc*.
- ~~Categoria `quality` em `regression_alert`.~~ **Resolvido por ADR-0001 D13:** não. Dedup e notificação de achados. RF-07 e CA-07 da spec foram ajustados.
- **Pendente (fora do escopo de Q):** posição da tela de contratos (a spec sugere Ações assistidas → Qualidade), com a Trinity no lote 6.

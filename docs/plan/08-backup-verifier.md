# Plano 08 — Prova de restauração do backup do banco auditado (`backup-verifier`)

- **Spec:** [../spec/08-backup-verifier.md](../spec/08-backup-verifier.md)
- **Base lida:** `main` em `b1a93cf`
- **Posição na ordem de implementação:** 9ª
- **Total:** 4 lotes, **20 d**, 3 lotes com gate R2, Trinity necessária (1 lote)

## 0. Resumo

Um subcomando da CLI restaura um `pg_dump` do banco auditado num **container efêmero, sem rede de saída**, no host em que a CLI roda. Depois verifica inventário, contagens, integridade física (amcheck) e, opcionalmente, o hash de amostra por tabela, mede o RTO e envia a evidência `target_restore_drill` para o ambiente no auditor. O procedimento e o formato de evidência do #106 (`deploy/snapshot-restore-verify.sh`, testes em `backend/internal/ops/restore_verify_test.go`) servem de modelo. A evidência do snapshot store (`retention_evidence`, `POST /api/v1/retention-policy/evidence`) continua separada e alimenta só a retenção.

### Decisões padrão aplicadas

- **decisão padrão (Q, nº 8):** a **restauração roda só pela CLI, num runner externo** (runner de CI ou máquina de operação). **A API nunca recebe acesso ao socket de container** (RNF-01). Responde à P-01.
- **decisão padrão (Q, nº 8):** **só `pg_dump`** (formato custom ou diretório) no v1. pgBackRest e WAL-G ficam fora. Responde à P-02.
- **decisão padrão (Q, nº 8):** o **hash de amostra por tabela é opt-in** (`--sample-hash`). Por padrão, o RF-05(c) não roda e não lê linhas da produção. Responde à P-03.
- **decisão padrão (Q, nº 1):** a evidência é enviada com o **token por ambiente** (plano 01), escopo novo `restore-evidence:write`.
- **decisão padrão (Q, nº 11):** tenant = ambiente.

### Dependências

Plano 01 (lote 1, modo CLI; lote 3, token). Uso opcional do plano 04 (diff de inventário via `internal/schemadiff`). Sem ele, usa `internal/comparison`.

## 1. Módulos e arquivos afetados

| Área | Caminho | Mudança |
|---|---|---|
| Orquestração | `backend/internal/restoreverify/` (novo: `runtime.go` chamando o binário `docker`/`podman` por `exec`, sem SDK nem socket na API; `container.go` com `--network none`, rótulo por execução, credencial gerada e destruição com `defer`; `prevalidate.go`; `rto.go`; `verify_inventory.go`; `verify_counts.go`; `verify_sample_hash.go`; `verify_amcheck.go`; `evidence.go`) | — |
| Coletores contra a cópia | `backend/internal/collectors/postgres/*` (`table.go`, `index.go`, `function.go`, `relation.go`), `backend/internal/comparison/compare.go` ou `internal/schemadiff` (plano 04) | reuso, sem a verificação de conta só leitura contra a cópia (a cópia é local e descartável; a recusa de escrita continua valendo para a **origem**) |
| CLI | `backend/cmd/auditor/main.go`, `backend/internal/cli/` | `auditor restore-verify` |
| Origem (opt-in) | `backend/internal/collectors/postgres/readonly_connection.go` | leitura só leitura de amostra para o hash, só com `--sample-hash` |
| Cifra | dependência opcional de `age` (padrão BA-16), decifração em memória ou no volume efêmero | — |
| API | novo `backend/internal/api/restore_drills.go`, `backend/internal/api/auth.go` (escopo `restore-evidence:write`), `api/health.go` | — |
| Store e regras | novo `backend/internal/repository/restore_drills.go`, `repository/backup_age.go` (padrão de idade), novo `backend/internal/analyzer/backup.go`, `backend/internal/actions/catalog.go` | `backup.restore_failed`, `backup.restore_divergence`, `backup.rto_exceeded`, `backup.drill_overdue` |
| Métricas | `backend/internal/observability/metrics.go` | `auditor_target_restore_drill_age_seconds{environment}` (padrão de `auditor_snapshot_backup_age_seconds`), `auditor_target_restore_rto_seconds{environment}` |
| PDF | `backend/internal/report/sections.go`, `report/testdata/*` | seção "Restauração verificada" (técnico e executivo) |
| Frontend | novo `frontend/src/pages/parts/RestoreDrillsCard.tsx` em `StatusPage.tsx` ou `EnvironmentsPage.tsx`, `services/api.ts` | — |
| Docs | novo `docs/backup-verifier.md` (runbook: runner, imagem, disco, isolamento, opt-in de hash), `docs/ops.md` (seção "Backup verificado e restauração de teste (BA-10)" passa a apontar para o novo runbook), `docs/rules.md` | — |

## 2. Migrations (provisórias)

| Nº provisório | Arquivo | Lote | Conteúdo |
|---|---|---|---|
| 39 | `backend/migrations/39_target_restore_drill.sql` | 3 | `target_restore_drill` (ambiente, `kind='target_restore_drill'`, `artifact_sha256` com CHECK `^[0-9a-f]{64}$` como em `retention_evidence`, `succeeded`, RTO em segundos, tempos de pré-validação e verificações, resultado por verificação em jsonb, divergências com tabela, tipo e contagens, **sem valores**, versão da CLI, id do token, `performed_at`, `recorded_at`); `target_restore_policy` (ambiente, RTO-alvo, prazo máximo entre ensaios, padrão 30 dias) |

Os números supõem 26–38 já mergeados. Lembrar `ci.yml`, `migrations/README.md` e `docs/ops.md`.

## 3. Endpoints e telas

| Método e rota | Quem | Observação |
|---|---|---|
| `GET /api/v1/environments/{id}/restore-drill-context` | token `restore-evidence:write` | devolve a major da origem, a última coleta completa (resumo de inventário e linhas estimadas, contadores de escrita para "comparável") e **fingerprints** (sha256 de `host:porta/banco`) dos alvos configurados do ambiente, para a recusa do RF-03. Nunca devolve DSN |
| `POST /api/v1/environments/{id}/restore-drills` | token `restore-evidence:write` | corpo validado. Recusa campos de valor |
| `GET /api/v1/environments/{id}/restore-drills`, `GET .../{drill}` | viewer | histórico e detalhe |
| `GET`/`PUT /api/v1/environments/{id}/restore-drill-policy` | viewer / operator | RTO-alvo e prazo (RF-09) |

**Tela (Trinity necessária).** Área **Restauração verificada** (a spec diz Ambientes ou Status; a Trinity escolhe): histórico, último sucesso, RTO × alvo, tendência, divergências por tabela e link para os achados. Configuração de RTO-alvo e prazo para operator.

## 4. Lotes

### Lote 1 — Orquestração de container efêmero, pré-validação e isolamento

**Issue:** `feat(cli): auditor restore-verify — pré-validação, container efêmero sem rede, confirmação de isolamento, RTO e limpeza garantida (RF-01–RF-04)`

**Descrição.** Entradas (RF-01): `--backup` (custom ou diretório), `--sha256` opcional, `--environment`, `--runtime docker|podman`, `--image postgres:N` (major ≥ origem), `--confirm-isolated-target` e `--submit`. Pré-validação: sha256, legibilidade e `pg_restore --list` **dentro** do container, porque a imagem distroless da API não tem `pg_restore`. Disco livre ≥ 2 × tamanho da origem, verificado antes. Major da imagem ≥ major da origem (do `restore-drill-context`). Isolamento: sem `--confirm-isolated-target`, código 4 sem criar container. Destino coincidente com o fingerprint de qualquer alvo configurado: código 4. Container com `--network none`, volume efêmero, credencial gerada por execução e rótulo `db-auditor.run=<uuid>`. `defer` com remoção de container e volume em sucesso, falha ou sinal, mais varredura de órfãos pelo rótulo no início. Artefato cifrado com `age`: decifração em memória ou no volume efêmero (RNF-02). RTO do início da restauração até o banco aceitar conexão com todos os objetos. Teto padrão de 2 h. Falha em qualquer etapa resulta em evidência `succeeded=false`, como no #106.

**Critérios de aceite.**
- [ ] CA-02: dump com 1 byte alterado e checksum informado: `succeeded=false`, nada restaurado e código 1 (achado crítico).
- [ ] CA-03: sem confirmação de isolamento, código 4 e nenhum container criado (o teste lista containers antes e depois).
- [ ] CA-04: destino igual ao fingerprint de um ambiente configurado resulta em código 4.
- [ ] CA-08: falha injetada no meio da restauração não deixa container nem volume com o rótulo da execução.
- [ ] Runtime ausente, disco insuficiente ou major menor resultam em código 2, sem iniciar.
- [ ] Nenhum segredo em argumento de processo do container (credencial por arquivo montado ou variável), verificado via `ps`/`inspect` no teste.

**Arquivos.** `cmd/auditor/main.go`, `internal/restoreverify/{runtime,container,prevalidate,rto,evidence}.go`, `internal/cli/*`, `docs/backup-verifier.md`.
**Estimativa:** 6 d. **Gate:** **R2** (isolamento, manuseio de backup de produção e credenciais). **Depende de:** plano 01 lotes 1 e 3.

### Lote 2 — Verificações na cópia restaurada

**Issue:** `feat(restore-verify): inventário, contagens, integridade física (amcheck) e hash de amostra opt-in (RF-05, RNF-06)`

**Descrição.** (a) Inventário: rodar os coletores PostgreSQL existentes contra a cópia e comparar com a última coleta completa da origem (`internal/schemadiff` do plano 04 ou `internal/comparison`). Tabela, índice ou função ausente gera divergência alta. (b) Contagens exatas na cópia × linhas estimadas da origem, com tolerância (> 0,5% **e** > 1000 linhas gera média, **valor inicial, calibrar**), só em tabelas comparáveis. (c) **Opt-in (`--sample-hash`, decisão padrão nº 8):** hash agregado por tabela de uma amostra determinística (ordem pela PK, até 1000 linhas, até as 200 maiores tabelas), calculado na origem pela conexão só leitura (timeouts da coleta) e na cópia. Exigido só para tabelas sem escrita entre o backup e a verificação (contadores `n_tup_*` das coletas). As outras ficam "não comparável" (RNF-06). Hash nunca por linha. Sem opt-in, (c) aparece como "desabilitada", e isso **não** é tratado como execução parcial (ver §8). Com opt-in e origem inacessível, (c) fica "não executada" e o código é 3. (d) `CREATE EXTENSION amcheck` **na cópia** e verificação de B-tree e heap (`bt_index_check`, `verify_heapam`). Corrupção gera divergência crítica.

**Critérios de aceite.**
- [ ] CA-01: dump válido com confirmação, as quatro verificações executadas (com `--sample-hash`), container destruído e código 0 sem divergência.
- [ ] CA-05: tabela sem escrita desde o backup com amostra diferente gera `backup.restore_divergence` alta, com tabela e contagens, sem nenhum valor de linha (varredura da evidência e dos logs).
- [ ] Origem inacessível com `--sample-hash` resulta em (c) "não executada" e código 3.
- [ ] Os dados restaurados nunca saem do container. A CLI só lê contagens, hashes agregados e catálogo.
- [ ] Testes nos jobs PG 14–18 com `pg_dump`/`pg_restore` da mesma major, a mesma restrição já documentada para `TestRestoreDrillRoundTripIntegration` em `docs/ops.md`.

**Arquivos.** `internal/restoreverify/verify_*.go`, `internal/collectors/postgres/*` (reuso), `internal/comparison/compare.go` ou `internal/schemadiff`, `docs/backup-verifier.md`.
**Estimativa:** 6 d. **Gate:** **R2** (lê linhas da produção no opt-in; dado pessoal só em memória). **Depende de:** lote 1.

### Lote 3 — Evidência na API, política por ambiente e regras `backup.*`

**Issue:** `feat(restore-drills): evidência target_restore_drill por ambiente via token, RTO-alvo e prazo, achados backup.* e métricas (RF-06, RF-08, RF-09)`

**Descrição.** Migration `39_target_restore_drill.sql` (provisória). `restore-drill-context` e `POST .../restore-drills` com o escopo `restore-evidence:write`, acrescentado à lista fechada do plano 01. Revalidação do corpo e recusa de qualquer campo fora do contrato. RTO-alvo e prazo por ambiente (operator). Regras: `restore_failed` (crítica), `restore_divergence` (crítica, alta ou média, conforme a §3), `rto_exceeded` (alta) e `drill_overdue` (média, > 30 dias, **valor inicial, calibrar**; mesmo prazo do BA-03), avaliada na análise do ambiente. Métricas de idade e RTO por ambiente.

**Critérios de aceite.**
- [ ] CA-06: RTO-alvo de 10 min e medido de 14 min gera `backup.rto_exceeded` alta.
- [ ] CA-07: 31 dias sem ensaio bem-sucedido geram `backup.drill_overdue` média na análise.
- [ ] RNF-04: o token de A não envia para B (403). Viewer lê. Operator configura.
- [ ] A evidência do snapshot store (`retention_evidence`) não muda nem bloqueia a retenção (os testes de `retention_test.go` continuam verdes).
- [ ] RNF-05: `auditor_target_restore_drill_age_seconds{environment}` cresce sem ensaio.

**Arquivos.** `migrations/39_target_restore_drill.sql`, `internal/api/restore_drills.go`, `internal/api/auth.go`, `internal/repository/restore_drills.go`, `internal/analyzer/backup.go`, `internal/actions/catalog.go`, `internal/observability/metrics.go`, `api/openapi.json`, `docs/rules.md`.
**Estimativa:** 4 d. **Gate:** **R2** (nova escrita por token; isolamento). **Depende de:** lote 1 (o lote 2 alimenta as divergências).

### Lote 4 — UI Restauração verificada e seção do PDF

**Issue:** `feat(ui): área Restauração verificada com histórico, RTO × alvo e divergências; seção no PDF técnico e executivo (RF-07)`

**Descrição.** Tela desenhada com a Trinity, mais configuração de RTO e prazo (operator). Seção "Restauração verificada" nos PDFs técnico e executivo (último ensaio, RTO, divergências), respeitando a redação: nomes de tabela ocultos no modo estrito.

**Critérios de aceite.**
- [ ] Histórico, último sucesso, RTO × alvo e tendência visíveis. Divergência linka para o achado.
- [ ] Viewer não vê a configuração.
- [ ] Golden do PDF com e sem ensaio.
- [ ] Layout aprovado pela Trinity.

**Arquivos.** `frontend/src/pages/parts/RestoreDrillsCard.tsx`, `frontend/src/pages/StatusPage.tsx` ou `EnvironmentsPage.tsx`, `frontend/src/services/api.ts`, `internal/report/sections.go`, `internal/report/testdata/*`, `docs/guia-do-usuario.md`.
**Estimativa:** 4 d. **Gate:** CI verde + Trinity. **Depende de:** lote 3.

## 5. Estratégia de testes

| Teste | Tipo | Critério ligado |
|---|---|---|
| `TestRestoreVerifyHappyPath` (dump válido; quatro verificações com `--sample-hash`; container destruído) | integração com runtime de container no runner | CA-01 |
| `TestCorruptedDumpChecksumFails` | integração | CA-02 |
| `TestMissingIsolationConfirmationExit4NoContainer` | integração | CA-03 |
| `TestTargetMatchesConfiguredEnvironmentExit4` | unitário + integração | CA-04 |
| `TestSampleHashDivergenceNoRowValues` | integração (origem e cópia descartáveis) | CA-05, RNF-02 |
| `TestRTOExceededFinding` | API/integração | CA-06 |
| `TestDrillOverdueFinding` (31 dias) | integração | CA-07 |
| `TestMidRestoreFailureLeavesNoContainerOrVolume` | integração com falha injetada | CA-08 |
| `TestRestoreEvidenceTokenScopedToEnvironment` | API com `authMiddleware` | RNF-04 |
| `TestDiskAndMajorPrechecksExit2` | unitário | §4 Erros |
| Regressão dos testes de `internal/ops` (#106) | existentes | separação das evidências |

O CI precisa de runtime de container no runner. O job `Backend (Go)` já usa `docker run` no passo de limite de memória (PF-07), então os testes de integração deste plano entram nesse job e são pulados sem a variável explícita (padrão `AUDITOR_TEST_*`).

## 6. Gates

| Lote | Gate | Motivo |
|---|---|---|
| 1 | **R2** | isolamento, backup sensível e credenciais |
| 2 | **R2** | leitura de linhas da produção (opt-in) |
| 3 | **R2** | escrita por token por ambiente |
| 4 | CI verde + Trinity | UI e relatório |

## 7. Riscos

- O runner precisa de acesso ao backup de produção, um ativo sensível. Mitigação: runbook com runner dedicado, artefato cifrado (`age`) e nenhum dado saindo do container.
- Restaurações grandes custam tempo e disco. Mitigação: verificação de disco antes, teto de 2 h e RTO registrado mesmo em falha.
- A origem muda depois do backup e gera falsos positivos. Mitigação: só tabelas "comparáveis" pelos contadores de escrita (RNF-06).
- Diferença de versão entre `pg_restore` e o servidor (documentada em `docs/ops.md`). Mitigação: o `pg_restore` roda dentro da imagem escolhida.
- Container órfão por `kill -9` da CLI. Mitigação: rótulo por execução e varredura no início da próxima execução.

## 8. Pontos para Q

- O hash de amostra opt-in (**decisão padrão** nº 8) exige que o runner tenha DSN só leitura da produção. Isso contraria o espírito da **decisão padrão** nº 2 ("o runner nunca recebe a senha do banco", dita para a 01). A nº 2 vale também para a 08?
- Com (c) desligado por padrão, o CA-01 ("executa as quatro verificações") só passa com o opt-in. E a spec trata "(c) não executada" como código 3. O plano considera "(c) desabilitada" como código 0. Confirmar.
- O RF-03 pede recusa quando o destino coincide com um DSN de ambiente configurado, mas a CLI não tem esses DSNs. O plano usa fingerprints de host fornecidos pela API. Confirmar que expor esses hashes ao token é aceitável.

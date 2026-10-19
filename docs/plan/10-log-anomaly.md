# Plano 10 — Anomalias em logs estruturados (`log-anomaly`)

- **Spec:** [../spec/10-log-anomaly.md](../spec/10-log-anomaly.md)
- **Base lida:** `main` em `b1a93cf`
- **Posição na ordem de implementação:** 10ª (última)
- **Total:** 4 lotes no v1, **14 d**, mais 1 lote condicional (6 d). 2 lotes com gate R2 no v1 (+1 condicional). Trinity: não necessária no v1 (só CLI); necessária no lote condicional

## 0. Resumo

Um subcomando da CLI lê logs JSON por linha (arquivo, vários arquivos ou stdin), mascara as partes variáveis, agrupa em templates com id estável (estilo Drain), conta por janela e nível, calcula a linha de base e o z-score, e aponta anomalias. Tudo **local** no v1: nada vai para a API.

### Decisões padrão aplicadas

- **decisão padrão (Q, nº 10):** **logs do PostgreSQL** (perfil `postgres-jsonlog`, PG 15+) **e JSON genérico** (perfil `generico` com mapeamento de horário, nível e mensagem). Responde à P-01.
- **decisão padrão (Q, nº 10):** **processamento só local no início**. RF-07 (envio ao auditor e lista "Anomalias de log" na UI) e CA-06 ficam no **lote 5 condicional**, fora do v1. Responde à P-02.
- **decisão padrão (Q, nº 7):** o insumo de último login para a 07 é **gerado localmente** no v1 (lote 4). O consumo pelo auditor depende do lote 5 condicional e do lote 6 condicional do plano 07.
- **decisão padrão (Q, nº 1):** quando o envio existir (lote 5), ele usará o **token por ambiente** do plano 01, escopo `log-aggregate:write`.
- **decisão padrão (Q, nº 11):** tenant = ambiente (só no lote 5).

### Dependências

Plano 01, lote 1 (modo CLI e códigos). O lote 5 condicional também depende do plano 01, lote 3 (token).

## 1. Módulos e arquivos afetados

| Área | Caminho | Mudança |
|---|---|---|
| Leitura | `backend/internal/loganomaly/` (novo: `reader.go` em streaming com linha ≤ 64 KB, `profile_postgres.go`, `profile_generic.go`) | — |
| Mascaramento e templates | `loganomaly/mask.go`, `loganomaly/template.go` (árvore de prefixo estilo Drain, id = hash do template), reuso de `internal/fingerprint/literals.go` (extraído no plano 01, lote 2) | números, identificadores, literais entre aspas, IPv4/IPv6, e-mails, hashes, UUIDs, caminhos |
| Estatística | `loganomaly/window.go`, `loganomaly/baseline.go`, `loganomaly/anomaly.go` | — |
| Saída | `loganomaly/report.go` (terminal, Markdown, JSON com `schema_version`) via `backend/internal/cli` | — |
| Último login | `loganomaly/lastlogin.go` | — |
| CLI | `backend/cmd/auditor/main.go` | `auditor log-anomaly` |
| Sanitização | `backend/internal/config/sanitize.go` | reuso para as mensagens de erro |
| Condicional (lote 5) | `backend/internal/api/log_anomalies.go`, `backend/internal/repository/log_anomalies.go`, `backend/internal/api/auth.go` (escopo `log-aggregate:write`), `frontend/src/pages/LogAnomaliesPage.tsx` | — |
| Docs | novo `docs/log-anomaly.md` (perfis, mapeamento genérico, limites, garantias LGPD), `docs/guia-do-usuario.md` (só no lote 5) | — |

## 2. Migrations (provisórias)

| Nº provisório | Arquivo | Lote | Conteúdo |
|---|---|---|---|
| — | — | 1–4 | **nenhuma no v1** (processamento local) |
| 40 (condicional) | `backend/migrations/40_log_anomaly.sql` | 5 | `log_aggregate_batch` (ambiente, id do token, período, versão do esquema, linhas lidas/descartadas/inválidas), `log_template` (ambiente, id do template, template mascarado, nível), `log_window_count`, `log_anomaly`. **Sem linhas brutas nem exemplos** |

O 40 segue o 39 do plano 08. O 41 (plano 07, condicional) vem depois.

## 3. Endpoints e telas

v1: **nenhum endpoint e nenhuma tela** (só CLI). **Trinity não é necessária no v1.**

Condicional (lote 5):

| Método e rota | Quem | Observação |
|---|---|---|
| `POST /api/v1/environments/{id}/log-aggregates` | token `log-aggregate:write` | token de outro ambiente: 403 (CA-06). Tamanho limitado |
| `GET /api/v1/environments/{id}/log-anomalies`, `GET .../log-templates` | viewer | — |
| `POST /api/v1/environments/{id}/account-last-logins` | token `log-aggregate:write` | só role e horário, para o plano 07 (lote 6) |

Tela condicional: lista "Anomalias de log" por ambiente (Trinity).

## 4. Lotes

### Lote 1 — Leitura em streaming e perfis de campo

**Issue:** `feat(log-anomaly): auditor log-anomaly lê JSON por linha de arquivos ou stdin com perfis postgres-jsonlog e genérico (RF-01, RNF-03)`

**Descrição.** Leitura em streaming com memória limitada (padrão 256 MB). Linha acima de 64 KB é descartada e contada. JSON inválido é contado e não interrompe. Perfil `postgres-jsonlog` (campos `timestamp`, `error_severity`, `message` do jsonlog do PG 15+). Perfil `generico` com `--time-field`, `--level-field`, `--message-field`. Arquivo inexistente ou ilegível sai com código 2. Perfil sem campo de horário também. Mais de 10% de linhas inválidas sai com código 3. Relatório de linhas lidas, descartadas e inválidas (RNF-05).

**Critérios de aceite.**
- [ ] CA-04: 20% de linhas não JSON. Processa o resto, informa as descartadas e sai com código 3.
- [ ] Código 2 para arquivo inexistente e para perfil sem horário.
- [ ] RNF-03: ≥ 50 MB/s em benchmark e memória abaixo do teto (teste com arquivo sintético).
- [ ] A CLI não abre conexão com banco nem com rede (RNF-01).

**Arquivos.** `cmd/auditor/main.go`, `internal/loganomaly/reader.go`, `internal/loganomaly/profile_*.go`, `docs/log-anomaly.md`.
**Estimativa:** 3 d. **Gate:** CI verde (sem dado persistido nem enviado). **Depende de:** plano 01 lote 1.

### Lote 2 — Mascaramento e templates com id estável

**Issue:** `feat(log-anomaly): mascaramento de partes variáveis e agrupamento em templates estilo Drain com id estável e regra não classificável (RF-02, RNF-02, RNF-06)`

**Descrição.** Mascarar números, identificadores, literais entre aspas e `$tag$`, IPs, e-mails, hashes, UUIDs, caminhos e parâmetros `user=`/`database=`, partindo de `internal/fingerprint/literals.go`. Template inteiro variável fica "não classificável" e não é exibido. Id = hash do template mascarado. No máximo 10 000 templates, com o excedente em "outros" e aviso. A mesma entrada com a mesma configuração dá os mesmos ids e anomalias. Linhas brutas e exemplos nunca saem da memória (nem para log de depuração).

**Critérios de aceite.**
- [ ] CA-02: linhas com e-mails, IPs e parâmetros de consulta não deixam nenhum desses valores na saída (terminal, Markdown, JSON). O teste usa um corpus com valores conhecidos e procura cada um.
- [ ] CA-05: duas execuções sobre a mesma entrada dão JSON idêntico byte a byte.
- [ ] Template totalmente variável sai como "não classificável".
- [ ] Fuzz do mascarador: nenhum e-mail ou IP gerado aleatoriamente sobrevive.

**Arquivos.** `internal/loganomaly/mask.go`, `internal/loganomaly/template.go`, `internal/fingerprint/literals.go`.
**Estimativa:** 5 d. **Gate:** **R2** (mascaramento é a garantia LGPD crítica da feature). **Depende de:** lote 1.

### Lote 3 — Janelas, linha de base, z-score, anomalias e gate

**Issue:** `feat(log-anomaly): contagem por janela e nível, base por K janelas ou referência, z-score, anomalias e saída Markdown/JSON com gate (RF-03–RF-06)`

**Descrição.** Janela padrão de 5 min. Base de K = 12 janelas, ou arquivo ou período de referência. Anomalias da §3: pico (z ≥ 3 e ≥ 20), pico forte (≥ 8× e ≥ 50), template novo (≥ 10), pico de erros (≥ 5×) e template que sumiu. Com menos de 6 janelas, base insuficiente: só templates novos, com confiança baixa, e código 3 quando a base não basta para picos. Todos os limiares são **valor inicial, calibrar**. Gate a partir de alta (`--fail-on high`). Saída com resumo, tabela e ressalvas.

**Critérios de aceite.**
- [ ] CA-01: 10 ocorrências por janela de "connection authorized: user=? database=?" e uma janela com 90: pico forte (9×) com template mascarado.
- [ ] CA-03: template de nível erro inédito com 15 ocorrências numa janela: template novo médio.
- [ ] Base insuficiente sai com código 3 e ressalva.
- [ ] Gate em alta: pico de erros sai com código 1.

**Arquivos.** `internal/loganomaly/window.go`, `internal/loganomaly/baseline.go`, `internal/loganomaly/anomaly.go`, `internal/loganomaly/report.go`.
**Estimativa:** 4 d. **Gate:** CI verde. **Depende de:** lote 2.

### Lote 4 — Insumo local de último login por role

**Issue:** `feat(log-anomaly): arquivo local de último login por role (só role e horário) a partir de eventos de conexão do postgres-jsonlog (RF-08)`

**Descrição.** `--last-login-out ultimo-login.json`. Dos eventos de conexão autorizada do perfil `postgres-jsonlog`, extrair **só** o nome da role e o horário mais recente, sem IP, banco de origem nem aplicação. Arquivo local, com `schema_version`. Pela **decisão padrão** nº 10, não é enviado no v1.

**Critérios de aceite.**
- [ ] CA-07: o insumo contém só a role e o horário mais recente, sem IP nem banco (teste inspeciona o JSON).
- [ ] Perfil genérico: a opção é recusada com código 2 (só faz sentido para `postgres-jsonlog`).

**Arquivos.** `internal/loganomaly/lastlogin.go`, `docs/log-anomaly.md`.
**Estimativa:** 2 d. **Gate:** **R2** (dado pessoal: nome de role e horário de autenticação). **Depende de:** lote 2.

### Lote 5 (condicional, fora do v1) — Envio de agregados ao auditor e UI

**Issue:** `feat(log-anomaly): envio opcional de agregados e de último login ao ambiente, lista Anomalias de log na UI (RF-07, CA-06)`

**Descrição.** Só depois que Q revogar ou ajustar a **decisão padrão** nº 10. Migration `40_log_anomaly.sql` (provisória). `--submit` com o token `log-aggregate:write`: só templates mascarados, ids, contagens por janela e anomalias. Envio rejeitado (credencial, escopo, tamanho) sai com código 2, e a saída local é preservada. A API recusa campos fora do contrato e revalida no servidor que nenhum template contém padrão de e-mail ou IP (defesa em profundidade). UI "Anomalias de log" (Trinity). Endpoint de último login para o lote 6 do plano 07. Métricas de lotes recebidos e rejeitados (RNF-05).

**Critérios de aceite.**
- [ ] CA-06: credencial de outro ambiente: 403 na API e código 2 na CLI.
- [ ] CA-02 aplicado ao lote enviado: nenhum valor no corpo nem no store.
- [ ] Layout aprovado pela Trinity.

**Estimativa:** 6 d. **Gate:** **R2** (escrita por token, dado derivado de log). **Depende de:** lotes 1–4 e plano 01 lote 3.

## 5. Estratégia de testes

| Teste | Tipo | Critério ligado |
|---|---|---|
| `TestStrongSpikeConnectionAuthorized` (10 → 90, 9×) | unitário com corpus sintético | CA-01 |
| `TestMaskingRemovesEmailsIPsParams` + `FuzzMasking` | unitário + fuzz | CA-02, RNF-02 |
| `TestNewErrorTemplateMedium` | unitário | CA-03 |
| `TestInvalidLinesExit3` (20% não JSON) | CLI | CA-04 |
| `TestDeterministicJSONOutput` | golden | CA-05, RNF-06 |
| `TestSubmitOtherEnvironment403Exit2` | API + CLI (**lote 5 condicional**) | CA-06 |
| `TestLastLoginOnlyRoleAndTime` | unitário | CA-07, RF-08 |
| `TestReaderThroughputAndMemory` | benchmark | RNF-03 |
| `TestTemplateCapOthers` (10 000) | unitário | RNF-03 |
| `TestNoNetworkNoDatabase` | unitário (sem rede no processo de teste) | RNF-01 |

## 6. Gates

| Lote | Gate | Motivo |
|---|---|---|
| 1 | CI verde | leitura local |
| 2 | **R2** | mascaramento (LGPD crítico) |
| 3 | CI verde | estatística sobre templates mascarados |
| 4 | **R2** | último login (dado pessoal) |
| 5 (cond.) | **R2** | envio por token |

## 7. Riscos

- Mascaramento incompleto vaza dado pessoal. Mitigação: padrões testados, fuzz, regra "não classificável" e gate R2.
- É a feature menos alinhada ao foco em banco e pode competir por esforço. Mitigação: última na ordem e v1 só CLI.
- Os templates do PostgreSQL mudam entre versões (14–18). Mitigação: golden por versão das mensagens de conexão mais comuns.
- O perfil genérico aceita logs de aplicação com dado pessoal arbitrário. Mitigação: o mesmo mascarador e a mesma regra "não classificável", e nada sai da máquina no v1.

## 8. Pontos para Q

- A **decisão padrão** nº 10 ("só local no início") tira do v1 o RF-07, o CA-06, o RNF-04 e a parte de API/UI do §4, e impede que o último login chegue à 07 (**decisão padrão** nº 7). Decidir se o v1 inclui pelo menos o envio do último login (só role e horário) ou se tudo espera o lote 5.

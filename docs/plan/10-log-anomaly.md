# Plano 10 — Anomalias em logs estruturados (`log-anomaly`)

- **Spec:** [../spec/10-log-anomaly.md](../spec/10-log-anomaly.md)
- **Base lida:** `main` em `b1a93cf`
- **Posição na ordem de implementação:** 10ª (última)
- **Decisões:** [ADR-0001](../adr/0001-decisoes-v1-features.md) (Aceita, 2026-10-18)
- **Total:** 5 lotes, **16 d**, 3 lotes com gate R2. Trinity não necessária (só CLI; a exibição do último login fica na tela do plano 07)

## 0. Resumo

Um subcomando da CLI lê logs JSON por linha (arquivo, vários arquivos ou stdin), mascara as partes variáveis, agrupa em templates com id estável (estilo Drain), conta por janela e nível, calcula a linha de base e o z-score, e aponta anomalias. Tudo é **local** no v1. A **única exceção** é o agregado de último login por role (só role e horário), que a CLI pode enviar ao ambiente (lote 5) para a revisão de acessos da 07.

### Decisões aplicadas (ADR-0001)

- **ADR-0001 D10:** **logs do PostgreSQL** (perfil `postgres-jsonlog`, PG 15+) **e JSON genérico** (perfil `generico` com mapeamento de horário, nível e mensagem). Responde à P-01.
- **ADR-0001 D10 (emendada):** **processamento local**. Templates, contagens e anomalias **não** sobem à API no v1, e a lista "Anomalias de log" na UI fica fora do v1 (RF-07). **Única exceção:** o agregado de último login por role (role e horário mais recente, nada mais) sobe ao ambiente no lote 5. Responde à P-02.
- **ADR-0001 D7:** o agregado alimenta a revisão de acessos (plano 07, lote 6, que tem a rota e a tabela).
- **ADR-0001 D1:** o envio usa o **token por ambiente** do plano 01, escopo `last-login:write`.
- **ADR-0001 D11:** tenant = ambiente. O envio só escreve no ambiente do token (CA-06, RNF-04).
- **ADR-0001 D16:** o mascaramento parte de `internal/fingerprint/literals.go`, extraído no plano 01, lote 2.

### Dependências

Plano 01, lote 1 (modo CLI e códigos). O lote 5 também depende do plano 01, lote 3 (token) e do plano 07, lote 6 (rota que recebe o último login). Os dois lotes (10 L5 e 07 L6) são implementados em sequência, depois do lote 4 deste plano.

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
| Envio do último login (lote 5) | novo `loganomaly/submit.go`, `backend/internal/cli/apiclient.go` (plano 01) | `--submit-last-login` para `POST .../account-last-logins` (rota do plano 07, lote 6) |
| Docs | novo `docs/log-anomaly.md` (perfis, mapeamento genérico, limites, garantias LGPD, envio do último login) | — |

## 2. Migrations (provisórias)

| Nº provisório | Arquivo | Lote | Conteúdo |
|---|---|---|---|
| — | — | 1–5 | **nenhuma** (processamento local; a tabela do último login é a `40_account_last_login.sql` do plano 07, lote 6) |

Tabelas de agregados de anomalia (`log_template`, `log_anomaly` etc.) ficam fora do v1 (ADR-0001 D10).

## 3. Endpoints e telas

Este plano **não cria endpoint nem tela**. A CLI consome uma rota criada no plano 07, lote 6:

| Método e rota | Quem | Observação |
|---|---|---|
| `POST /api/v1/environments/{id}/account-last-logins` | token `last-login:write` | só `role` e `last_login_at`. Token de outro ambiente: 403 (CA-06). Campo extra: 400. Tamanho limitado |

**Trinity não é necessária.** O último login aparece na matriz da revisão de acessos (plano 07, lote 5). Fora do v1 (ADR-0001 D10): envio de templates e anomalias e lista "Anomalias de log".

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

**Descrição.** `--last-login-out ultimo-login.json`. Dos eventos de conexão autorizada do perfil `postgres-jsonlog`, extrair **só** o nome da role e o horário mais recente, sem IP, banco de origem nem aplicação. Arquivo local, com `schema_version`, no mesmo formato que o lote 5 envia (ADR-0001 D10).

**Critérios de aceite.**
- [ ] CA-07: o insumo contém só a role e o horário mais recente, sem IP nem banco (teste inspeciona o JSON).
- [ ] Perfil genérico: a opção é recusada com código 2 (só faz sentido para `postgres-jsonlog`).

**Arquivos.** `internal/loganomaly/lastlogin.go`, `docs/log-anomaly.md`.
**Estimativa:** 2 d. **Gate:** **R2** (dado pessoal: nome de role e horário de autenticação). **Depende de:** lote 2.

### Lote 5 — Envio do agregado de último login ao ambiente

**Issue:** `feat(log-anomaly): --submit-last-login envia ao ambiente só role e horário mais recente, com token last-login:write (RF-08, CA-06, ADR-0001 D10)`

**Descrição.** `--submit-last-login --environment <id>` envia o conteúdo do arquivo do lote 4 (só `role` e `last_login_at`) para `POST .../account-last-logins` (plano 07, lote 6). Autenticação: token do ambiente em `AUDITOR_CLI_TOKEN`/`_FILE`, escopo `last-login:write`. **Nada mais sai da máquina:** a CLI monta o corpo a partir de uma struct fechada, sem templates, contagens, IP, banco ou aplicação. Envio rejeitado (credencial, escopo, tamanho) sai com código 2, e a saída local é preservada. Só para o perfil `postgres-jsonlog`.

**Critérios de aceite.**
- [ ] CA-06: credencial de outro ambiente: 403 na API e código 2 na CLI.
- [ ] CA-02 e CA-07 aplicados ao envio: o corpo capturado num servidor falso tem só `role` e `last_login_at`, e nenhum e-mail, IP ou parâmetro do corpus aparece.
- [ ] A CLI não faz nenhuma outra chamada de rede além desse POST (teste com servidor falso que conta requisições).
- [ ] Sem `--submit-last-login`, nenhuma conexão de rede (RNF-01 continua valendo).

**Arquivos.** `internal/loganomaly/submit.go`, `internal/cli/apiclient.go`, `cmd/auditor/main.go`, `docs/log-anomaly.md`.
**Estimativa:** 2 d. **Gate:** **R2** (dado pessoal de autenticação sai da máquina; uso de token). **Depende de:** lote 4, plano 01 lote 3 e plano 07 lote 6.

## 5. Estratégia de testes

| Teste | Tipo | Critério ligado |
|---|---|---|
| `TestStrongSpikeConnectionAuthorized` (10 → 90, 9×) | unitário com corpus sintético | CA-01 |
| `TestMaskingRemovesEmailsIPsParams` + `FuzzMasking` | unitário + fuzz | CA-02, RNF-02 |
| `TestNewErrorTemplateMedium` | unitário | CA-03 |
| `TestInvalidLinesExit3` (20% não JSON) | CLI | CA-04 |
| `TestDeterministicJSONOutput` | golden | CA-05, RNF-06 |
| `TestSubmitLastLoginOtherEnvironment403Exit2` | API + CLI (lote 5, com a rota do plano 07 lote 6) | CA-06 |
| `TestSubmitLastLoginBodyOnlyRoleAndTime` (servidor falso inspeciona o corpo e conta requisições) | CLI | CA-02, CA-07, ADR-0001 D10 |
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
| 5 | **R2** | dado pessoal sai da máquina por token |

## 7. Riscos

- Mascaramento incompleto vaza dado pessoal. Mitigação: padrões testados, fuzz, regra "não classificável" e gate R2.
- É a feature menos alinhada ao foco em banco e pode competir por esforço. Mitigação: última na ordem e v1 só CLI.
- Os templates do PostgreSQL mudam entre versões (14–18). Mitigação: golden por versão das mensagens de conexão mais comuns.
- O perfil genérico aceita logs de aplicação com dado pessoal arbitrário. Mitigação: o mesmo mascarador e a mesma regra "não classificável", e nada do perfil genérico sai da máquina no v1.
- O envio do último login é o único caminho de saída e pode crescer por pressão de escopo. Mitigação: corpo de struct fechada, recusa de campos extras no servidor e gate R2 (ADR-0001 D10).

## 8. Pontos para Q (resolvidos)

- ~~Envio mínimo do último login × "só local".~~ **Resolvido por ADR-0001 D10 (emendada):** no v1 sobe só o agregado de role e horário (lote 5, não mais condicional). O RF-07 (envio de anomalias e UI) fica fora do v1. O CA-06 e o RNF-04 da spec passam a valer para o envio do último login.

# Plano 05 — Mapa de dados pessoais LGPD (`pii-scanner`)

- **Spec:** [../spec/05-pii-scanner.md](../spec/05-pii-scanner.md)
- **Base lida:** `main` em `b1a93cf`
- **Posição na ordem de implementação:** 6ª
- **Total:** 6 lotes, **24 d**, 4 lotes com gate R2, Trinity necessária (1 lote)

## 0. Resumo

A classificação acontece em duas etapas. Na etapa 1, só metadados: nome e tipo de coluna, com dicionário editável. Na etapa 2, opcional e restrita, há validação de conteúdo em memória com detectores brasileiros (CPF, CNPJ, e-mail, telefone, CEP, PIX). O resultado é cruzado com a matriz de privilégio efetivo da 07. Nenhum valor amostrado é persistido, logado ou exportado: nem mascarado, nem com hash (RNF-02). Este plano também endurece a amostragem por páginas de `internal/quality`, que a 03 reaproveita.

### Decisões padrão aplicadas

- **decisão padrão (Q, nº 4):** a etapa 2 de conteúdo tem **flag própria, mais restrita** que a do diagnóstico de qualidade: `AUDITOR_PII_CONTENT_SCAN_ENABLED=1` no processo, **mais** habilitação explícita por ambiente feita pelo operator. `AUDITOR_DATA_QUALITY_ENABLED` não basta. Responde à P-01.
- **decisão padrão (Q, nº 11):** tenant = ambiente. O mapa pertence ao ambiente e é informação sensível. Ver exige viewer, e baixar exige auditor (RNF-04).
- Fora do v1 (P-02 sem decisão; já fora de escopo no MVP pela spec): JSONB e texto livre.

### Dependências

Plano 07, lote 1 (origem por privilégio e privilégio por coluna). Sem ele, o lote 4 usa `grant_snapshot` por tabela e marca a evidência "nível tabela" (§6 da spec). É **pré-requisito** do lote 3 do plano 03 (amostragem por páginas endurecida).

## 1. Módulos e arquivos afetados

| Área | Caminho | Mudança |
|---|---|---|
| Classificação | `backend/internal/pii/` (novo: `dictionary.go`, `categories.go`, `detect_cpf.go`, `detect_cnpj.go`, `detect_email.go`, `detect_phone.go`, `detect_cep.go`, `detect_pix.go`, `confidence.go`, `scan.go`) | detectores puros, sem I/O |
| Amostragem segura | `backend/internal/quality/probe.go` (+ novo `quality/sample.go`) | amostragem por páginas (`TABLESAMPLE SYSTEM` com teto de linhas), transação `READ ONLY`, timeout por coluna, valores só em memória. API compartilhada com a 03 |
| Conexão | `backend/internal/collectors/postgres/readonly_connection.go`, `config/targets.go` | reuso (conta sem escrita, allowlist, TLS) |
| Store | `backend/internal/repository/pii.go` (novo), `repository/discard.go` (supressão com justificativa) | — |
| Regras | `backend/internal/analyzer/pii.go` (novo), `analyzer/security.go` (reuso de `security.powerful_role`), `backend/internal/actions/catalog.go` | `pii.column_detected`, `pii.broad_read_access`, `pii.no_rls_hypothesis` |
| API e RBAC | `backend/internal/api/pii.go` (novo), `backend/internal/api/auth.go` (`/pii-scans` e `/pii-classifications/` exigem auditor) | — |
| Capacidades | `backend/internal/api/capabilities.go`, `backend/internal/capabilities/manifest.go` | expor a etapa 2 habilitada ou não (como faz hoje com `AUDITOR_DATA_QUALITY_ENABLED`) |
| PDF e CSV | `backend/internal/report/sections.go`, `report/sensitive.go`, `report/redaction.go` | — |
| Frontend | `frontend/src/pages/PersonalDataPage.tsx` (novo), `services/api.ts`, `components/layout/Shell.tsx` | — |
| Docs | `docs/rules.md`, `docs/ops.md` (flag e habilitação), `docs/hardening.md` (garantias LGPD), `docs/guia-do-usuario.md`, `.env.example`, `deploy/env.prod.example` | — |

## 2. Migrations (provisórias)

| Nº provisório | Arquivo | Lote | Conteúdo |
|---|---|---|---|
| 35 | `backend/migrations/35_pii_scanner.sql` | 1 | `pii_dictionary_entry` (ambiente, padrão de nome, categoria, ativo), `pii_environment_setting` (ambiente, `content_scan_enabled`, `enabled_by`, `enabled_at`), `pii_scan` (ambiente, `audit_run_id` de referência, etapas, estado `running`/`success`/`partial_success`/`failed`, cobertura, o que faltou), `pii_column_result` (coluna qualificada, categoria, confiança, método, tamanho da amostra, contagem e % validado, % nulo; **nenhuma coluna de valor**), `pii_classification_review` (confirmação, com descarte via supressão existente) |

Os números supõem 26–34 já mergeados. Lembrar `ci.yml`, `migrations/README.md` e `docs/ops.md`.

## 3. Endpoints e telas

| Método e rota | Papel | Observação |
|---|---|---|
| `POST /api/v1/environments/{id}/pii-scans` (`stages: [1]` ou `[1,2]`) | auditor | etapa 2 com a flag desligada ou o ambiente não habilitado: 409 |
| `GET /api/v1/environments/{id}/pii-scans`, `GET .../{scan}` | viewer | — |
| `GET /api/v1/environments/{id}/pii-map?category=&confidence=&limit=&offset=` | viewer | mapa com contas com leitura e RLS |
| `PATCH /api/v1/environments/{id}/pii-classifications/{column}` | auditor | confirmar ou descartar com justificativa |
| `GET`/`PUT /api/v1/environments/{id}/pii-dictionary` | viewer / operator | — |
| `PUT /api/v1/environments/{id}/pii-settings` | operator | liga ou desliga a etapa 2 no ambiente |
| `GET /api/v1/environments/{id}/pii-map/export/download?format=csv` | auditor | — |

**Tela (Trinity necessária).** A spec diz "nova aba Dados pessoais em Segurança". Não há seção Segurança na navegação, então a Trinity define o lugar. A tela tem mapa por tabela/coluna (categoria, confiança, método, contas com leitura, RLS), filtros, confirmar/descartar com justificativa, o estado "não aplicável a este mecanismo" (MongoDB) e uma indicação clara de que a etapa 2 está desligada.

## 4. Lotes

### Lote 1 — Etapa 1: classificação por metadados e dicionário

**Issue:** `feat(pii): mapa de dados pessoais por nome e tipo de coluna, com dicionário editável por ambiente (spec 05 RF-01, RF-03)`

**Descrição.** Migration `35_pii_scanner.sql` (provisória). Heurística de nome e tipo sobre `column_snapshot` (cpf, cnpj, email, telefone/celular, cep, pix, nome, nascimento, endereço, rg), com sinônimos em pt-BR e en, e `column_stat_snapshot` para % nulo. Nenhuma linha é lida. Confiança "baixa" só pelo nome, ou "média" com nome e tipo compatível (§3). O dicionário é editável pelo operator. MongoDB fica "não aplicável". `authorizeRequest` passa a exigir auditor para `POST .../pii-scans` e `PATCH .../pii-classifications/`.

**Critérios de aceite.**
- [ ] CA-06: ambiente MongoDB responde e mostra "não aplicável a este mecanismo".
- [ ] A etapa 1 não abre conexão com o alvo (teste com DSN inválido continua passando).
- [ ] Viewer recebe 403 ao iniciar a varredura. Auditor recebe 202 ou 200.
- [ ] Allowlist e denylist de schemas respeitadas.

**Arquivos.** `migrations/35_pii_scanner.sql`, `internal/pii/dictionary.go`, `internal/pii/categories.go`, `internal/pii/scan.go`, `internal/repository/pii.go`, `internal/api/pii.go`, `internal/api/auth.go`, `api/openapi.json`.
**Estimativa:** 4 d. **Gate:** **R2** (o mapa é informação sensível; mudança em `authorizeRequest`). **Depende de:** —

### Lote 2 — Detectores brasileiros e regra de confiança

**Issue:** `feat(pii): detectores CPF/CNPJ com dígito verificador, e-mail, telefone BR, CEP e chave PIX, com regra de confiança (spec 05 RF-02, §3)`

**Descrição.** Funções puras: CPF (DV, rejeita sequências repetidas), CNPJ (DV; o formato alfanumérico da Receita Federal é ponto aberto, ver §8), e-mail, telefone BR fixo e celular com ou sem DDI, CEP, PIX (CPF, CNPJ, e-mail, telefone ou chave aleatória UUID). A confiança segue a §3: alta com ≥ 90% válidos e ≥ 50 não nulos, média de 50 a 90%, baixa abaixo de 50 não nulos. As funções nunca devolvem nem logam o valor, só `bool`/contadores.

**Critérios de aceite.**
- [ ] CA-03: coluna `cpf` com 80% sem DV válido fica com confiança média, não alta.
- [ ] Testes de tabela com casos válidos, inválidos e de borda por detector, mais fuzz curto.
- [ ] Nenhuma função exporta o valor, o que é verificado por revisão de assinatura e teste de que o erro não contém a entrada.

**Arquivos.** `internal/pii/detect_*.go`, `internal/pii/confidence.go`, testes.
**Estimativa:** 3 d. **Gate:** CI verde (funções puras, sem dado real). **Depende de:** —

### Lote 3 — Etapa 2: validação de conteúdo com amostragem segura

**Issue:** `feat(pii): validação de conteúdo opt-in com flag própria, amostragem por páginas e garantia de que nenhum valor sai da memória (spec 05 RF-02, RNF-01–RNF-03, RNF-05)`

**Descrição.** Implementa a **decisão padrão** nº 4: `AUDITOR_PII_CONTENT_SCAN_ENABLED=1` **e** `pii_environment_setting.content_scan_enabled = true`, senão 409. Endurecer `internal/quality` com uma API de amostragem compartilhada (`quality/sample.go`): por páginas em tabela grande, transação `READ ONLY`, timeout por coluna, conta de leitura sem escrita (mesma verificação do diagnóstico atual) e valores processados em *streaming* pelo detector e descartados. Limites: 1000 linhas por tabela, 50 colunas candidatas, 200 tabelas, 10 min por execução (**valor inicial, calibrar**). Ao atingir o teto, a execução fica `partial_success`, com o que faltou. Sem SELECT na tabela, a coluna fica "não verificada — sem permissão", com cobertura parcial. Métricas de colunas por método e de execuções interrompidas.

**Critérios de aceite.**
- [ ] CA-01: coluna `documento` com 95% de CPFs válidos em 500 não nulos fica CPF, confiança alta, método "conteúdo".
- [ ] CA-02: teste de redação no padrão dos testes de `internal/quality`. Depois de uma varredura com CPFs sintéticos conhecidos, procurar cada CPF (completo, só dígitos, mascarado `***.***.***-**` com os dígitos parciais e sha256/md5) no snapshot store (todas as tabelas), nos logs capturados, na evidência dos achados, no CSV, nas mensagens de erro e em `/metrics`. Zero ocorrências.
- [ ] CA-05: com a etapa 2 desabilitada (flag ou ambiente), o auditor recebe 409 e a etapa 1 continua possível.
- [ ] Teto de 10 min: `partial_success`, com lista do que faltou.
- [ ] A amostragem compartilhada mantém o comportamento atual de `POST .../quality-scans` (os testes existentes continuam verdes).

**Arquivos.** `internal/quality/sample.go`, `internal/quality/probe.go`, `internal/pii/scan.go`, `internal/api/pii.go`, `internal/api/capabilities.go`, `internal/observability/metrics.go`, `.env.example`, `deploy/env.prod.example`, `docs/hardening.md`, `docs/ops.md`.
**Estimativa:** 6 d. **Gate:** **R2** (lê linhas reais com dado pessoal). **Depende de:** lotes 1 e 2.

### Lote 4 — Cruzamento com acesso, achados e revisão humana

**Issue:** `feat(pii): cruzamento com privilégio efetivo, RLS e roles poderosas; achados pii.* e confirmação/descarte (spec 05 RF-04–RF-06)`

**Descrição.** Para cada coluna classificada, levantar as contas com LOGIN que têm SELECT efetivo, usando a origem e a coluna do plano 07 (lote 1) ou `grant_snapshot` com a marca "nível tabela", mais RLS (`rls_policy_snapshot`) e poderosas (`security.powerful_role`, `pg_read_all_data`, excluindo a conta do auditor). Regras com os limiares da §3 (**valor inicial, calibrar**): `pii.column_detected` (informativa), `pii.broad_read_access` (média; alta para CPF/CNPJ/PIX) com a lista de contas na evidência, e `pii.no_rls_hypothesis` (baixa, hipótese). A confirmação usa `pii_classification_review`. O descarte usa a supressão existente com justificativa e não reabre sem mudança de evidência. A assinatura de descarte exclui campos voláteis, padrão `evidence.impact_basis`.

**Critérios de aceite.**
- [ ] CA-04: CPF de confiança alta legível por 7 contas com LOGIN gera `pii.broad_read_access` alta, com a lista das contas na evidência.
- [ ] CA-07: um descarte com justificativa é mantido na próxima varredura com a mesma evidência.
- [ ] Sem o plano 07, a evidência traz "nível tabela".
- [ ] Regras com os cinco campos e consulta segura no catálogo.

**Arquivos.** `internal/analyzer/pii.go`, `internal/analyzer/registry.go`, `internal/actions/catalog.go`, `internal/repository/pii.go`, `internal/repository/discard.go`, `docs/rules.md`.
**Estimativa:** 4 d. **Gate:** **R2** (exposição de dado pessoal × contas). **Depende de:** lote 3 e plano 07 lote 1.

### Lote 5 — Aba Dados pessoais

**Issue:** `feat(ui): aba Dados pessoais com mapa, filtros, contas com leitura, RLS e confirmar/descartar`

**Descrição.** Tela desenhada com a Trinity. Mapa, filtros por categoria e confiança, contas com leitura e RLS, ações confirmar e descartar com justificativa (auditor), indicação da etapa 2 desligada e "não aplicável" para MongoDB. Nenhum valor exibido, porque a API não os tem.

**Critérios de aceite.**
- [ ] CA-06 na UI.
- [ ] CA-05 na UI: a opção "validar conteúdo" fica desabilitada com o motivo quando a etapa 2 está desligada.
- [ ] Viewer não vê ações de confirmação ou descarte.
- [ ] Layout aprovado pela Trinity.

**Arquivos.** `frontend/src/pages/PersonalDataPage.tsx`, `frontend/src/services/api.ts`, `frontend/src/components/layout/Shell.tsx`, `frontend/src/app.tsx`, `docs/guia-do-usuario.md`.
**Estimativa:** 4 d. **Gate:** CI verde + Trinity. **Depende de:** lote 4.

### Lote 6 — Exportação CSV e seção "Dados pessoais" no PDF

**Issue:** `feat(pii): export CSV do mapa (download auditor) e seção Dados pessoais no PDF com redação (spec 05 RF-07)`

**Descrição.** CSV com as colunas do §4 (coluna qualificada, categoria, confiança, método, % validado, amostra, nº de contas com leitura, RLS), terminando em `# export_complete`, com download só para auditor. Seção do PDF técnico com totais por categoria. O detalhe por coluna sai só "sem redação". Nos modos "identificadores" e "estrita", só contagens (`report/sensitive.go`).

**Critérios de aceite.**
- [ ] CA-02 (CSV e PDF): nenhum valor de CPF no CSV nem no PDF (mesma varredura do lote 3).
- [ ] Viewer recebe 403 no download.
- [ ] Golden do PDF nos três modos de redação.

**Arquivos.** `internal/api/pii.go`, `internal/report/sections.go`, `internal/report/sensitive.go`, `internal/report/redaction.go`, `internal/report/testdata/*`.
**Estimativa:** 3 d. **Gate:** **R2** (exportação de mapa sensível). **Depende de:** lote 4.

## 5. Estratégia de testes

| Teste | Tipo | Critério ligado |
|---|---|---|
| `TestContentScanClassifiesCPFHighConfidence` (500 não nulos, 95% válidos) | integração no alvo descartável | CA-01 |
| `TestNoSampledValueAnywhere` (store, logs, evidência, CSV, PDF, erros, `/metrics`; completo, mascarado, hash) | integração (padrão dos testes de redação de `internal/quality`) | CA-02, RNF-02, RNF-05 |
| `TestCPFWithoutValidDVIsMedium` | unitário | CA-03 |
| `TestBroadReadAccessSevenLogins` | integração (com o lote 1 do plano 07) | CA-04 |
| `TestContentStageDisabled409` (flag de processo e ambiente) | API | CA-05, decisão padrão nº 4 |
| `TestPIIMongoNotApplicable` | API + componente | CA-06 |
| `TestDiscardPersistsAcrossScans` | integração | CA-07, RF-06 |
| `TestPIIScanCeilingPartialSuccess` | integração | RNF-03, §4 Erros |
| `TestPIIScanRequiresAuditor`, `TestPIIExportRequiresAuditor` | API com `authMiddleware` | RNF-04 |
| `FuzzBrazilianDetectors` | fuzz | RF-02 |

## 6. Gates

| Lote | Gate | Motivo |
|---|---|---|
| 1 | **R2** | mapa sensível; RBAC |
| 2 | CI verde | funções puras |
| 3 | **R2** | leitura de valores pessoais reais |
| 4 | **R2** | exposição de dado pessoal × contas |
| 5 | CI verde + Trinity | UI |
| 6 | **R2** | exportação |

## 7. Riscos

- Falso positivo em números de 11 dígitos sem DV (telefones). Mitigação: DV obrigatório para a confiança alta.
- JSONB e texto livre com dado pessoal ficam fora do escopo. A ressalva aparece no mapa e no PDF.
- O próprio mapa é alvo de ataque. Mitigação: download só para auditor, redação e nenhum valor.
- Carga de leitura em produção. Mitigação: amostragem por páginas, tetos e timeouts.
- Uma regressão na amostragem compartilhada pode afetar o diagnóstico de qualidade atual. Os testes existentes de `internal/quality` são gate.

## 8. Pontos para Q

- O RF-02 fala em "CNPJ com dígito verificador". O CNPJ **alfanumérico** da Receita Federal (em vigor a partir de julho de 2026) muda o cálculo do DV. Suportar os dois formatos no v1?
- A spec não diz quem edita o dicionário. O plano atribui ao operator. Confirmar.
- A habilitação por ambiente, além da flag de processo, é a leitura do plano para "flag mais restrita" (**decisão padrão** nº 4). Confirmar se basta a flag de processo.

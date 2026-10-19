# ADR-0001 — Decisões de arquitetura do v1 das features do roadmap

- **Status:** Aceita
- **Data:** 2026-10-18
- **Decisores:** Q (arquiteto), com M (PO) e Maycon Mendes
- **Escopo:** as 10 specs de [`docs/spec/`](../spec/README.md) e os planos de [`docs/plan/`](../plan/README.md)
- **Substitui:** as "decisões padrão" listadas no README dos planos no commit `a2ce03a`

## Contexto

As 10 specs do roadmap (`b1a93cf`) deixaram perguntas em aberto (P-xx). Os planos de desenvolvimento (`a2ce03a`) aplicaram 11 decisões padrão de Q e levantaram conflitos entre essas decisões, as specs e o código. Em 2026-10-18, Q resolveu os conflitos:
- 7 decisões explícitas;
- adoção das recomendações dos planos para os outros 9 pontos;
- adiamento da posição das telas na navegação.

Esta ADR é a fonte única dessas decisões. Specs e planos citam as decisões pelo número (**ADR-0001 Dn**). Quando uma spec e esta ADR divergirem, vale a ADR, e a spec deve ser corrigida.

Fatos do código que pesaram nas decisões (`main` em `b1a93cf`):
- `authMiddleware` aceita só `Bearer` ≤ 128 caracteres resolvido como sessão;
- `AUDITOR_REPORT_API_TOKEN` é um token global de relatório;
- `internal/notify` usa canais globais por variável de ambiente (`AUDITOR_ALERT_WEBHOOK_URL`, `AUDITOR_ALERT_EMAIL_TO`, SMTP);
- `quality.Request.Validate` limita a amostra a 1000 linhas;
- `regression_alert` só tem as categorias `findings`, `structure` e `workload`;
- a imagem é compilada com `CGO_ENABLED=0`;
- `fingerprint.NormalizeSQL` não remove literais; quem remove é `NormalizeQuery` em `collectors/postgres/stats_workload.go`.

## Decisão

As decisões D1–D11 são as 11 decisões padrão originais, com as emendas de 2026-10-18 marcadas **(emendada)**. As decisões D12–D16 são novas.

### D1 — Credencial da CLI: token por ambiente com escopo mínimo
- A CLI (specs 01, 03, 07 e 10, além da 08) usa um **token novo por ambiente**, criado por operator.
- O token tem **validade obrigatória** e escopos de uma **lista fechada**:
  - `snapshot-context:read`, `migration-analysis:write` e `deploy-annotation:write` (01);
  - `quality-contract:run` (03);
  - `restore-evidence:write` (08);
  - `last-login:write` (07/10, ver D10).
- O token de um ambiente nunca lê nem escreve em outro (403).
- `AUDITOR_REPORT_API_TOKEN` **não** é reutilizado.

### D2 — Spec 01: contexto no CI só por snapshot via API (confirmada)
- O CI usa só o último snapshot completo, lido pela API com o token da D1. **O runner de CI nunca recebe senha nem DSN do banco auditado.**
- O modo (b), conexão direta só leitura, existe **só para uso local do operador**:
  - a GitHub Action não o oferece;
  - a documentação proíbe DSN no runner de CI.
- A única exceção ao "nenhum DSN fora do auditor" é o opt-in da D8.

### D3 — Spec 02: moeda, preço e bloat
- **Uma moeda por ambiente** (`BRL` ou `USD`), com tabela de preços configurável e versionada.
- A moeda fica **travada depois da primeira versão** da tabela de preços. Trocar a moeda exige procedimento manual e corta a série. Não há câmbio.
- **Bloat só por estimativa** estatística. A medição física por extensão no alvo fica para depois (fora do v1).
- Os modelos iniciais são só RDS PostgreSQL, Cloud SQL e self-hosted (P-03).

### D4 — Spec 03: amostra dos contratos; spec 05: flag própria (emendada)
- **Spec 03:** amostra padrão de **10 000 linhas** por checagem. Por **opt-in**, até **100 000**:
  - o operator sobe o teto do processo com `AUDITOR_DATA_QUALITY_MAX_SAMPLE` (padrão 10 000, máximo 100 000);
  - cada contrato declara `sample_limit` dentro do teto.
- O diagnóstico sob demanda que já existe (`POST .../quality-scans`) mantém o teto atual de 1000.
- Esta decisão substitui o "1000 linhas" da decisão padrão original.
- **Spec 05:** a validação de conteúdo de PII tem **flag própria, mais restrita**. São exigidas as duas coisas:
  - `AUDITOR_PII_CONTENT_SCAN_ENABLED=1` no processo;
  - habilitação explícita por ambiente, feita pelo operator.

  `AUDITOR_DATA_QUALITY_ENABLED` não basta. A amostra da 05 continua em 1000 linhas por tabela, independente do teto da 03.

### D5 — Spec 04: evidência de pipeline
- Duas fontes de evidência, e **qualquer uma** correlaciona o evento:
  - a **anotação de implantação** (`audit_annotation` `kind='deployment'`), que vale sempre que existir;
  - o **registro de migrations** das ferramentas conhecidas (golang-migrate, goose, Flyway, Alembic, Rails, Prisma), opcional e configurado por ambiente.
- O registro **não** é obrigatório para marcar "fora do pipeline".
- Ferramentas sem carimbo de aplicação (golang-migrate, Alembic, Rails) correlacionam por "a versão mudou entre as duas coletas", com confiança rebaixada.
- Não há entidade nova de par de ambientes. O RF-06 usa:
  - o atributo `is_production`;
  - o par definido em `object_mapping`.
- Webhook: JSON genérico e formato nativo do Slack, com o formato escolhido **por webhook** (D15).

### D6 — Spec 06: sugestões de índice e destinatários (emendada)
- Só as sugestões de índice existentes (`index.candidate`, `performance.workload_scan`). **Sem HypoPG no v1.**
- **Destinatários por ambiente** (e-mail e webhook), configurados pelo operator.
- Quando um ambiente não tem destinatário configurado, o digest usa os **canais globais** atuais (fallback).
- Viewer recebe 403 ao mudar destinatários.
- Os alertas que já existem (regressão etc.) continuam só nos canais globais.

### D7 — Spec 07: responsável, exportação e último login (emendada)
- **O RNF-04 da spec prevalece.** O responsável é uma **conta do auditor** com papel auditor ou acima no ambiente.
- Pode ter um **contato livre opcional** (nome e/ou e-mail), usado só na planilha e no PDF.
- Mapear um responsável sem acesso ao ambiente gera 400.
- Quem decide uma linha:
  - o responsável dela;
  - nas linhas sem responsável, o operator.
- Outro auditor recebe 403.
- A UI tem a visão "minhas pendências".
- **Saída em CSV no v1. XLSX fica para depois** (fora do v1, RF-09 e RNF-03 parciais).
- O **último login** vem do agregado de logs da spec 10 (D10), ao lado da amostra de sessões atual, com a fonte explícita.

### D8 — Spec 08: restauração e hash de amostra (emendada)
- A restauração roda **só pela CLI**, num runner externo. A API nunca acessa o runtime de container.
- **Só `pg_dump`** (custom ou diretório) no v1. pgBackRest e WAL-G ficam fora.
- O **hash de amostra por tabela** é **opt-in** (`--sample-hash`). Ele só roda com um **DSN só leitura da origem fornecido explicitamente pelo operator no runner** (`AUDITOR_RESTORE_VERIFY_SOURCE_DSN` ou `_FILE`, nunca em argumento), validado pela conexão só leitura (conta sem escrita, allowlist, TLS). É a única exceção à D2.
- Sem opt-in, a verificação (c) é "desabilitada" e **não** conta como execução parcial: o código segue as outras verificações e pode ser 0.
- Com opt-in e origem inacessível, (c) fica "não executada" e o código é 3.
- O CA-01 padrão usa inventário, contagens e integridade física. O hash tem critério de aceite próprio (CA-09).
- A recusa de destino igual a um ambiente configurado usa **fingerprints** (sha256 de `host:porta/banco`) fornecidos pela API ao token do ambiente. A API nunca devolve DSNs.

### D9 — Spec 09: chunks sem consulta e achado de cenário
- Chunks possivelmente sem consulta entram no v1, usando **contadores de varredura por chunk como proxy**, sempre rotulados **aproximados**. Reset invalida a janela.
- O achado `timescale.retention_cagg_conflict` de cenário (CA-04) é criado **no salvamento do cenário**, vinculado à última coleta completa.

### D10 — Spec 10: fontes e o que sobe à API (emendada)
- O v1 lê **logs JSON do PostgreSQL** (`postgres-jsonlog`, PG 15+) e **JSON genérico** com mapeamento de campos.
- O processamento é local: linhas, templates, contagens e anomalias **não** sobem à API no v1 (o RF-07 fica fora do v1).
- **Única exceção:** o **agregado de último login por role**, com **só o nome da role e o horário mais recente**, nada mais. Ele é enviado com o token da D1, no escopo `last-login:write`, e só para o ambiente do token. O auditor o usa na spec 07 (D7).

### D11 — Tenant = ambiente
- A unidade de isolamento é o ambiente (`audit_environment`).
- Os papéis são `viewer` < `auditor` < `operator`, com concessões em `auditor_user_environment`.
- Todo dado novo pertence a um único ambiente. Toda leitura cruzada exige concessão aos dois lados.

### D12 — Spec 01: referência de versão e SARIF
- A operação de referência do CA-08 é tornar uma coluna obrigatória (MR-04):
  - no PG 18 existe `NOT NULL ... NOT VALID`;
  - no PG 14–17 a alternativa segura é `CHECK (... IS NOT NULL) NOT VALID` + `VALIDATE`.
- O golden roda nas cinco versões da matriz (14–18).
- SARIF fica fora do v1 (P-03).

### D13 — Spec 03: contrato canônico e regressão
- O **contrato canônico fica no auditor**.
- O YAML importado cria uma versão `proposed`, que um auditor aprova.
- O YAML passado à CLI (`--file`/`--dir`) é uma **execução ad hoc** registrada como `cli`. Ele não cria nem aprova versão.
- Violação persistente usa a **dedup e a notificação de achados** existentes (`NotifyFindings`/`ClaimAlert`, regra de lacuna AN-11). **Não** há categoria nova em `regression_alert`.

### D14 — Spec 05: detectores e dicionário
- O detector de CNPJ aceita o formato **numérico e o alfanumérico** (Receita Federal, em vigor desde julho de 2026), com o DV de cada formato.
- O dicionário de nomes é editado pelo **operator**.
- JSONB e texto livre ficam fora do v1 (P-02).

### D15 — Formato de webhook por destino
- O formato (`generic` ou `slack`) é configurado **por webhook**, no destino por ambiente (D6). Não há configuração global.
- O webhook global (`AUDITOR_ALERT_WEBHOOK_URL`) continua no JSON genérico atual. **Os alertas que já existem não mudam.**
- Usam destinos por ambiente: o digest (06) e os alertas de drift (04).

### D16 — Correções factuais e registro de decisões
- A remoção de literais sai de `collectors/postgres/stats_workload.go` (`NormalizeQuery`) para `internal/fingerprint/literals.go` (plano 01, lote 2), sem mudar a saída do coletor de workload. As specs que diziam que `internal/fingerprint` já remove literais foram corrigidas.
- O parser de DDL é em **Go puro**, porque a imagem é `CGO_ENABLED=0`.
- Decisões de arquitetura passam a ser registradas em `docs/adr/` (esta é a primeira).

### Pendente, sem decisão nesta ADR
- **Posição das telas na navegação.** As specs citam "Segurança", "Performance" e "Tabelas temporais", que não são seções de `Shell.tsx`. Fica adiada para os lotes de tela de cada plano, quando Q chamará a Trinity. Os planos não decidem o lugar.

## Consequências

- **Specs e planos alinhados.** As specs 01–10 foram atualizadas para não contradizer esta ADR: as perguntas em aberto foram marcadas resolvidas, com o número da decisão. Os planos citam "ADR-0001 Dn" no lugar de "decisão padrão".
- **Escopo do v1 maior:**
  - os lotes antes condicionais (07 L6 e 10 L5) entram no v1, porque o último login sobe como agregado (D10);
  - o plano 06 ganha destinos por ambiente (D6, D15);
  - a 07 ganha autorização por responsável (D7);
  - a 05 ganha o CNPJ alfanumérico (D14).
- **Totais dos planos:** 55 lotes, 223 dias-pessoa e 28 lotes com gate R2 (ver [README dos planos](../plan/README.md)).
- **Superfícies novas de segurança** (todas com gate R2 nos planos):
  - destinos de webhook por ambiente: URL de webhook (a do Slack contém segredo), e-mail de destinatário como dado pessoal, risco de SSRF;
  - autorização por responsável na 07;
  - escopo `last-login:write`;
  - DSN de origem no runner, só no opt-in da 08.
- **Comportamento existente preservado:**
  - alertas atuais e webhook global sem mudança (D6, D15);
  - diagnóstico `quality-scans` com teto de 1000 (D4);
  - `AUDITOR_REPORT_API_TOKEN` sem novas rotas (D1).
- **Custo da amostra maior na 03.** O padrão de 10 000 aumenta a leitura no alvo. Mitigação: páginas, timeouts de 30 s, 4 checagens simultâneas por ambiente, e o opt-in acima de 10 000.
- **Pendências:** a posição das telas (Trinity, nos lotes de UI) e, no lote 3 do plano 06, a forma de guardar a URL de webhook em repouso (decisão de R2 no gate do lote).

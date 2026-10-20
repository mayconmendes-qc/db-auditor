# Contratos de qualidade de dados

Contratos versionados por tabela (RF-01). O canônico fica no auditor (ADR-0001 D13). YAML importado cria versão `proposed`. YAML na CLI é execução *ad hoc* (`cli`), sem criar nem aprovar versão.

## Pré-requisitos

- `AUDITOR_DATA_QUALITY_ENABLED=1`
- Conta de leitura sem escrita no alvo
- `AUDITOR_DATA_QUALITY_MAX_SAMPLE` (padrão `10000`, máximo `100000`) — teto do processo; cada contrato declara `sample_limit` dentro desse teto (ADR-0001 D4)
- Diagnóstico sob demanda (`quality-scans`) permanece com teto 1000

## Esquema YAML

```yaml
database: app
schema: public
table: orders
sample_limit: 10000          # padrão 10000; até o teto do processo
schedule_profile: daily      # opcional: fast|daily|weekly|monthly
checks:                      # no máximo 50
  - type: nulls
    column: customer_id
    max_null_pct: 0
    critical: true           # severidade alta se true
  - type: uniqueness
    columns: [id]
  - type: orphan
    column: customer_id
    ref_schema: public
    ref_table: customers
    ref_column: id
  - type: domain
    column: status
    allowed: [pending, paid, cancelled]
    tolerance_pct: 0
  - type: freshness
    column: updated_at
    max_age_hours: 24
  - type: volume
    max_change_pct: 50
```

Tipos: `nulls`, `uniqueness`, `orphan`, `domain`, `freshness`, `volume`.

YAML inválido: API 400 com `line` e `field`.

## API (resumo)

| Método | Rota | Papel |
|---|---|---|
| GET | `/api/v1/environments/{id}/quality-contracts` | viewer |
| POST | `.../quality-contracts` | auditor |
| POST | `.../quality-contracts/proposals` | auditor |
| POST | `.../quality-contracts/import` | auditor |
| POST | `.../quality-contracts/{c}/versions` | auditor |
| POST | `.../quality-contracts/{c}/versions/{v}/approve` | auditor (viewer → 403) |
| POST | `.../quality-contracts/{c}/runs` | auditor ou token `quality-contract:run` |
| GET | `.../runs`, `.../runs/{run}`, `.../trend`, `.../markdown` | viewer |

Lista de domínio omitida para viewer (RNF-02).

## CLI

```bash
auditor dq-contracts run \
  --environment <uuid> \
  --contract <uuid> \
  --fail-on high \
  --format md
```

Ad hoc (não aprova):

```bash
auditor dq-contracts run --environment <uuid> --file contract.yaml --fail-on high
```

Autenticação: `AUDITOR_ENV_TOKEN` / `_FILE` com escopo `quality-contract:run`. A CLI **não** recebe DSN do alvo.

Códigos: 0 ok · 1 gate · 2 uso/YAML/recurso desabilitado/credencial · 3 checagem não executada.

## Persistência

Só contagens, proporções, tamanho da amostra e idade em horas do frescor. Nunca valores de linha nem carimbo de frescor.

Achado: `quality.contract_violation` com dedup por contrato + checagem + versão. Sem categoria nova em `regression_alert` (D13).

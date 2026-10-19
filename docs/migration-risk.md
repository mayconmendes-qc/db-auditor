# Risco de migrations (`migration-risk`)

Analisa arquivos `.sql` de migration PostgreSQL (majors 14–18), classifica locks e impacto
(MR-01…MR-11) e, opcionalmente, usa o último snapshot do ambiente via API — **sem DSN no CI**
(ADR-0001 D2).

## CLI

```bash
# Análise estática (sem contexto)
auditor migration-risk --path ./migrations --pg-version 16 --format md

# Com contexto do snapshot (CI / local com token)
export AUDITOR_API_URL=https://auditor.example
export AUDITOR_CLI_TOKEN=dba_…   # ou AUDITOR_CLI_TOKEN_FILE
auditor migration-risk --path ./migrations \
  --environment <uuid> --api-url "$AUDITOR_API_URL" --fail-on high

# Enviar histórico (escopo migration-analysis:write)
auditor migration-risk --path ./migrations --environment <uuid> \
  --submit --repository org/app --pr 42 --commit "$GITHUB_SHA"

# Modo direto só leitura (uso local do operador — NUNCA no runner de CI)
export AUDITOR_MIGRATION_RISK_DSN=postgres://readonly@…/db
auditor migration-risk --path ./migrations --direct
```

Segredos só por variável ou `*_FILE`. Flags `--token` / `--password` / `--dsn` são recusadas (código 2).

### Códigos de saída

| Código | Significado |
|---|---|
| 0 | nada acima do limiar (`--fail-on`, padrão `high`) |
| 1 | bloqueio pelo gate |
| 2 | uso, configuração ou credencial |
| 3 | contexto parcial/antigo/ausente ou erro de parse |
| 4 | recusa de segurança (conta com escrita no modo `--direct`) |

### Supressão

```sql
-- db-auditor:ignore MR-01 motivo: janela de manutenção aprovada no ticket T-123
CREATE INDEX idx_orders_email ON public.orders (email);
```

## GitHub Action

```yaml
permissions:
  contents: read
  pull-requests: write

jobs:
  migration-risk:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: ./.github/actions/migration-risk
        with:
          environment_id: ${{ vars.AUDITOR_ENVIRONMENT_ID }}
          api_url: ${{ vars.AUDITOR_API_URL }}
          path: migrations
          fail_on: high
        env:
          AUDITOR_CLI_TOKEN: ${{ secrets.AUDITOR_CLI_TOKEN }}
```

A Action usa só o modo snapshot (a). Não aceita DSN. Publica **um** comentário no PR com o
marcador `<!-- db-auditor:migration-risk -->` e o atualiza a cada push.

## Tokens

Criados pelo operator na tela Ambientes. Escopos iniciais: `snapshot-context:read`,
`migration-analysis:write`, `deploy-annotation:write`. Ver [ops.md](ops.md) e [hardening.md](hardening.md).

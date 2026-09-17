# Timescale Auditor

Serviço **read-only** de inventário, comparação e diagnóstico dos ambientes TimescaleDB (Tiger Cloud e Datacenter Unifique).

## Estrutura canônica do repositório

```text
.
├── backend/
│   ├── cmd/auditor/              # entrypoint HTTP
│   ├── internal/
│   │   ├── api/                  # handlers HTTP (sem regra de domínio pesada)
│   │   ├── config/               # env, timeouts, allow/deny lists
│   │   ├── database/             # pool pgx + sqlc gerado
│   │   └── repository/           # transações e repositórios mínimos
│   ├── migrations/               # schema do Snapshot Store
│   ├── sql/queries/              # SQL fonte do sqlc
│   ├── Containerfile
│   ├── go.mod / go.sum
│   ├── Makefile
│   ├── sqlc.yaml
│   └── .golangci.yml
├── frontend/
│   ├── src/
│   │   ├── components/           # layout + primitives UI
│   │   ├── hooks/
│   │   ├── pages/
│   │   ├── services/             # cliente HTTP tipado
│   │   └── types/
│   ├── package.json              # Bun
│   └── …
├── .github/workflows/ci.yml
├── compose.yaml
├── Makefile
├── .env.example
└── README.md
```

### Responsabilidades dos packages (backend)

| Package | Responsabilidade |
|---------|------------------|
| `cmd/auditor` | Bootstrap do processo: config, pool, HTTP server, shutdown |
| `internal/api` | Fronteira HTTP; serialização JSON; sem SQL direto |
| `internal/config` | Validação de env; sanitização de secrets; allow/deny de escopo |
| `internal/database` | `pgxpool` e runtime params (`statement_timeout`, …) |
| `internal/database/sqlc` | Código **gerado** por sqlc — não editar à mão |
| `internal/repository` | Orquestra queries tipadas e `pgx.Tx` |

Regra: preferir Go idiomático e packages pequenos; evitar camadas sem necessidade.

## Stack

| Camada | Tecnologia |
|--------|------------|
| Backend | Go 1.27.x, `net/http`, `pgx`/`pgxpool`, `sqlc`, PostgreSQL |
| Frontend | React, TypeScript strict, Tailwind CSS, Vitest, Biome, **Bun** |
| Runtime local | Podman Compose |

## Ambiente local

1. `cp .env.example .env` e defina `POSTGRES_PASSWORD`.
2. Suba a API e o Snapshot Store:

```bash
make up
# ou: podman compose up -d --build
```

3. Smoke check:

```bash
make smoke
# equivale a: curl /health e /ready
```

Logs e parada:

```bash
make logs
make down
```

Reset **destrutivo** do volume local do Postgres:

```bash
make reset-volume   # podman compose down -v
```

### Frontend (Bun)

```bash
cd frontend && bun install && bun run dev
```

O frontend consome **apenas** a API HTTP.

## Qualidade

```bash
make backend-fmt backend-check backend-staticcheck backend-lint backend-test
make backend-test-cover
make frontend-install frontend-check frontend-typecheck frontend-test
```

Regenerar sqlc (requer `sqlc` instalado):

```bash
cd backend && make sqlc
```

## CI

- Backend: gofmt, vet, **staticcheck**, golangci-lint, `go test -cover`, Trivy
- Frontend: Biome, typecheck, Vitest, Trivy

## Segurança

- Ambientes auditados: somente leitura
- Secrets fora do repositório
- Connection strings sanitizadas em logs
- Allowlist/denylist de databases e schemas via env
- `statement_timeout`, `lock_timeout`, `application_name`

## Documentação de produto

Notion: *Anotações / Timescale Auditor* (EF, Backlog, Fluxo de Desenvolvimento).
